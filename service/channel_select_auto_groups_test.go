package service

import (
	"fmt"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupChannelSelectAutoGroupsTest(t *testing.T) *gorm.DB {
	t.Helper()

	originalDB := model.DB
	originalMemoryCacheEnabled := common.MemoryCacheEnabled
	originalRetryTimes := common.RetryTimes
	originalAutoGroups := setting.AutoGroups2JsonString()
	originalUsableGroups := setting.UserUsableGroups2JSONString()
	originalGroupRatios := ratio_setting.GroupRatio2JSONString()
	originalMaxTokenAutoGroups := setting.GetMaxTokenAutoGroups()

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.Channel{}, &model.Ability{}))
	model.DB = db
	common.MemoryCacheEnabled = true
	common.RetryTimes = 0

	require.NoError(t, setting.UpdateAutoGroupsByJsonString(`[]`))
	require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(`{"default":"Default","vip":"VIP"}`))
	require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(`{"default":1,"vip":2}`))
	require.NoError(t, setting.UpdateMaxTokenAutoGroups("2"))

	t.Cleanup(func() {
		model.DB = originalDB
		common.MemoryCacheEnabled = originalMemoryCacheEnabled
		common.RetryTimes = originalRetryTimes
		require.NoError(t, setting.UpdateAutoGroupsByJsonString(originalAutoGroups))
		require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(originalUsableGroups))
		require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(originalGroupRatios))
		require.NoError(t, setting.UpdateMaxTokenAutoGroups(fmt.Sprintf("%d", originalMaxTokenAutoGroups)))

		if originalMemoryCacheEnabled && originalDB != nil &&
			originalDB.Migrator().HasTable(&model.Channel{}) && originalDB.Migrator().HasTable(&model.Ability{}) {
			model.InitChannelCache()
		}
		sqlDB, err := db.DB()
		if err == nil {
			require.NoError(t, sqlDB.Close())
		}
	})

	return db
}

func createChannelSelectAutoGroupsChannel(t *testing.T, db *gorm.DB, id int, group, modelName string) {
	t.Helper()
	priority := int64(0)
	weight := uint(100)
	require.NoError(t, db.Create(&model.Channel{
		Id:       id,
		Type:     constant.ChannelTypeOpenAI,
		Key:      fmt.Sprintf("key-%d", id),
		Status:   common.ChannelStatusEnabled,
		Name:     fmt.Sprintf("channel-%d", id),
		Weight:   &weight,
		Models:   modelName,
		Group:    group,
		Priority: &priority,
	}).Error)
	require.NoError(t, db.Create(&model.Ability{
		Group:     group,
		Model:     modelName,
		ChannelId: id,
		Enabled:   true,
		Priority:  &priority,
		Weight:    weight,
	}).Error)
}

func TestCacheGetRandomSatisfiedChannelUsesTokenAutoGroupsWhenGlobalAutoIsEmpty(t *testing.T) {
	db := setupChannelSelectAutoGroupsTest(t)
	const modelName = "auto-groups-runtime-model"
	createChannelSelectAutoGroupsChannel(t, db, 2101, "vip", modelName)
	createChannelSelectAutoGroupsChannel(t, db, 2102, "default", modelName)
	model.InitChannelCache()

	gin.SetMode(gin.TestMode)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	common.SetContextKey(ctx, constant.ContextKeyUserGroup, "default")
	common.SetContextKey(ctx, constant.ContextKeyTokenAutoGroups, []string{"vip", "default"})
	common.SetContextKey(ctx, constant.ContextKeyTokenCrossGroupRetry, true)

	retry := 0
	param := &RetryParam{
		Ctx:         ctx,
		TokenGroup:  "auto",
		ModelName:   modelName,
		RequestPath: "/v1/chat/completions",
		Retry:       &retry,
	}

	first, selectedGroup, err := CacheGetRandomSatisfiedChannel(param)
	require.NoError(t, err)
	require.NotNil(t, first)
	assert.Equal(t, 2101, first.Id)
	assert.Equal(t, "vip", selectedGroup)
	assert.Equal(t, "vip", common.GetContextKeyString(ctx, constant.ContextKeyAutoGroup))
	assert.Empty(t, setting.GetAutoGroups(), "the selection must not depend on the global Auto list")

	param.IncreaseRetry()
	second, selectedGroup, err := CacheGetRandomSatisfiedChannel(param)
	require.NoError(t, err)
	require.NotNil(t, second)
	assert.Equal(t, 2102, second.Id)
	assert.Equal(t, "default", selectedGroup)
	assert.Equal(t, "default", common.GetContextKeyString(ctx, constant.ContextKeyAutoGroup))
}

