package service

import (
	"fmt"
	"strings"

	"github.com/QuantumNous/new-api/common"
)

// NotifyError sends an error notification email to all configured admin recipients.
// If ErrorEmailNotifyEnabled is false or no recipients are configured, it silently returns.
func NotifyError(subject string, errDetail string) {
	if !common.ErrorEmailNotifyEnabled {
		return
	}
	if common.ErrorEmailNotifyRecipients == "" {
		return
	}

	recipients := strings.Split(common.ErrorEmailNotifyRecipients, ",")
	for _, recipient := range recipients {
		recipient = strings.TrimSpace(recipient)
		if recipient == "" {
			continue
		}
		content := buildErrorEmailContent(subject, errDetail)
		err := common.SendEmail(
			fmt.Sprintf("[%s Error] %s", common.SystemName, subject),
			recipient,
			content,
		)
		if err != nil {
			common.SysError(fmt.Sprintf("failed to send error notification email to %s: %v", recipient, err))
		}
	}
}

func buildErrorEmailContent(subject string, errDetail string) string {
	return fmt.Sprintf(`<!DOCTYPE html>
<html>
<head><meta charset="utf-8"></head>
<body style="font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;">
	<h2>🚨 %s Error Notification</h2>
	<p><strong>Error Type:</strong> %s</p>
	<div style="background:#f8f8f8; border:1px solid #e0e0e0; border-radius:4px; padding:12px;">
		<pre style="margin:0; white-space:pre-wrap; word-break:break-word; font-size:13px;">%s</pre>
	</div>
	<p style="color:#999; font-size:12px;">This is an automated error notification from %s.</p>
</body>
</html>`, common.SystemName, subject, errDetail, common.SystemName)
}
