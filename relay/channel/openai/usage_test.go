package openai

import (
	"encoding/json"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/setting/ratio_setting"

	"github.com/stretchr/testify/require"
)

// TestAmplifyCachedTokensForResponse locks the package-level helper used by
// every relay adaptor path that mutates CachedTokens before the response body
// reaches the wire. Phase 2 added this helper; Phase 3 changed the source of
// truth from a global common.CacheReadAmplificationRatio to a per-group
// resolver (with global fallback). The legacy "info=nil" entry point is still
// supported for test ergonomics.
func TestAmplifyCachedTokensForResponse(t *testing.T) {
	// Snapshot and restore global + per-group state so this test doesn't leak.
	prevGlobal := common.CacheReadAmplificationRatio
	prevPerGroup := ratio_setting.GetGroupCacheReadAmplificationRatioCopy()
	defer func() {
		common.CacheReadAmplificationRatio = prevGlobal
		_ = ratio_setting.UpdateGroupCacheReadAmplificationRatioByJSONString(stringifyPerGroup(t, prevPerGroup))
	}()
	common.CacheReadAmplificationRatio = 1.0 // baseline so per-group wins when configured

	cases := []struct {
		name     string
		ratio    float64
		input    int
		want     int
		skipMutate bool // when input is 0 we should remain 0 regardless of ratio
	}{
		{name: "ratio 1.0 is identity", ratio: 1.0, input: 100, want: 100},
		{name: "ratio 2.0 doubles", ratio: 2.0, input: 100, want: 200},
		{name: "ratio 0.5 halves", ratio: 0.5, input: 100, want: 50},
		{name: "ratio 1.5 with 80", ratio: 1.5, input: 80, want: 120},
		{name: "zero input stays zero", ratio: 2.0, input: 0, want: 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			common.CacheReadAmplificationRatio = tc.ratio
			// Per-group empty so resolver falls back to global.
			require.NoError(t, ratio_setting.UpdateGroupCacheReadAmplificationRatioByJSONString(`{}`))

			u := &dto.Usage{
				PromptTokensDetails: dto.InputTokenDetails{CachedTokens: tc.input},
			}
			// info=nil path: helper uses global common.CacheReadAmplificationRatio.
			amplifyCachedTokensForResponse(nil, u)
			require.Equal(t, tc.want, u.PromptTokensDetails.CachedTokens)
		})
	}

	// nil usage guard
	require.NotPanics(t, func() { amplifyCachedTokensForResponse(nil, nil) })
}

// TestAmplifyCachedTokensForResponsePerGroup locks Phase 3's per-group
// behaviour: when info.UsingGroup has a configured entry in
// ratio_setting.GroupCacheReadAmplificationRatio, that value wins regardless
// of the global ratio. When the group is not configured, the global is used.
func TestAmplifyCachedTokensForResponsePerGroup(t *testing.T) {
	prevGlobal := common.CacheReadAmplificationRatio
	prevPerGroup := ratio_setting.GetGroupCacheReadAmplificationRatioCopy()
	defer func() {
		common.CacheReadAmplificationRatio = prevGlobal
		_ = ratio_setting.UpdateGroupCacheReadAmplificationRatioByJSONString(stringifyPerGroup(t, prevPerGroup))
	}()

	require.NoError(t, ratio_setting.UpdateGroupCacheReadAmplificationRatioByJSONString(
		`{"default":1,"vip":1.5,"svip":2}`))
	common.CacheReadAmplificationRatio = 1.0

	cases := []struct {
		name    string
		group   string
		global  float64
		want    int
	}{
		{name: "vip per-group 1.5 wins over global 1.0", group: "vip", global: 1.0, want: 150},
		{name: "svip per-group 2.0 wins over global 1.5", group: "svip", global: 1.5, want: 200},
		{name: "default per-group 1.0 wins over global 3.0", group: "default", global: 3.0, want: 100},
		{name: "unknown group falls back to global 2.5", group: "auto", global: 2.5, want: 250},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			common.CacheReadAmplificationRatio = tc.global
			u := &dto.Usage{
				PromptTokensDetails: dto.InputTokenDetails{CachedTokens: 100},
			}
			info := &relaycommon.RelayInfo{UsingGroup: tc.group}
			amplifyCachedTokensForResponse(info, u)
			require.Equal(t, tc.want, u.PromptTokensDetails.CachedTokens)
		})
	}

	// Per-group ratio of 0 falls back to global.
	t.Run("zero per-group falls back to global", func(t *testing.T) {
		require.NoError(t, ratio_setting.UpdateGroupCacheReadAmplificationRatioByJSONString(`{"zeros":0}`))
		common.CacheReadAmplificationRatio = 2.5
		u := &dto.Usage{PromptTokensDetails: dto.InputTokenDetails{CachedTokens: 100}}
		info := &relaycommon.RelayInfo{UsingGroup: "zeros"}
		amplifyCachedTokensForResponse(info, u)
		require.Equal(t, 250, u.PromptTokensDetails.CachedTokens)
	})
}

