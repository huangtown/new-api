package service

import (
	"fmt"
	"strings"

	"github.com/QuantumNous/new-api/common"
)

// NotifyError sends an error notification to all configured channels.
// Currently supports email and PushPlus.
func NotifyError(subject string, errDetail string) {
	common.NotifyError(subject, errDetail)
	sendPushPlusErrorNotify(subject, errDetail)
}

func sendPushPlusErrorNotify(subject string, errDetail string) {
	if !common.PushPlusEnabled || strings.TrimSpace(common.PushPlusToken) == "" {
		return
	}
	title := fmt.Sprintf("[%s Error] %s", common.SystemName, subject)
	if err := SendPushPlusNotify(common.PushPlusToken, common.PushPlusTopic, title, errDetail); err != nil {
		common.SysError(fmt.Sprintf("failed to send pushplus error notification: %v", err))
	}
}

// BuildRetryChainDetail returns a stable, human-readable retry chain for notifications.
func BuildRetryChainDetail(useChannel []string) string {
	if len(useChannel) == 0 {
		return ""
	}
	return fmt.Sprintf("\n重试链路: %s", strings.Join(useChannel, " -> "))
}
