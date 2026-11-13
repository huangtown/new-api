package operation_setting

import (
	"fmt"
	"strconv"
	"strings"
	"sync/atomic"

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

var channelRelayTimeouts atomic.Value
var channelFirstTokenTimeouts atomic.Value
var channelResponseHeaderTimeouts atomic.Value
var channelRelayTimeoutEnabled atomic.Bool
var channelFirstTokenTimeoutEnabled atomic.Bool
var channelResponseHeaderTimeoutEnabled atomic.Bool

func init() {
	channelRelayTimeouts.Store(map[int]int{})
	channelFirstTokenTimeouts.Store(map[int]int{})
	channelResponseHeaderTimeouts.Store(map[int]int{})
	channelRelayTimeoutEnabled.Store(true)
	channelFirstTokenTimeoutEnabled.Store(true)
	channelResponseHeaderTimeoutEnabled.Store(true)
}

// The setters only touch the atomic flag; model.updateOptionMap owns the
// OptionMap write under its lock, so writing the map here would race.
func SetChannelRelayTimeoutEnabled(enabled bool) {
	channelRelayTimeoutEnabled.Store(enabled)
}

func IsChannelRelayTimeoutEnabled() bool {
	return channelRelayTimeoutEnabled.Load()
}

func SetChannelFirstTokenTimeoutEnabled(enabled bool) {
	channelFirstTokenTimeoutEnabled.Store(enabled)
}

func IsChannelFirstTokenTimeoutEnabled() bool {
	return channelFirstTokenTimeoutEnabled.Load()
}

func SetChannelResponseHeaderTimeoutEnabled(enabled bool) {
	channelResponseHeaderTimeoutEnabled.Store(enabled)
}

func IsChannelResponseHeaderTimeoutEnabled() bool {
	return channelResponseHeaderTimeoutEnabled.Load()
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
	timeouts, err := ParseChannelRelayTimeouts(value)
	if err != nil {
		return err
	}
	channelRelayTimeouts.Store(timeouts)
	return nil
}

func ChannelRelayTimeouts2JSONString() string {
	timeouts := channelRelayTimeouts.Load().(map[int]int)
	jsonBytes, err := common.Marshal(timeouts)
	if err != nil {
		return "{}"
	}
	return string(jsonBytes)
}

func GetChannelRelayTimeout(channelID int) (int, bool) {
	if channelID <= 0 || !channelRelayTimeoutEnabled.Load() {
		return 0, false
	}
	timeouts := channelRelayTimeouts.Load().(map[int]int)
	timeoutSeconds, ok := timeouts[channelID]
	return timeoutSeconds, ok
}

func UpdateChannelFirstTokenTimeoutsByJSONString(value string) error {
	timeouts, err := ParseChannelFirstTokenTimeouts(value)
	if err != nil {
		return err
	}
	channelFirstTokenTimeouts.Store(timeouts)
	return nil
}

func ChannelFirstTokenTimeouts2JSONString() string {
	timeouts := channelFirstTokenTimeouts.Load().(map[int]int)
	jsonBytes, err := common.Marshal(timeouts)
	if err != nil {
		return "{}"
	}
	return string(jsonBytes)
}

// GetChannelFirstTokenTimeout returns the per-channel first-token budget.
func GetChannelFirstTokenTimeout(channelID int) (int, bool) {
	if channelID <= 0 {
		return 0, false
	}
	timeouts := channelFirstTokenTimeouts.Load().(map[int]int)
	timeoutSeconds, ok := timeouts[channelID]
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
	timeouts, err := ParseChannelResponseHeaderTimeouts(value)
	if err != nil {
		return err
	}
	channelResponseHeaderTimeouts.Store(timeouts)
	return nil
}

func ChannelResponseHeaderTimeouts2JSONString() string {
	timeouts := channelResponseHeaderTimeouts.Load().(map[int]int)
	jsonBytes, err := common.Marshal(timeouts)
	if err != nil {
		return "{}"
	}
	return string(jsonBytes)
}

// GetChannelResponseHeaderTimeout returns the per-channel response-header
// budget. The feature switch gates it the same way the other two gates work.
func GetChannelResponseHeaderTimeout(channelID int) (int, bool) {
	if channelID <= 0 || !channelResponseHeaderTimeoutEnabled.Load() {
		return 0, false
	}
	timeouts := channelResponseHeaderTimeouts.Load().(map[int]int)
	timeoutSeconds, ok := timeouts[channelID]
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