// TestApplyUsagePostProcessingSkipsOpenRouter locks the OpenRouter exclusion
// branch in applyUsagePostProcessing. The OpenRouter Claude billing path
// reverse-derives cache creation tokens from the upstream cost using the
// ORIGINAL cache read count; feeding it the amplified value would skew the
// math, so the helper must skip amplification for that channel.
func TestApplyUsagePostProcessingSkipsOpenRouter(t *testing.T) {
	prevGlobal := common.CacheReadAmplificationRatio
	prevPerGroup := ratio_setting.GetGroupCacheReadAmplificationRatioCopy()
	defer func() {
		common.CacheReadAmplificationRatio = prevGlobal
		_ = ratio_setting.UpdateGroupCacheReadAmplificationRatioByJSONString(stringifyPerGroup(t, prevPerGroup))
	}()
	// Set both global and per-group aggressively so any non-skip would
	// amplify; the test only passes if OpenRouter stays at 100.
	common.CacheReadAmplificationRatio = 2.0
	require.NoError(t, ratio_setting.UpdateGroupCacheReadAmplificationRatioByJSONString(`{"vip":3}`))

	t.Run("OpenRouter channel does not amplify", func(t *testing.T) {
		u := &dto.Usage{
			PromptTokensDetails: dto.InputTokenDetails{CachedTokens: 100},
		}
		info := &relaycommon.RelayInfo{
			UsingGroup: "vip",
			ChannelMeta: &relaycommon.ChannelMeta{ChannelType: constant.ChannelTypeOpenRouter},
		}
		applyUsagePostProcessing(info, u, nil)
		require.Equal(t, 100, u.PromptTokensDetails.CachedTokens,
			"OpenRouter must NOT amplify CachedTokens")
	})

	t.Run("non-OpenRouter channel does amplify", func(t *testing.T) {
		u := &dto.Usage{
			PromptTokensDetails: dto.InputTokenDetails{CachedTokens: 100},
		}
		info := &relaycommon.RelayInfo{
			UsingGroup: "vip",
			ChannelMeta: &relaycommon.ChannelMeta{ChannelType: constant.ChannelTypeOpenAI},
		}
		applyUsagePostProcessing(info, u, nil)
		require.Equal(t, 300, u.PromptTokensDetails.CachedTokens,
			"non-OpenRouter channel must amplify CachedTokens via per-group lookup")
	})

	t.Run("nil info is a safe no-op", func(t *testing.T) {
		u := &dto.Usage{
			PromptTokensDetails: dto.InputTokenDetails{CachedTokens: 100},
		}
		require.NotPanics(t, func() { applyUsagePostProcessing(nil, u, nil) })
		require.Equal(t, 100, u.PromptTokensDetails.CachedTokens)
	})

	t.Run("nil usage is a safe no-op", func(t *testing.T) {
		info := &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{ChannelType: constant.ChannelTypeOpenAI}}
		require.NotPanics(t, func() { applyUsagePostProcessing(info, nil, nil) })
	})
}

func stringifyPerGroup(t *testing.T, m map[string]float64) string {
	t.Helper()
	b, err := json.Marshal(m)
	require.NoError(t, err)
	return string(b)
}