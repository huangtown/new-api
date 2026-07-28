package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestGetRandomSatisfiedChannelExcludingWithMemoryCache(t *testing.T) {
	oldMemoryCacheEnabled := common.MemoryCacheEnabled
	channelSyncLock.Lock()
	oldGroup2Model2Channels := group2model2channels
	oldChannelsIDM := channelsIDM
	oldAdvancedConfigs := channel2advancedCustomConfig
	channelSyncLock.Unlock()
	t.Cleanup(func() {
		common.MemoryCacheEnabled = oldMemoryCacheEnabled
		channelSyncLock.Lock()
		group2model2channels = oldGroup2Model2Channels
		channelsIDM = oldChannelsIDM
		channel2advancedCustomConfig = oldAdvancedConfigs
		channelSyncLock.Unlock()
	})

	highPriority := int64(100)
	lowPriority := int64(10)
	weight := uint(100)
	common.MemoryCacheEnabled = true
	channelSyncLock.Lock()
	group2model2channels = map[string]map[string][]int{
		"default": map[string][]int{"test-model": []int{1, 2, 3}},
	}
	channelsIDM = map[int]*Channel{
		1: &Channel{Id: 1, Priority: &highPriority, Weight: &weight},
		2: &Channel{Id: 2, Priority: &lowPriority, Weight: &weight},
		3: &Channel{Id: 3, Priority: &lowPriority, Weight: &weight},
	}
	channel2advancedCustomConfig = nil
	channelSyncLock.Unlock()

	channel, err := GetRandomSatisfiedChannelExcluding("default", "test-model", 2, "", []int{1, 2})
	require.NoError(t, err)
	require.NotNil(t, channel)
	assert.Equal(t, 3, channel.Id)

	channel, err = GetRandomSatisfiedChannelExcluding("default", "test-model", 2, "", []int{1, 2, 3})
	require.NoError(t, err)
	assert.Nil(t, channel)
}

func TestGetChannelExcludingWithoutMemoryCache(t *testing.T) {
	oldDB := DB
	oldMemoryCacheEnabled := common.MemoryCacheEnabled
	oldMainDatabaseType := common.MainDatabaseType()
	oldLogDatabaseType := common.LogDatabaseType()
	t.Cleanup(func() {
		DB = oldDB
		common.MemoryCacheEnabled = oldMemoryCacheEnabled
		common.SetDatabaseTypes(oldMainDatabaseType, oldLogDatabaseType)
	})

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&Channel{}, &Ability{}))
	DB = db
	common.MemoryCacheEnabled = false
	common.SetMainDatabaseType(common.DatabaseTypeSQLite)

	highPriority := int64(100)
	lowPriority := int64(10)
	weight := uint(100)
	channels := []Channel{
		{Id: 1, Status: common.ChannelStatusEnabled, Name: "first"},
		{Id: 2, Status: common.ChannelStatusEnabled, Name: "second"},
		{Id: 3, Status: common.ChannelStatusEnabled, Name: "third"},
	}
	require.NoError(t, DB.Create(&channels).Error)
	abilities := []Ability{
		{Group: "default", Model: "test-model", ChannelId: 1, Enabled: true, Priority: &highPriority, Weight: weight},
		{Group: "default", Model: "test-model", ChannelId: 2, Enabled: true, Priority: &lowPriority, Weight: weight},
		{Group: "default", Model: "test-model", ChannelId: 3, Enabled: true, Priority: &lowPriority, Weight: weight},
	}
	require.NoError(t, DB.Create(&abilities).Error)

	channel, err := GetChannelExcluding("default", "test-model", 2, "", []int{1, 2})
	require.NoError(t, err)
	require.NotNil(t, channel)
	assert.Equal(t, 3, channel.Id)

	channel, err = GetChannelExcluding("default", "test-model", 2, "", []int{1, 2, 3})
	require.NoError(t, err)
	assert.Nil(t, channel)
}
