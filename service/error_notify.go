package service

import (
	"fmt"
	"strings"

	"github.com/QuantumNous/new-api/common"
)

// NotifyError sends an error notification email to all configured admin recipients.
// If ErrorEmailNotifyEnabled is false or no recipients are configured, it silently returns.
func NotifyError(subject string, errDetail string) {
	common.NotifyError(subject, errDetail)
}

// BuildRetryChainDetail returns a stable, human-readable retry chain for email.
func BuildRetryChainDetail(useChannel []string) string {
	if len(useChannel) == 0 {
		return ""
	}
	return fmt.Sprintf("\n重试链路: %s", strings.Join(useChannel, " -> "))
}
