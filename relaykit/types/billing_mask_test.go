package types

import (
	"strings"
	"testing"
)

// Regression test: an unconfigured (empty) keyword list must NOT mask errors.
// strings.Split("", ",") yields []string{""}, and strings.Contains(s, "") is
// always true, so before the TrimSpace/empty guard this masked every error.
func TestContainsBillingKeywords_EmptyConfigDoesNotMatch(t *testing.T) {
	cases := []struct {
		name     string
		keywords []string
	}{
		{"empty config from strings.Split(\"\", \",\")", []string{""}},
		{"whitespace-only keyword", []string{"   "}},
		{"trailing comma leaves empty element", []string{"RMB", ""}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if containsBillingKeywords("upstream channel timed out after 30s", tc.keywords) {
				t.Errorf("benign error was matched by keywords %#v", tc.keywords)
			}
		})
	}
}

func TestContainsBillingKeywords_RealKeywordsStillMatch(t *testing.T) {
	keywords := []string{"RMB", "额度", "余额", "充值"}
	cases := []struct {
		msg  string
		want bool
	}{
		{"insufficient 额度 remaining", true},
		{"your rmb balance is low", true}, // case-insensitive
		{"用户余额不足", true},
		{"upstream channel timed out after 30s", false},
		{"model not found", false},
	}
	for _, tc := range cases {
		if got := containsBillingKeywords(tc.msg, keywords); got != tc.want {
			t.Errorf("containsBillingKeywords(%q) = %v, want %v", tc.msg, got, tc.want)
		}
	}
}

// Keywords with surrounding whitespace (e.g. "RMB, 额度") must still match.
func TestContainsBillingKeywords_TrimsWhitespace(t *testing.T) {
	if !containsBillingKeywords("insufficient 额度", []string{"RMB", " 额度 "}) {
		t.Error("whitespace-padded keyword failed to match")
	}
}

func TestMaskBillingErrorForNonAdmin(t *testing.T) {
	const masked = "bad response status code 524"
	newErr := func() *NewAPIError {
		return NewError(errStr("余额不足，请充值"), ErrorCodeInvalidRequest)
	}

	t.Run("普通用户匹配关键词时被掩盖", func(t *testing.T) {
		e := newErr()
		e.MaskBillingErrorForNonAdmin(false, true, []string{"余额"}, 524, masked)
		if e.StatusCode != 524 {
			t.Errorf("StatusCode = %d, want 524", e.StatusCode)
		}
		if e.Error() == "余额不足，请充值" {
			t.Error("original billing message leaked to non-root user")
		}
		if e.RelayError != nil {
			t.Error("RelayError was not cleared")
		}
	})

	t.Run("普通管理员（role=10）也应被掩盖（新需求）", func(t *testing.T) {
		e := newErr()
		// isRootUser=false 表示不是超级管理员（role < 100）
		e.MaskBillingErrorForNonAdmin(false, true, []string{"余额"}, 524, masked)
		if e.StatusCode != 524 {
			t.Error("admin error should be masked under new requirement")
		}
	})

	t.Run("超级管理员（role >= 100）看到原始错误", func(t *testing.T) {
		e := newErr()
		// isRootUser=true 表示超级管理员
		e.MaskBillingErrorForNonAdmin(true, true, []string{"余额"}, 524, masked)
		if e.StatusCode == 524 {
			t.Error("root user error was masked")
		}
	})

	t.Run("禁用功能时不掩盖", func(t *testing.T) {
		e := newErr()
		e.MaskBillingErrorForNonAdmin(false, false, []string{"余额"}, 524, masked)
		if e.StatusCode == 524 {
			t.Error("masked despite feature being disabled")
		}
	})

	t.Run("空关键词配置不掩盖", func(t *testing.T) {
		e := NewError(errStr("upstream timeout"), ErrorCodeInvalidRequest)
		e.MaskBillingErrorForNonAdmin(false, true, []string{""}, 524, masked)
		if e.StatusCode == 524 {
			t.Error("benign error masked under empty keyword config")
		}
	})
}

type errStr string

func (e errStr) Error() string { return string(e) }

// 掩盖后必须同时重置 errorType。此前只清了 RelayError，ToOpenAIError()
// 仍走 ErrorTypeOpenAIError 分支，对 nil RelayError 断言失败 -> 空消息 ->
// 回退成 errorType 字面量，客户端看到 "openai_error" 而不是配置的掩盖文案。
func TestMaskPreservesCustomMessageAcrossFormats(t *testing.T) {
	const custom = "bad response"

	t.Run("上游 OpenAI 格式错误", func(t *testing.T) {
		e := WithOpenAIError(OpenAIError{
			Message: "Insufficient account balance",
			Type:    "openai_error",
			Code:    "insufficient_user_quota",
		}, 403)

		e.MaskBillingErrorForNonAdmin(false, true,
			[]string{"Insufficient account balance"}, 504, custom)

		if e.StatusCode != 504 {
			t.Errorf("StatusCode = %d, want 504", e.StatusCode)
		}
		if got := e.ToOpenAIError().Message; got != custom {
			t.Errorf("ToOpenAIError().Message = %q, want %q", got, custom)
		}
		if got := e.ToClaudeError().Message; got != custom {
			t.Errorf("ToClaudeError().Message = %q, want %q", got, custom)
		}
	})

	t.Run("上游 Claude 格式错误", func(t *testing.T) {
		e := WithClaudeError(ClaudeError{
			Message: "余额不足",
			Type:    "invalid_request_error",
		}, 403)

		e.MaskBillingErrorForNonAdmin(false, true, []string{"余额"}, 504, custom)

		if got := e.ToClaudeError().Message; got != custom {
			t.Errorf("ToClaudeError().Message = %q, want %q", got, custom)
		}
		if got := e.ToOpenAIError().Message; got != custom {
			t.Errorf("ToOpenAIError().Message = %q, want %q", got, custom)
		}
	})

	t.Run("掩盖后不泄露上游原文", func(t *testing.T) {
		e := WithOpenAIError(OpenAIError{
			Message: "Insufficient account balance",
			Type:    "openai_error",
		}, 403)

		e.MaskBillingErrorForNonAdmin(false, true,
			[]string{"Insufficient account balance"}, 504, custom)

		if e.RelayError != nil {
			t.Error("RelayError 未清除，存在泄露风险")
		}
		if strings.Contains(e.ToOpenAIError().Message, "Insufficient") {
			t.Error("上游原文泄露到掩盖后的消息里")
		}
	})
}

// maskMessage 为空时调用方（relay.go）会填入默认文案，这里确认
// 掩盖本身不会把空消息又回退成 errorType 字面量。
func TestMaskWithDefaultMessage(t *testing.T) {
	e := WithOpenAIError(OpenAIError{
		Message: "余额不足",
		Type:    "openai_error",
	}, 403)

	e.MaskBillingErrorForNonAdmin(false, true, []string{"余额"}, 504,
		"bad response status code 504")

	if got := e.ToOpenAIError().Message; got != "bad response status code 504" {
		t.Errorf("Message = %q, want default mask text", got)
	}
}
