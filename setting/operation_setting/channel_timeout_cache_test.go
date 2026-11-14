package operation_setting

import (
	"sync"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func resetTimeoutCaches(t *testing.T) {
	t.Helper()
	common.OptionMapRWMutex.Lock()
	if common.OptionMap == nil {
		common.OptionMap = make(map[string]string)
	}
	for _, key := range []string{
		ChannelRelayTimeoutsOptionKey,
		ChannelFirstTokenTimeoutsOptionKey,
		ChannelResponseHeaderTimeoutsOptionKey,
		ChannelRelayTimeoutEnabledOptionKey,
		ChannelFirstTokenTimeoutEnabledOptionKey,
		ChannelResponseHeaderTimeoutEnabledOptionKey,
	} {
		common.OptionMap[key] = "{}"
		delete(common.OptionMap, key)
	}
	common.OptionMapRWMutex.Unlock()

	refreshAllTimeoutCaches()
	t.Cleanup(refreshAllTimeoutCaches)
}

func refreshAllTimeoutCaches() {
	common.OptionMapRWMutex.RLock()
	relay := common.OptionMap[ChannelRelayTimeoutsOptionKey]
	first := common.OptionMap[ChannelFirstTokenTimeoutsOptionKey]
	header := common.OptionMap[ChannelResponseHeaderTimeoutsOptionKey]
	common.OptionMapRWMutex.RUnlock()

	RefreshChannelRelayTimeoutCache(relay)
	RefreshChannelFirstTokenTimeoutCache(first)
	RefreshChannelResponseHeaderTimeoutCache(header)
}

// A value written straight into OptionMap — the path model.updateOptionMap and
// the periodic sync both take — must be visible to the very next lookup. This
// is what makes a saved change take effect immediately instead of waiting up to
// one SYNC_FREQUENCY tick.
func TestTimeoutCacheSeesOptionMapChangeImmediately(t *testing.T) {
	resetTimeoutCaches(t)

	require.NoError(t, UpdateChannelResponseHeaderTimeoutsByJSONString(`{"54":12}`))
	got, ok := GetChannelResponseHeaderTimeout(54)
	require.True(t, ok)
	assert.Equal(t, 12, got)

	// Simulate another instance's sync (or a direct OptionMap write): the cache
	// must notice the raw string changed and reparse on the next read.
	common.OptionMapRWMutex.Lock()
	common.OptionMap[ChannelResponseHeaderTimeoutsOptionKey] = `{"54":30,"7":5}`
	common.OptionMapRWMutex.Unlock()

	got, ok = GetChannelResponseHeaderTimeout(54)
	require.True(t, ok)
	assert.Equal(t, 30, got, "cache must follow OptionMap without a manual refresh")

	got, ok = GetChannelResponseHeaderTimeout(7)
	require.True(t, ok)
	assert.Equal(t, 5, got)

	_, ok = GetChannelResponseHeaderTimeout(999)
	assert.False(t, ok)
}

// The save path (model.updateOptionMap) already holds OptionMapRWMutex when it
// dispatches to these refresh helpers. Re-locking a non-reentrant RWMutex there
// deadlocks the whole process, so the refresh path must never take that lock.
func TestRefreshDoesNotReenterOptionMapLock(t *testing.T) {
	resetTimeoutCaches(t)

	done := make(chan struct{})
	go func() {
		defer close(done)
		// Mirror model.updateOptionMap: hold the write lock, write the value,
		// then refresh the parse caches.
		common.OptionMapRWMutex.Lock()
		common.OptionMap[ChannelResponseHeaderTimeoutsOptionKey] = `{"54":12}`
		RefreshChannelResponseHeaderTimeoutCache(`{"54":12}`)
		RefreshChannelRelayTimeoutCache(`{"1":600}`)
		RefreshChannelFirstTokenTimeoutCache(`{"2":8}`)
		common.OptionMapRWMutex.Unlock()
	}()

	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("refresh deadlocked: it must not re-acquire OptionMapRWMutex while the caller holds it")
	}

	got, ok := GetChannelResponseHeaderTimeout(54)
	require.True(t, ok)
	assert.Equal(t, 12, got)
}

// A malformed raw value must degrade to "no per-channel budget" rather than
// panicking a relay request.
func TestTimeoutCacheDegradesOnMalformedValue(t *testing.T) {
	resetTimeoutCaches(t)

	common.OptionMapRWMutex.Lock()
	common.OptionMap[ChannelRelayTimeoutsOptionKey] = `{"not-a-channel":600}`
	common.OptionMapRWMutex.Unlock()

	_, ok := GetChannelRelayTimeout(1)
	assert.False(t, ok, "malformed config must not produce a budget")

	// A valid value after the malformed one must recover.
	common.OptionMapRWMutex.Lock()
	common.OptionMap[ChannelRelayTimeoutsOptionKey] = `{"1":600}`
	common.OptionMapRWMutex.Unlock()

	got, ok := GetChannelRelayTimeout(1)
	require.True(t, ok)
	assert.Equal(t, 600, got)
}

// The switch must gate the per-channel budget. It is backed by its own cache so
// that InitOptionMap can read it while holding the OptionMap lock, which means
// the setter — not a raw OptionMap write — is the write path.
func TestTimeoutSwitchGatesBudget(t *testing.T) {
	resetTimeoutCaches(t)

	require.NoError(t, UpdateChannelRelayTimeoutsByJSONString(`{"9":600}`))
	SetChannelRelayTimeoutEnabled(true)
	_, ok := GetChannelRelayTimeout(9)
	require.True(t, ok)

	SetChannelRelayTimeoutEnabled(false)
	_, ok = GetChannelRelayTimeout(9)
	assert.False(t, ok, "a disabled switch must not expose a budget")

	SetChannelRelayTimeoutEnabled(true)
	_, ok = GetChannelRelayTimeout(9)
	require.True(t, ok)
}

// Concurrent readers must not race with a writer replacing the cached map.
func TestTimeoutCacheConcurrentReadWrite(t *testing.T) {
	resetTimeoutCaches(t)

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		for i := 0; i < 200; i++ {
			common.OptionMapRWMutex.Lock()
			common.OptionMap[ChannelResponseHeaderTimeoutsOptionKey] = `{"54":12}`
			RefreshChannelResponseHeaderTimeoutCache(`{"54":12}`)
			common.OptionMapRWMutex.Unlock()
		}
	}()
	go func() {
		defer wg.Done()
		for i := 0; i < 200; i++ {
			_, _ = GetChannelResponseHeaderTimeout(54)
		}
	}()
	wg.Wait()
}
