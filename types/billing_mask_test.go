package types

import "testing"

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
