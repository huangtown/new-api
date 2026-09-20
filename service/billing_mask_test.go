/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
package service

import (
	"net/http"
	"strconv"
	"testing"

	"github.com/QuantumNous/new-api/types"
	"github.com/stretchr/testify/require"
)

// newTestPolicy 直接构造策略，绕开 ResolveBillingMaskPolicy 的库查询，
// 让这些用例不依赖数据库。身份判定本身由 TestResolveStatusCode 等覆盖。
func newTestPolicy(keywords []string, statusCode int, message string) BillingMaskPolicy {
	return BillingMaskPolicy{
		Active:     true,
		StatusCode: statusCode,
		Message:    message,
		keywords:   keywords,
	}
}

// 掩盖状态码必须落在 400..599。1xx 会让 net/http 当成非提交响应，客户端
// 收到 HTTP 200 且 body 为空；2xx/3xx 会把失败伪装成成功。
func TestResolveMaskStatusCode(t *testing.T) {
	cases := []struct {
		raw  string
		want int
	}{
		{"504", 504},
		{"400", 400},
		{"599", 599},
		{" 502 ", 502}, // 带空白
		{"", fallbackMaskCode},
		{"abc", fallbackMaskCode},
		{"0", fallbackMaskCode},
		{"100", fallbackMaskCode}, // 1xx：客户端会收到 200 + 空 body
		{"199", fallbackMaskCode},
		{"200", fallbackMaskCode}, // 2xx：失败伪装成成功
		{"302", fallbackMaskCode},
		{"399", fallbackMaskCode},
		{"600", fallbackMaskCode},
		{"999", fallbackMaskCode},
		{"-1", fallbackMaskCode},
	}
	for _, tc := range cases {
		t.Run("raw="+tc.raw, func(t *testing.T) {
			require.Equal(t, tc.want, resolveMaskStatusCode(tc.raw))
		})
	}
}

// 纯空白的自定义文案等同于未配置，否则会给客户端交付一条空白报错。
func TestResolveMaskMessage(t *testing.T) {
	require.Equal(t, "服务暂时不可用", resolveMaskMessage("服务暂时不可用", 504))
	require.Equal(t, "trimmed", resolveMaskMessage("  trimmed  ", 504))

	fallback := "bad response status code 504"
	for _, blank := range []string{"", " ", "\t", "\n", "   \t\n  "} {
		require.Equal(t, fallback, resolveMaskMessage(blank, 504),
			"纯空白文案 %q 应回退到默认格式", blank)
	}
}

// 空关键词会命中所有报错（strings.Contains(s, "") 恒为 true），必须被剔除。
func TestParseMaskKeywords(t *testing.T) {
	require.Equal(t, []string{"RMB", "额度"}, parseMaskKeywords("RMB,额度"))
	require.Equal(t, []string{"RMB", "额度"}, parseMaskKeywords(" RMB , 额度 "))
	require.Equal(t, []string{"RMB"}, parseMaskKeywords("RMB,,"), "空元素应被剔除")
	require.Empty(t, parseMaskKeywords(""))
	require.Empty(t, parseMaskKeywords(",,,"))
	require.Empty(t, parseMaskKeywords("  ,  "))
}

// 检测必须同时覆盖 e.Err 与渲染后的消息。ErrOptionWithHideErrMsg 只替换 Err，
// 上游计费原文仍留在 RelayError 里并最终发给客户端——只看 e.Err 会漏掉它。
func TestApplyDetectsLeakInRenderedMessage(t *testing.T) {
	policy := newTestPolicy([]string{"余额", "balance"}, 504, "服务暂时不可用")

	e := types.WithOpenAIError(types.OpenAIError{
		Message: "Insufficient account balance, 余额不足请充值",
		Type:    "openai_error",
	}, 403)
	// 模拟 relay/channel/api_request.go:538 的 ErrOptionWithHideErrMsg：
	// 只替换 Err，RelayError 保持上游原文。
	e.SetMessage("upstream error: do request failed")

	require.NotContains(t, e.Error(), "balance",
		"前提：e.Err 已被替换，不含关键词")
	require.Contains(t, e.RenderedMessage(), "balance",
		"前提：客户端实际拿到的消息仍含上游原文")

	require.True(t, policy.Apply(e), "掩盖必须触发")
	require.Equal(t, 504, e.StatusCode)
	require.Equal(t, "服务暂时不可用", e.ToOpenAIError().Message)
	require.NotContains(t, e.ToOpenAIError().Message, "balance")
}

// 错误日志是独立于 HTTP 响应的第二个出口，用户可通过 /api/log/self 读回。
func TestMaskLogContent(t *testing.T) {
	policy := newTestPolicy([]string{"余额", "充值"}, 504, "服务暂时不可用")

	leaked := "status_code=403, 账户余额不足，请充值"
	got := policy.MaskLogContent(leaked)
	require.NotContains(t, got, "余额", "错误日志泄露了计费原文")
	require.NotContains(t, got, "充值")
	require.Equal(t, "status_code=504, 服务暂时不可用", got)

	benign := "status_code=500, upstream channel timeout"
	require.Equal(t, benign, policy.MaskLogContent(benign),
		"无关报错不应被改写，否则运维排障失去信息")
}

