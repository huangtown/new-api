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
	MaxChannelRelayTimeoutSeconds = 86400
)

var channelRelayTimeouts atomic.Value

func init() {
	channelRelayTimeouts.Store(map[int]int{})
}

func ParseChannelRelayTimeouts(value string) (map[int]int, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return map[int]int{}, nil
	}

	var raw map[string]int
	if err := common.UnmarshalJsonStr(value, &raw); err != nil {
		return nil, fmt.Errorf("invalid channel relay timeouts: %w", err)
	}
	if raw == nil {
		return nil, fmt.Errorf("channel relay timeouts must be a JSON object")
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
	if channelID <= 0 {
		return 0, false
	}
	timeouts := channelRelayTimeouts.Load().(map[int]int)
	timeoutSeconds, ok := timeouts[channelID]
	return timeoutSeconds, ok
}
