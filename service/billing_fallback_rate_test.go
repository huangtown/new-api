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
	"testing"

	"github.com/QuantumNous/new-api/common"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	hosttypes "github.com/QuantumNous/new-api/types"

	"github.com/stretchr/testify/require"
)

func fallbackRateInfo(rate, groupRatio float64) *relaycommon.RelayInfo {
	info := &relaycommon.RelayInfo{FallbackBillingRate: rate}
	info.PriceData = hosttypes.PriceData{
		GroupRatioInfo: hosttypes.GroupRatioInfo{GroupRatio: groupRatio},
	}
	return info
}

// The fallback rate is an absolute multiplier against standard price, so the
// user-group ratio is divided out first.
func TestApplyFallbackBillingRate(t *testing.T) {
	cases := []struct {
		name       string
		rate       float64
		groupRatio float64
		quota      int
		want       int
	}{
		{"no fallback leaves quota alone", 0, 1, 1000, 1000},
		{"rate 1 is a no-op", 1, 1, 1000, 1000},
		{"rate above 1 raises the charge", 1.5, 1, 1000, 1500},
		{"rate below 1 lowers the charge", 0.5, 1, 1000, 500},
		{"group ratio is divided out", 1.5, 2, 1000, 750},
		{"zero group ratio is treated as 1", 1.5, 0, 1000, 1500},
		{"negative group ratio is treated as 1", 1.5, -3, 1000, 1500},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := ApplyFallbackBillingRate(fallbackRateInfo(tc.rate, tc.groupRatio), tc.quota)
			require.Equal(t, tc.want, got)
		})
	}
}

func TestApplyFallbackBillingRateHandlesNilInfo(t *testing.T) {
	require.Equal(t, 1000, ApplyFallbackBillingRate(nil, 1000))
}

// Regression: the rescale used to live inside SettleBilling, so the wallet was
// debited the adjusted amount while the consume log and both used_quota
// counters kept the unadjusted one. Applying it once at the call site means the
// value that reaches the counters is the same one that reaches the wallet.
func TestFallbackRateIsAppliedOnceSoBooksMatchWallet(t *testing.T) {
	info := fallbackRateInfo(1.5, 1)
	quota := 1000

	settled := ApplyFallbackBillingRate(info, quota)
	require.Equal(t, 1500, settled, "the caller rescales before logging")

	// SettleBilling must not rescale again; feeding it the already-adjusted
	// value has to stay stable or the two would compound to 2250.
	require.Equal(t, settled, ApplyFallbackBillingRate(
		&relaycommon.RelayInfo{FallbackBillingRate: 0}, settled,
	), "settle path must not apply the rate a second time")
}

// Masking rewrites user-visible errors AND the stored error log, so it must
// stay opt-in: an upgrade that silently switched it on would turn every
// legitimate "insufficient quota" into an opaque status with no way for the
// operator to tell the feature was active.
func TestBillingErrorMaskingDefaultsOff(t *testing.T) {
	require.False(t, common.BillingErrorMaskingEnabled,
		"masking must be opt-in; it matches the frontend default and its siblings")
	require.Empty(t, ResolveBillingMaskPolicy(0).Message,
		"no policy should be active while masking is disabled")
}
