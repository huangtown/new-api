package common

import (
	"encoding/json"
	"fmt"
	"html"
	"strings"
)

// NotifyError sends an error notification email to all configured recipients.
func NotifyError(subject string, errDetail string) {
	if !ErrorEmailNotifyEnabled || strings.TrimSpace(ErrorEmailNotifyRecipients) == "" {
		return
	}
	for _, recipient := range strings.Split(ErrorEmailNotifyRecipients, ",") {
		recipient = strings.TrimSpace(recipient)
		if recipient == "" {
			continue
		}
		if err := SendEmail(fmt.Sprintf("[%s Error] %s", SystemName, subject), recipient, buildErrorEmailContent(subject, errDetail)); err != nil {
			SysError(fmt.Sprintf("failed to send error notification email to %s: %v", recipient, err))
		}
	}
}

func buildErrorEmailContent(subject, errDetail string) string {
	systemName := html.EscapeString(SystemName)
	return fmt.Sprintf(`<!DOCTYPE html><html><head><meta charset="utf-8"></head><body>
<h2>🚨 %s Error Notification</h2><p><strong>Error Type:</strong> %s</p>
<pre style="white-space:pre-wrap;word-break:break-word">%s</pre>
<p style="color:#999">This is an automated error notification from %s.</p>
</body></html>`, systemName, html.EscapeString(subject), html.EscapeString(errDetail), systemName)
}

// MarshalErrorLogDetail is retained for callers that need stable JSON details.
func MarshalErrorLogDetail(fields map[string]interface{}) string {
	b, err := json.MarshalIndent(fields, "", "  ")
	if err != nil {
		return ""
	}
	return string(b)
}
