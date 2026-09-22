/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
package service

import (
	"net/http/httptest"
	"testing"

	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/relaykit/types"
	hosttypes "github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// amplifiedQuota bills a request whose upstream reported realCached cache-read
// tokens, after the adaptor amplified that count by `ratio`.
func amplifiedQuota(t *testing.T, realCached int, ratio float64) int {
	t.Helper()
	gin.SetMode(gin.TestMode)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())

	amplified := int(float64(realCached) * ratio)
	usage := &dto.Usage{
		PromptTokens:     1000,
		CompletionTokens: 0,
		PromptTokensDetails: dto.InputTokenDetails{
			CachedTokens: amplified,
		},
	}
	relayInfo := &relaycommon.RelayInfo{
		RelayFormat:             types.RelayFormatOpenAI,
		FinalRequestRelayFormat: types.RelayFormatOpenAI,
		OriginModelName:         "gpt-4o-mini",
		PriceData: hosttypes.PriceData{
			ModelRatio:         1,
			CacheRatio:         0.1,
			CompletionRatio:    1,
			ImageRatio:         1,
			CacheCreationRatio: 1,
			GroupRatioInfo:     hosttypes.GroupRatioInfo{GroupRatio: 1},
		},
	}
	if ratio != 1 {
		relayInfo.OriginalCachedTokens = realCached
	}
	return calculateTextQuotaSummary(ctx, relayInfo, usage).Quota
}

// Raising the cache read amplification ratio must raise the bill. The cached
// slice is carved out of prompt_tokens and re-priced at cacheRatio, so the
// removal has to use the upstream's real count; using the inflated number
// shrank the base charge and made a higher multiplier bill LESS — at ratio 3
// the 1000/400/0.1 case fell from 640 to 120.
func TestCacheReadAmplificationRaisesTheBill(t *testing.T) {
	const realCached = 400

	base := amplifiedQuota(t, realCached, 1)
	doubled := amplifiedQuota(t, realCached, 2)
	tripled := amplifiedQuota(t, realCached, 3)

	require.Greater(t, doubled, base, "ratio 2 must cost more than ratio 1")
	require.Greater(t, tripled, doubled, "ratio 3 must cost more than ratio 2")

	// base = (1000-400) + 400*0.1 = 640; each extra multiple adds 400*0.1 = 40.
	require.Equal(t, base+40, doubled)
	require.Equal(t, base+80, tripled)
}

// A ratio below 1 is a discount and must lower the bill, never raise it.
func TestCacheReadAmplificationBelowOneLowersTheBill(t *testing.T) {
	const realCached = 400
	require.Less(t, amplifiedQuota(t, realCached, 0.5), amplifiedQuota(t, realCached, 1))
}

// Without amplification OriginalCachedTokens stays zero and the subtraction
// must fall back to the reported count, leaving existing billing untouched.
func TestCacheReadBillingUnchangedWithoutAmplification(t *testing.T) {
	require.Equal(t, 640, amplifiedQuota(t, 400, 1))
}
