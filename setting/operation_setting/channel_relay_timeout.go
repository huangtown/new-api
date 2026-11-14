package operation_setting

import (
	"fmt"
	"strconv"
	"strings"
	"sync"

	"github.com/QuantumNous/new-api/common"
)

const (
	ChannelRelayTimeoutsOptionKey = "ChannelRelayTimeouts"
	// ChannelFirstTokenTimeoutsOptionKey bounds only the wait for the first
	// streamed token. It exists because a total timeout (ChannelRelayTimeouts /
	// RELAY_TIMEOUT) is meaningless for streaming: http.Client.Timeout covers the
	// whole response read, so a healthy long stream gets cut off mid-flight.
	ChannelFirstTokenTimeoutsOptionKey = "ChannelFirstTokenTimeouts"
	// The two switches below let an operator keep a configured map but turn its
	// effect off without deleting the entries. Both default to enabled so an
	// existing configuration keeps behaving as it did before the switches
	// existed.
	ChannelRelayTimeoutEnabledOptionKey      = "ChannelRelayTimeoutEnabled"
	ChannelFirstTokenTimeoutEnabledOptionKey = "ChannelFirstTokenTimeoutEnabled"
	// ChannelResponseHeaderTimeoutsOptionKey bounds how long the relay waits for
	// upstream response headers. This is the phase a slow upstream sits in when it
	// accepts the connection and returns nothing for a long time; neither the
	// total timeout (not applied to streams) nor the first-token timeout (starts
	// only after the headers arrive) covers it.
	ChannelResponseHeaderTimeoutsOptionKey       = "ChannelResponseHeaderTimeouts"
	ChannelResponseHeaderTimeoutEnabledOptionKey = "ChannelResponseHeaderTimeoutEnabled"
	MaxChannelRelayTimeoutSeconds                = 86400
)

// timeoutMapCache memoises the parse of one channel→seconds option.
//
// common.OptionMap is the single source of truth: it is written under its lock
// on save, so reading it on every lookup makes a configuration change visible
// to the very next request instead of waiting for the periodic sync loop.
// Parsing is the expensive part, so it is skipped unless the raw string changed.
type timeoutMapCache struct {
	key string

	mu   sync.RWMutex
	raw  string
	seen bool
	m    map[int]int
}

// load returns the current map for this option. The returned map is never
// mutated by callers, so sharing it between requests is safe.
func (c *timeoutMapCache) load() map[int]int {
	common.OptionMapRWMutex.RLock()
	raw := common.OptionMap[c.key]
	common.OptionMapRWMutex.RUnlock()

	c.mu.RLock()
	if c.seen && c.raw == raw {
		m := c.m
		c.mu.RUnlock()
		return m
	}
	c.mu.RUnlock()

	parsed, err := parseChannelTimeoutMap(raw, c.key)
	if err != nil {
		// A malformed value never reaches the store (validation rejects it),
		// but degrade to "no per-channel budget" rather than failing a relay.
		parsed = map[int]int{}
	}

	c.mu.Lock()
	c.raw = raw
	c.seen = true
	c.m = parsed
	c.mu.Unlock()
	return parsed
}

// updateFromRaw refreshes the cache only. It must NOT touch OptionMap: callers
// inside model.updateOptionMap already hold OptionMapRWMutex, and re-locking a
// non-reentrant RWMutex here would deadlock the save path.
func (c *timeoutMapCache) updateFromRaw(raw string) {
	parsed, err := parseChannelTimeoutMap(raw, c.key)
	if err != nil {
		parsed = map[int]int{}
	}
	c.mu.Lock()
	c.raw = raw
	c.seen = true
	c.m = parsed
	c.mu.Unlock()
}

func (c *timeoutMapCache) store(raw string) {
	common.OptionMapRWMutex.Lock()
	// OptionMap is normally allocated by InitOptionMap. Guard the write anyway:
	// tests and early-init paths can reach here before that runs, and a nil-map
	// assignment would panic the process.
	if common.OptionMap == nil {
		common.OptionMap = make(map[string]string)
	}
	common.OptionMap[c.key] = raw
	common.OptionMapRWMutex.Unlock()
	c.updateFromRaw(raw)
}