// 超级管理员透传：策略未激活时，两个出口都原样放行。
func TestInactivePolicyPassesThrough(t *testing.T) {
	var policy BillingMaskPolicy // Active 为 false，等同超管/功能关闭

	e := types.WithOpenAIError(types.OpenAIError{
		Message: "账户余额不足",
		Type:    "openai_error",
	}, 403)

	require.False(t, policy.Apply(e), "超管不应被掩盖")
	require.Equal(t, 403, e.StatusCode, "原始状态码应保留")
	require.Contains(t, e.ToOpenAIError().Message, "余额", "超管应看到原文")

	leaked := "status_code=403, 账户余额不足"
	require.Equal(t, leaked, policy.MaskLogContent(leaked), "超管的日志应保留原文")
}

// 管理员配置的掩盖文案是固定字符串，不应再过 MaskSensitiveInfo，
// 否则 "Contact support@acme.com" 会被打码成 "support@***.com"。
func TestCustomMessageNotMangledByMasking(t *testing.T) {
	cases := []string{
		"Contact support@acme.com",
		"see status.acme.com for details",
		"Please retry later.Contact support",
		"服务暂时不可用，请联系 support@acme.com",
	}
	for _, custom := range cases {
		t.Run(custom, func(t *testing.T) {
			policy := newTestPolicy([]string{"余额"}, 504, custom)
			e := types.WithOpenAIError(types.OpenAIError{
				Message: "余额不足",
				Type:    "openai_error",
			}, 403)

			require.True(t, policy.Apply(e))
			require.Equal(t, custom, e.ToOpenAIError().Message,
				"自定义文案被 MaskSensitiveInfo 篡改了")
			require.Equal(t, custom, e.ToClaudeError().Message)
		})
	}
}

// 掩盖后两种渲染格式都必须输出配置文案，而不是 errorType 字面量。
func TestMaskedErrorRendersConsistently(t *testing.T) {
	const custom = "bad response"

	t.Run("上游 OpenAI 格式", func(t *testing.T) {
		policy := newTestPolicy([]string{"Insufficient account balance"}, 504, custom)
		e := types.WithOpenAIError(types.OpenAIError{
			Message: "Insufficient account balance",
			Type:    "openai_error",
			Code:    "insufficient_user_quota",
		}, 403)

		require.True(t, policy.Apply(e))
		require.Equal(t, custom, e.ToOpenAIError().Message)
		require.Equal(t, custom, e.ToClaudeError().Message)
	})

	t.Run("上游 Claude 格式", func(t *testing.T) {
		policy := newTestPolicy([]string{"余额"}, 504, custom)
		e := types.WithClaudeError(types.ClaudeError{
			Message: "余额不足",
			Type:    "invalid_request_error",
		}, 403)

		require.True(t, policy.Apply(e))
		require.Equal(t, custom, e.ToClaudeError().Message)
		require.Equal(t, custom, e.ToOpenAIError().Message)
	})
}

// 掩盖后不得残留任何上游痕迹。
func TestMaskClearsUpstreamTraces(t *testing.T) {
	policy := newTestPolicy([]string{"余额"}, 504, "服务暂时不可用")
	e := types.WithOpenAIError(types.OpenAIError{
		Message:  "余额不足",
		Type:     "openai_error",
		Metadata: []byte(`{"raw":"余额不足，当前余额 0.01 元"}`),
	}, 403)

	require.True(t, policy.Apply(e))
	require.Nil(t, e.RelayError, "RelayError 未清除")
	require.Nil(t, e.Metadata, "Metadata 未清除，OpenRouter 路径会在此泄露原文")
	require.NotContains(t, e.ToOpenAIError().Message, "余额")
}

// 无关报错不应被掩盖，否则运维与用户都失去可诊断信息。
func TestBenignErrorsPassThrough(t *testing.T) {
	policy := newTestPolicy([]string{"余额", "充值"}, 504, "服务暂时不可用")
	e := types.WithOpenAIError(types.OpenAIError{
		Message: "upstream channel timed out after 30s",
		Type:    "openai_error",
	}, 500)

	require.False(t, policy.Apply(e), "无关报错被误掩盖")
	require.Equal(t, 500, e.StatusCode)
	require.Contains(t, e.ToOpenAIError().Message, "timed out")
}

// 掩盖状态码必须是客户端能真实收到的错误码。
func TestMaskStatusCodeIsDeliverable(t *testing.T) {
	require.GreaterOrEqual(t, minMaskStatusCode, http.StatusBadRequest,
		"下界不得低于 400，否则会把失败伪装成成功")
	require.Equal(t, http.StatusServiceUnavailable, fallbackMaskCode)

	for code := 100; code < 400; code++ {
		require.Equal(t, fallbackMaskCode, resolveMaskStatusCode(strconv.Itoa(code)),
			"非错误状态码 %d 应回退", code)
	}
}
