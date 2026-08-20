package openai

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"

	"github.com/stretchr/testify/require"
)

// TestAmplifyCachedTokensForResponse locks the package-level helper used by
// every relay adaptor path that mutates CachedTokens before the response body
// reaches the wire. Phase 2 added this helper so that all paths share one
// implementation; regressions here would silently disable amplification for
// every chat / image / Responses path.
func TestAmplifyCachedTokensForResponse(t *testing.T) {
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
			prev := common.CacheReadAmplificationRatio
			common.CacheReadAmplificationRatio = tc.ratio
			defer func() { common.CacheReadAmplificationRatio = prev }()

			u := &dto.Usage{
				PromptTokensDetails: dto.InputTokenDetails{CachedTokens: tc.input},
			}
			amplifyCachedTokensForResponse(u)
			require.Equal(t, tc.want, u.PromptTokensDetails.CachedTokens)
		})
	}

	// nil guard
	require.NotPanics(t, func() { amplifyCachedTokensForResponse(nil) })
}

// TestApplyUsagePostProcessingSkipsOpenRouter locks the OpenRouter exclusion
// branch in applyUsagePostProcessing. The OpenRouter Claude billing path
// reverse-derives cache creation tokens from the upstream cost using the
// ORIGINAL cache read count; feeding it the amplified value would skew the
// math, so the helper must skip amplification for that channel.
func TestApplyUsagePostProcessingSkipsOpenRouter(t *testing.T) {
	prev := common.CacheReadAmplificationRatio
	common.CacheReadAmplificationRatio = 2.0
	defer func() { common.CacheReadAmplificationRatio = prev }()

	t.Run("OpenRouter channel does not amplify", func(t *testing.T) {
		u := &dto.Usage{
			PromptTokensDetails: dto.InputTokenDetails{CachedTokens: 100},
		}
		info := &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{ChannelType: constant.ChannelTypeOpenRouter}}
		applyUsagePostProcessing(info, u, nil)
		require.Equal(t, 100, u.PromptTokensDetails.CachedTokens,
			"OpenRouter must NOT amplify CachedTokens")
	})

	t.Run("non-OpenRouter channel does amplify", func(t *testing.T) {
		u := &dto.Usage{
			PromptTokensDetails: dto.InputTokenDetails{CachedTokens: 100},
		}
		info := &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{ChannelType: constant.ChannelTypeOpenAI}}
		applyUsagePostProcessing(info, u, nil)
		require.Equal(t, 200, u.PromptTokensDetails.CachedTokens,
			"non-OpenRouter channel must amplify CachedTokens")
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