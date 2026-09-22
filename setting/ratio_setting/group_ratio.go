package ratio_setting

import (
	"encoding/json"
	"errors"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/config"
	"github.com/QuantumNous/new-api/types"
)

var defaultGroupRatio = map[string]float64{
	"default": 1,
	"vip":     1,
	"svip":    1,
}

var groupRatioMap = types.NewRWMap[string, float64]()

var defaultGroupGroupRatio = map[string]map[string]float64{
	"vip": {
		"edit_this": 0.9,
	},
}

var groupGroupRatioMap = types.NewRWMap[string, map[string]float64]()

var defaultGroupSpecialUsableGroup = map[string]map[string]string{}

var defaultGroupCacheReadAmplificationRatio = map[string]float64{
	"default": 1,
	"vip":     1,
	"svip":    1,
}

var groupCacheReadAmplificationRatioMap = types.NewRWMap[string, float64]()

type GroupRatioSetting struct {
	GroupRatio                       *types.RWMap[string, float64]            `json:"group_ratio"`
	GroupGroupRatio                  *types.RWMap[string, map[string]float64] `json:"group_group_ratio"`
	GroupSpecialUsableGroup          *types.RWMap[string, map[string]string]  `json:"group_special_usable_group"`
	GroupCacheReadAmplificationRatio *types.RWMap[string, float64]            `json:"group_cache_read_amplification_ratio"`
}

var groupRatioSetting GroupRatioSetting

func init() {
	groupSpecialUsableGroup := types.NewRWMap[string, map[string]string]()
	groupSpecialUsableGroup.AddAll(defaultGroupSpecialUsableGroup)

	groupRatioMap.AddAll(defaultGroupRatio)
	groupGroupRatioMap.AddAll(defaultGroupGroupRatio)
	groupCacheReadAmplificationRatioMap.AddAll(defaultGroupCacheReadAmplificationRatio)

	groupRatioSetting = GroupRatioSetting{
		GroupSpecialUsableGroup:          groupSpecialUsableGroup,
		GroupRatio:                       groupRatioMap,
		GroupGroupRatio:                  groupGroupRatioMap,
		GroupCacheReadAmplificationRatio: groupCacheReadAmplificationRatioMap,
	}

	config.GlobalConfig.Register("group_ratio_setting", &groupRatioSetting)
}

func GetGroupRatioSetting() *GroupRatioSetting {
	if groupRatioSetting.GroupSpecialUsableGroup == nil {
		groupRatioSetting.GroupSpecialUsableGroup = types.NewRWMap[string, map[string]string]()
		groupRatioSetting.GroupSpecialUsableGroup.AddAll(defaultGroupSpecialUsableGroup)
	}
	if groupRatioSetting.GroupCacheReadAmplificationRatio == nil {
		groupRatioSetting.GroupCacheReadAmplificationRatio = types.NewRWMap[string, float64]()
		groupRatioSetting.GroupCacheReadAmplificationRatio.AddAll(defaultGroupCacheReadAmplificationRatio)
	}
	return &groupRatioSetting
}

func GetGroupRatioCopy() map[string]float64 {
	return groupRatioMap.ReadAll()
}

func ContainsGroupRatio(name string) bool {
	_, ok := groupRatioMap.Get(name)
	return ok
}

func GroupRatio2JSONString() string {
	return groupRatioMap.MarshalJSONString()
}

func UpdateGroupRatioByJSONString(jsonStr string) error {
	return types.LoadFromJsonString(groupRatioMap, jsonStr)
}

func GetGroupRatio(name string) float64 {
	ratio, ok := groupRatioMap.Get(name)
	if !ok {
		common.SysLog("group ratio not found: " + name)
		return 1
	}
	return ratio
}

func GetGroupGroupRatio(userGroup, usingGroup string) (float64, bool) {
	gp, ok := groupGroupRatioMap.Get(userGroup)
	if !ok {
		return -1, false
	}
	ratio, ok := gp[usingGroup]
	if !ok {
		return -1, false
	}
	return ratio, true
}

func GroupGroupRatio2JSONString() string {
	return groupGroupRatioMap.MarshalJSONString()
}

func UpdateGroupGroupRatioByJSONString(jsonStr string) error {
	return types.LoadFromJsonString(groupGroupRatioMap, jsonStr)
}

func CheckGroupRatio(jsonStr string) error {
	checkGroupRatio := make(map[string]float64)
	err := json.Unmarshal([]byte(jsonStr), &checkGroupRatio)
	if err != nil {
		return err
	}
	for name, ratio := range checkGroupRatio {
		if ratio < 0 {
			return errors.New("group ratio must be not less than 0: " + name)
		}
	}
	return nil
}

// GetGroupCacheReadAmplificationRatioCopy returns a plain map snapshot of the
// current GroupCacheReadAmplificationRatio values.
func GetGroupCacheReadAmplificationRatioCopy() map[string]float64 {
	return groupCacheReadAmplificationRatioMap.ReadAll()
}

// ContainsGroupCacheReadAmplificationRatio checks whether a group name has an
// entry in the per-group cache read amplification map.
func ContainsGroupCacheReadAmplificationRatio(name string) bool {
	_, ok := groupCacheReadAmplificationRatioMap.Get(name)
	return ok
}

// GroupCacheReadAmplificationRatio2JSONString serialises the map to JSON.
func GroupCacheReadAmplificationRatio2JSONString() string {
	return groupCacheReadAmplificationRatioMap.MarshalJSONString()
}

// UpdateGroupCacheReadAmplificationRatioByJSONString loads the map from JSON.
func UpdateGroupCacheReadAmplificationRatioByJSONString(jsonStr string) error {
	return types.LoadFromJsonString(groupCacheReadAmplificationRatioMap, jsonStr)
}

// GetGroupCacheReadAmplificationRatio returns the ratio for a named group,
// defaulting to 1.0 if not found.
func GetGroupCacheReadAmplificationRatio(name string) float64 {
	ratio, ok := groupCacheReadAmplificationRatioMap.Get(name)
	if !ok {
		common.SysLog("group cache read amplification ratio not found: " + name)
		return 1
	}
	return ratio
}

// CheckGroupCacheReadAmplificationRatio validates that every ratio is > 0.
// Zero is treated as "unset" so the resolver falls back to the global value;
// negative values are rejected outright.
func CheckGroupCacheReadAmplificationRatio(jsonStr string) error {
	checkMap := make(map[string]float64)
	err := json.Unmarshal([]byte(jsonStr), &checkMap)
	if err != nil {
		return err
	}
	for name, ratio := range checkMap {
		if ratio < 0 {
			return errors.New("group cache read amplification ratio must be not less than 0: " + name)
		}
	}
	return nil
}

// ResolveCacheReadAmplificationRatio returns the effective cache read
// amplification ratio for a given using-group. Per-group override wins;
// otherwise fall back to the global common.CacheReadAmplificationRatio.
// Callers on the OpenRouter path must skip this entirely (the per-group map
// is not consulted there) to preserve the reverse-calc contract of
// CalcOpenRouterCacheCreateTokens, which must see the ORIGINAL upstream count.
func ResolveCacheReadAmplificationRatio(groupName string) float64 {
	if ratio, ok := groupCacheReadAmplificationRatioMap.Get(groupName); ok && ratio > 0 {
		return ratio
	}
	return common.CacheReadAmplificationRatio
}
