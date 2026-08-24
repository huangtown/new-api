package ratio_setting

import (
	"encoding/json"
	"testing"

	"github.com/QuantumNous/new-api/common"

	"github.com/stretchr/testify/require"
)

// TestResolveCacheReadAmplificationRatio exercises the per-group → global
// fallback chain documented on ResolveCacheReadAmplificationRatio. Phase 3
// introduced this helper so the wire layer can amplify per-using-group
// instead of relying solely on the global common.CacheReadAmplificationRatio.
func TestResolveCacheReadAmplificationRatio(t *testing.T) {
	// Snapshot and restore global so this test doesn't leak state to the rest
	// of the suite.
	prevGlobal := common.CacheReadAmplificationRatio
	defer func() { common.CacheReadAmplificationRatio = prevGlobal }()

	// Snapshot and restore per-group map so seeded defaults don't bleed in.
	prevPerGroup := GetGroupCacheReadAmplificationRatioCopy()
	defer func() { _ = UpdateGroupCacheReadAmplificationRatioByJSONString(mustJSON(t, prevPerGroup)) }()

	common.CacheReadAmplificationRatio = 1.0 // baseline
	require.NoError(t, UpdateGroupCacheReadAmplificationRatioByJSONString(`{"default":1,"vip":1.5,"svip":2}`))

	cases := []struct {
		name    string
		group   string
		global  float64
		want    float64
	}{
		{"per-group vip wins over global 1.0", "vip", 1.0, 1.5},
		{"per-group svip wins over global 2.5", "svip", 2.5, 2.0},
		{"per-group default wins over global 2.5", "default", 2.5, 1.0},
		{"unknown group falls back to global 1.5", "unknown-group", 1.5, 1.5},
		{"empty group name falls back to global 3.0", "", 3.0, 3.0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			common.CacheReadAmplificationRatio = tc.global
			require.Equal(t, tc.want, ResolveCacheReadAmplificationRatio(tc.group))
		})
	}

	// Per-group ratio of 0 is treated as "unset" and falls back to global.
	t.Run("zero in per-group map falls back to global", func(t *testing.T) {
		require.NoError(t, UpdateGroupCacheReadAmplificationRatioByJSONString(`{"zeros":0}`))
		common.CacheReadAmplificationRatio = 2.5
		require.Equal(t, 2.5, ResolveCacheReadAmplificationRatio("zeros"))
	})
}

// TestResolveCacheReadAmplificationRatioGlobalOnly verifies backward
// compatibility: Phase 1 deployments set only the global value (no per-group
// config). With no per-group entries, every lookup must return the global
// ratio verbatim, preserving Phase 1 behavior.
func TestResolveCacheReadAmplificationRatioGlobalOnly(t *testing.T) {
	prevGlobal := common.CacheReadAmplificationRatio
	defer func() { common.CacheReadAmplificationRatio = prevGlobal }()

	prevPerGroup := GetGroupCacheReadAmplificationRatioCopy()
	defer func() { _ = UpdateGroupCacheReadAmplificationRatioByJSONString(mustJSON(t, prevPerGroup)) }()

	require.NoError(t, UpdateGroupCacheReadAmplificationRatioByJSONString(`{}`))

	common.CacheReadAmplificationRatio = 2.0
	for _, g := range []string{"default", "vip", "svip", "anything"} {
		require.Equal(t, 2.0, ResolveCacheReadAmplificationRatio(g),
			"without per-group entries, every group must fall back to global")
	}
}

// TestCheckGroupCacheReadAmplificationRatio validates the >0 rule on JSON
// input. Negative values are rejected; zero is allowed (it means "unset" and
// will fall back to global at resolve time).
func TestCheckGroupCacheReadAmplificationRatio(t *testing.T) {
	require.NoError(t, CheckGroupCacheReadAmplificationRatio(`{"default":1,"vip":1.5}`))
	require.NoError(t, CheckGroupCacheReadAmplificationRatio(`{"default":0}`),
		"zero is allowed at validation time; resolver treats it as unset")
	require.Error(t, CheckGroupCacheReadAmplificationRatio(`{"vip":-0.5}`))
	require.Error(t, CheckGroupCacheReadAmplificationRatio(`{"vip":-1}`))
	require.Error(t, CheckGroupCacheReadAmplificationRatio(`{not json}`))
}

// TestGroupCacheReadAmplificationRatioRoundTrip verifies the option round-
// trip: Update* loads a JSON string into the in-memory map; Get*Copy exposes
// it; re-serializing and re-loading yields an equivalent map.
func TestGroupCacheReadAmplificationRatioRoundTrip(t *testing.T) {
	prev := GetGroupCacheReadAmplificationRatioCopy()
	defer func() { _ = UpdateGroupCacheReadAmplificationRatioByJSONString(mustJSON(t, prev)) }()

	want := map[string]float64{"default": 1, "vip": 1.5, "svip": 2.0}
	jsonStr := mustJSON(t, want)
	require.NoError(t, UpdateGroupCacheReadAmplificationRatioByJSONString(jsonStr))

	got := GetGroupCacheReadAmplificationRatioCopy()
	require.Equal(t, want, got,
		"round-trip JSON → map → JSON should preserve values")

	// Re-serialize the in-memory state and load it again; should be a no-op.
	round2 := mustJSON(t, got)
	require.NoError(t, UpdateGroupCacheReadAmplificationRatioByJSONString(round2))
	require.Equal(t, want, GetGroupCacheReadAmplificationRatioCopy())

	// ContainsGroupCacheReadAmplificationRatio should report membership correctly.
	require.True(t, ContainsGroupCacheReadAmplificationRatio("vip"))
	require.False(t, ContainsGroupCacheReadAmplificationRatio("missing"))

	// GroupCacheReadAmplificationRatio2JSONString should be non-empty.
	require.NotEmpty(t, GroupCacheReadAmplificationRatio2JSONString())
}

// TestGetGroupCacheReadAmplificationRatioMissingGroup documents that an
// unknown group returns 1.0 (mirrors GetGroupRatio's behaviour for missing
// groups) so the wire-layer no-op when ratio is 1.0 stays valid.
func TestGetGroupCacheReadAmplificationRatioMissingGroup(t *testing.T) {
	require.Equal(t, 1.0, GetGroupCacheReadAmplificationRatio("does-not-exist"))
}

func mustJSON(t *testing.T, v interface{}) string {
	t.Helper()
	b, err := json.Marshal(v)
	require.NoError(t, err)
	return string(b)
}