// rawJSON returns the last cached raw value without touching OptionMap.
//
// Several callers run while model.InitOptionMap holds OptionMapRWMutex
// (e.g. building the default map). Taking that lock again there would deadlock,
// since Go's RWMutex is not reentrant, so cache introspection must stay on the
// cache's own lock.
func (c *timeoutMapCache) rawJSON() string {
	c.mu.RLock()
	raw := c.raw
	c.mu.RUnlock()
	if strings.TrimSpace(raw) == "" {
		return "{}"
	}
	return raw
}

var channelRelayTimeoutCache = &timeoutMapCache{key: ChannelRelayTimeoutsOptionKey}
var channelFirstTokenTimeoutCache = &timeoutMapCache{key: ChannelFirstTokenTimeoutsOptionKey}
var channelResponseHeaderTimeoutCache = &timeoutMapCache{key: ChannelResponseHeaderTimeoutsOptionKey}

// flagCache mirrors one boolean option for the same reason: the accessors are
// called from InitOptionMap while the OptionMap lock is held, so they cannot
// read OptionMap themselves. Every write path (save, sync loop) goes through
// model.updateOptionMap, which calls the matching setter.
type flagCache struct {
	mu   sync.RWMutex
	seen bool
	v    bool
}

func (c *flagCache) get() bool {
	c.mu.RLock()
	seen, v := c.seen, c.v
	c.mu.RUnlock()
	if !seen {
		return true // switches default to enabled
	}
	return v
}

func (c *flagCache) set(v bool) {
	c.mu.Lock()
	c.seen = true
	c.v = v
	c.mu.Unlock()
}

var channelRelayTimeoutEnabled = &flagCache{}
var channelFirstTokenTimeoutEnabled = &flagCache{}
var channelResponseHeaderTimeoutEnabled = &flagCache{}

func SetChannelRelayTimeoutEnabled(enabled bool) {
	channelRelayTimeoutEnabled.set(enabled)
}

func IsChannelRelayTimeoutEnabled() bool {
	return channelRelayTimeoutEnabled.get()
}

func SetChannelFirstTokenTimeoutEnabled(enabled bool) {
	channelFirstTokenTimeoutEnabled.set(enabled)
}

func IsChannelFirstTokenTimeoutEnabled() bool {
	return channelFirstTokenTimeoutEnabled.get()
}

func SetChannelResponseHeaderTimeoutEnabled(enabled bool) {
	channelResponseHeaderTimeoutEnabled.set(enabled)
}

func IsChannelResponseHeaderTimeoutEnabled() bool {
	return channelResponseHeaderTimeoutEnabled.get()
}

// parseChannelTimeoutMap is shared by the total and first-token maps: both are
// JSON objects keyed by channel ID with a positive seconds value.
func parseChannelTimeoutMap(value string, label string) (map[int]int, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return map[int]int{}, nil
	}

	var raw map[string]int
	if err := common.UnmarshalJsonStr(value, &raw); err != nil {
		return nil, fmt.Errorf("invalid %s: %w", label, err)
	}
	if raw == nil {
		return nil, fmt.Errorf("%s must be a JSON object", label)
	}

	timeouts := make(map[int]int, len(raw))
	for rawChannelID, timeoutSeconds := range raw {
		channelID, err := strconv.Atoi(strings.TrimSpace(rawChannelID))
		if err != nil || channelID <= 0 {
			return nil, fmt.Errorf("channel ID %q must be a positive integer", rawChannelID)
		}
		if _, exists := timeouts[channelID]; exists {
			return nil, fmt.Errorf("channel ID %d is configured more than once", channelID)
		}
		if timeoutSeconds <= 0 || timeoutSeconds > MaxChannelRelayTimeoutSeconds {
			return nil, fmt.Errorf("channel %d timeout must be between 1 and %d seconds", channelID, MaxChannelRelayTimeoutSeconds)
		}
		timeouts[channelID] = timeoutSeconds
	}
	return timeouts, nil
}

func ParseChannelRelayTimeouts(value string) (map[int]int, error) {
	return parseChannelTimeoutMap(value, "channel relay timeouts")
}

func ParseChannelFirstTokenTimeouts(value string) (map[int]int, error) {
	return parseChannelTimeoutMap(value, "channel first token timeouts")
}

