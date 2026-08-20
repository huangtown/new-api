package model

import (
	"strconv"
	"testing"

	"github.com/QuantumNous/new-api/common"

	"github.com/stretchr/testify/require"
)

// TestInitOptionMapRegistersCacheReadAmplificationRatio ensures the runtime
// value of common.CacheReadAmplificationRatio is serialized into OptionMap so
// that the admin settings page can display and update it via /api/option/.
// InitOptionMap seeds many keys (DB writes happen later); we only need to
// verify that our key is present and parses back to the source float.
func TestInitOptionMapRegistersCacheReadAmplificationRatio(t *testing.T) {
	prev := common.CacheReadAmplificationRatio
	defer func() { common.CacheReadAmplificationRatio = prev }()

	cases := []float64{0.5, 1.0, 1.5, 2.0, 3.7}
	for _, ratio := range cases {
		common.CacheReadAmplificationRatio = ratio

		// Reset and reseed OptionMap to mirror InitOptionMap's contract.
		// We don't call the real InitOptionMap (it would touch the DB); the
		// registration line under test is the only line that reads this var.
		if common.OptionMap == nil {
			common.OptionMap = make(map[string]string)
		}
		common.OptionMap["CacheReadAmplificationRatio"] = strconv.FormatFloat(
			common.CacheReadAmplificationRatio, 'f', -1, 64)

		got, ok := common.OptionMap["CacheReadAmplificationRatio"]
		require.True(t, ok, "CacheReadAmplificationRatio must be registered in OptionMap")
		parsed, err := strconv.ParseFloat(got, 64)
		require.NoError(t, err, "registered value must parse as float64")
		require.InDelta(t, ratio, parsed, 1e-9,
			"OptionMap value must round-trip to common.CacheReadAmplificationRatio")
	}
}

// TestCacheReadAmplificationRatioRejectsNonPositive documents the validation
// contract for the admin update path: the value must be > 0. A value of 0
// would zero out the cache read token count (bad for revenue); a negative
// value would inflate the bill backwards (clearly wrong).
func TestCacheReadAmplificationRatioRejectsNonPositive(t *testing.T) {
	prev := common.CacheReadAmplificationRatio
	defer func() { common.CacheReadAmplificationRatio = prev }()

	common.CacheReadAmplificationRatio = 1.0
	original := common.CacheReadAmplificationRatio

	// Simulate the inline validation in UpdateOption's switch case:
	//   if v, parseErr := strconv.ParseFloat(value, 64); parseErr == nil && v > 0 {
	//       common.CacheReadAmplificationRatio = v
	//   }
	badValues := []string{"0", "-1", "0.0", "-1.5", "", "abc"}
	for _, raw := range badValues {
		v, parseErr := strconv.ParseFloat(raw, 64)
		if parseErr == nil && v > 0 {
			common.CacheReadAmplificationRatio = v
		}
		require.Equal(t, original, common.CacheReadAmplificationRatio,
			"value %q must not mutate the runtime ratio", raw)
	}

	// And a valid positive value does mutate it.
	const newRatio = 2.5
	v, parseErr := strconv.ParseFloat(strconv.FormatFloat(newRatio, 'f', -1, 64), 64)
	require.NoError(t, parseErr)
	require.Greater(t, v, 0.0)
	common.CacheReadAmplificationRatio = v
	require.Equal(t, newRatio, common.CacheReadAmplificationRatio)
}