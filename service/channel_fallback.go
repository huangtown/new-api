package service

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/types"
)

// ShouldTriggerFallback checks whether an error should trigger the fallback
// channel mechanism. It matches against configured status codes and keywords.
func ShouldTriggerFallback(err *types.NewAPIError) bool {
	if !common.FallbackEnabled {
		return false
	}
	if err == nil {
		return false
	}

	// Check status code match
	statusMatched := false
	statusCodes := splitAndTrim(common.FallbackStatusCodes, ",")
	if len(statusCodes) == 0 {
		statusMatched = true // no status code filter configured → match all
	}
	for _, sc := range statusCodes {
		if code, parseErr := strconv.Atoi(sc); parseErr == nil && code == err.StatusCode {
			statusMatched = true
			break
		}
	}
	if !statusMatched {
		return false
	}

	// Check keyword match in error message
	keywords := splitAndTrim(common.FallbackTriggerKeywords, ",")
	if len(keywords) == 0 {
		return false // no keywords configured
	}
	lowerMsg := strings.ToLower(err.Error())
	for _, kw := range keywords {
		if kw != "" && strings.Contains(lowerMsg, strings.ToLower(kw)) {
			return true
		}
	}
	return false
}

// GetFallbackChannel loads a fallback channel. It first checks the per-group
// configuration (GroupFallbackChannelIDs), then falls back to the global
// FallbackChannelIDs.
func GetFallbackChannel(usedChannelIds []int, group string) (*model.Channel, error) {
	var channelIDs string

	// 1) Try per-group config first
	if group != "" && common.GroupFallbackChannelIDs != "" {
		groupMap := make(map[string]string)
		if err := json.Unmarshal([]byte(common.GroupFallbackChannelIDs), &groupMap); err == nil {
			if v, ok := groupMap[group]; ok && v != "" {
				channelIDs = v
			}
		}
	}

	// 2) Fall back to global config
	if channelIDs == "" {
		channelIDs = common.FallbackChannelIDs
	}

	if channelIDs == "" {
		return nil, fmt.Errorf("no fallback channel IDs configured")
	}

	ids := splitAndTrim(channelIDs, ",")
	for _, idStr := range ids {
		id, err := strconv.Atoi(idStr)
		if err != nil {
			continue
		}
		// Skip channels already used
		if containsInt(usedChannelIds, id) {
			continue
		}
		channel, err := model.GetChannelById(id, true)
		if err != nil {
			common.SysLog(fmt.Sprintf("fallback channel #%d not found: %v", id, err))
			continue
		}
		if channel.Status != common.ChannelStatusEnabled {
			common.SysLog(fmt.Sprintf("fallback channel #%d is not enabled (status=%d)", id, channel.Status))
			continue
		}
		return channel, nil
	}
	return nil, fmt.Errorf("no available fallback channel found")
}

func splitAndTrim(s, sep string) []string {
	var result []string
	for _, part := range strings.Split(s, sep) {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

func containsInt(slice []int, val int) bool {
	for _, v := range slice {
		if v == val {
			return true
		}
	}
	return false
}