func ParseChannelResponseHeaderTimeouts(value string) (map[int]int, error) {
	return parseChannelTimeoutMap(value, "channel response header timeouts")
}

func UpdateChannelRelayTimeoutsByJSONString(value string) error {
	if _, err := ParseChannelRelayTimeouts(value); err != nil {
		return err
	}
	channelRelayTimeoutCache.store(value)
	return nil
}

// RefreshChannelRelayTimeoutCache reparses the per-channel map for callers that
// already wrote OptionMap themselves (model.updateOptionMap holds the lock).
func RefreshChannelRelayTimeoutCache(raw string) {
	channelRelayTimeoutCache.updateFromRaw(raw)
}

func ChannelRelayTimeouts2JSONString() string {
	return channelRelayTimeoutCache.rawJSON()
}

func GetChannelRelayTimeout(channelID int) (int, bool) {
	if channelID <= 0 || !IsChannelRelayTimeoutEnabled() {
		return 0, false
	}
	timeoutSeconds, ok := channelRelayTimeoutCache.load()[channelID]
	return timeoutSeconds, ok
}

func UpdateChannelFirstTokenTimeoutsByJSONString(value string) error {
	if _, err := ParseChannelFirstTokenTimeouts(value); err != nil {
		return err
	}
	channelFirstTokenTimeoutCache.store(value)
	return nil
}

// RefreshChannelFirstTokenTimeoutCache reparses the per-channel map for callers
// that already wrote OptionMap themselves.
func RefreshChannelFirstTokenTimeoutCache(raw string) {
	channelFirstTokenTimeoutCache.updateFromRaw(raw)
}

func ChannelFirstTokenTimeouts2JSONString() string {
	return channelFirstTokenTimeoutCache.rawJSON()
}

// GetChannelFirstTokenTimeout returns the per-channel first-token budget.
func GetChannelFirstTokenTimeout(channelID int) (int, bool) {
	if channelID <= 0 {
		return 0, false
	}
	timeoutSeconds, ok := channelFirstTokenTimeoutCache.load()[channelID]
	return timeoutSeconds, ok
}

// ResolveFirstTokenTimeout returns the first-token budget for a channel:
// the per-channel override when configured, otherwise the global env value.
// 0 means nothing is configured; whether a default is then applied is the
// caller's decision (see IsChannelFirstTokenTimeoutEnabled).
func ResolveFirstTokenTimeout(channelID int) int {
	if timeoutSeconds, ok := GetChannelFirstTokenTimeout(channelID); ok {
		return timeoutSeconds
	}
	return common.RelayFirstTokenTimeout
}

func UpdateChannelResponseHeaderTimeoutsByJSONString(value string) error {
	if _, err := ParseChannelResponseHeaderTimeouts(value); err != nil {
		return err
	}
	channelResponseHeaderTimeoutCache.store(value)
	return nil
}

// RefreshChannelResponseHeaderTimeoutCache reparses the per-channel map for
// callers that already wrote OptionMap themselves.
func RefreshChannelResponseHeaderTimeoutCache(raw string) {
	channelResponseHeaderTimeoutCache.updateFromRaw(raw)
}

func ChannelResponseHeaderTimeouts2JSONString() string {
	return channelResponseHeaderTimeoutCache.rawJSON()
}

// GetChannelResponseHeaderTimeout returns the per-channel response-header
// budget. The feature switch gates it the same way the other two gates work.
func GetChannelResponseHeaderTimeout(channelID int) (int, bool) {
	if channelID <= 0 || !IsChannelResponseHeaderTimeoutEnabled() {
		return 0, false
	}
	timeoutSeconds, ok := channelResponseHeaderTimeoutCache.load()[channelID]
	return timeoutSeconds, ok
}

// ResolveResponseHeaderTimeout returns the response-header budget for a
// channel: the per-channel override when configured, otherwise the global env
// value. 0 means no per-request bound; the transport-level
// RELAY_RESPONSE_HEADER_TIMEOUT still applies in that case.
func ResolveResponseHeaderTimeout(channelID int) int {
	if timeoutSeconds, ok := GetChannelResponseHeaderTimeout(channelID); ok {
		return timeoutSeconds
	}
	return common.RelayResponseHeaderTimeout
}