// Unique retries prefer a channel the request has not tried yet, but a group
// with a single channel must still be retryable: excluding the only candidate
// would turn a transient upstream 5xx into an immediate hard failure.
func TestCacheGetRandomSatisfiedChannelRetriesTheOnlyChannelWhenAllExcluded(t *testing.T) {
	db := setupChannelSelectAutoGroupsTest(t)
	const modelName = "single-channel-retry-model"
	createChannelSelectAutoGroupsChannel(t, db, 2201, "default", modelName)
	model.InitChannelCache()

	gin.SetMode(gin.TestMode)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	common.SetContextKey(ctx, constant.ContextKeyUserGroup, "default")

	retry := 0
	param := &RetryParam{
		Ctx:                ctx,
		TokenGroup:         "default",
		ModelName:          modelName,
		RequestPath:        "/v1/chat/completions",
		Retry:              &retry,
		ExcludedChannelIDs: []int{2201},
	}

	channel, selectedGroup, err := CacheGetRandomSatisfiedChannel(param)
	require.NoError(t, err)
	require.NotNil(t, channel, "the only channel must remain selectable on retry")
	assert.Equal(t, 2201, channel.Id)
	assert.Equal(t, "default", selectedGroup)
}

// With an untried channel available the exclusion must still bite, otherwise
// unique retries would be a no-op and a failing channel could be picked twice.
func TestCacheGetRandomSatisfiedChannelStillSkipsTriedChannelWhenAlternativeExists(t *testing.T) {
	db := setupChannelSelectAutoGroupsTest(t)
	const modelName = "two-channel-retry-model"
	createChannelSelectAutoGroupsChannel(t, db, 2301, "default", modelName)
	createChannelSelectAutoGroupsChannel(t, db, 2302, "default", modelName)
	model.InitChannelCache()

	gin.SetMode(gin.TestMode)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	common.SetContextKey(ctx, constant.ContextKeyUserGroup, "default")

	retry := 0
	param := &RetryParam{
		Ctx:                ctx,
		TokenGroup:         "default",
		ModelName:          modelName,
		RequestPath:        "/v1/chat/completions",
		Retry:              &retry,
		ExcludedChannelIDs: []int{2301},
	}

	channel, _, err := CacheGetRandomSatisfiedChannel(param)
	require.NoError(t, err)
	require.NotNil(t, channel)
	assert.Equal(t, 2302, channel.Id, "must prefer the channel that has not been tried")
}

// The fallback must not re-run the auto-group scan: that scan advances
// ContextKeyAutoGroupIndex and resets the retry counter as it walks groups, so
// consuming it twice would skip a group on the next retry.
func TestCacheGetRandomSatisfiedChannelFallbackKeepsAutoGroupProgression(t *testing.T) {
	db := setupChannelSelectAutoGroupsTest(t)
	const modelName = "auto-fallback-model"
	createChannelSelectAutoGroupsChannel(t, db, 2401, "vip", modelName)
	createChannelSelectAutoGroupsChannel(t, db, 2402, "default", modelName)
	model.InitChannelCache()

	gin.SetMode(gin.TestMode)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	common.SetContextKey(ctx, constant.ContextKeyUserGroup, "default")
	common.SetContextKey(ctx, constant.ContextKeyTokenAutoGroups, []string{"vip", "default"})
	common.SetContextKey(ctx, constant.ContextKeyTokenCrossGroupRetry, true)

	retry := 0
	param := &RetryParam{
		Ctx:                ctx,
		TokenGroup:         "auto",
		ModelName:          modelName,
		RequestPath:        "/v1/chat/completions",
		Retry:              &retry,
		ExcludedChannelIDs: []int{2401},
	}

	// vip only holds the excluded channel, so the fallback inside the vip
	// lookup returns it rather than letting the scan fall through to default.
	channel, selectedGroup, err := CacheGetRandomSatisfiedChannel(param)
	require.NoError(t, err)
	require.NotNil(t, channel)
	assert.Equal(t, 2401, channel.Id)
	assert.Equal(t, "vip", selectedGroup)
	assert.Equal(t, "vip", common.GetContextKeyString(ctx, constant.ContextKeyAutoGroup))

	// The group index must have advanced exactly one step, so the next retry
	// lands on default instead of running off the end of the group list.
	param.IncreaseRetry()
	param.ExcludedChannelIDs = []int{2401}
	next, nextGroup, err := CacheGetRandomSatisfiedChannel(param)
	require.NoError(t, err)
	require.NotNil(t, next, "the next retry must still find the default group")
	assert.Equal(t, 2402, next.Id)
	assert.Equal(t, "default", nextGroup)
}
