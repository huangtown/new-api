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
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relaykit/types"
)

// BillingMaskPolicy 是一次请求内报错掩盖的完整决策结果。
//
// 掩盖必须应用在 *每一个* 会把报错文本交给用户的出口上，不只是 HTTP 响应体：
// 错误日志（用户可通过 /api/log/self 读回）同样是一个出口。任何遗漏的出口
// 都让掩盖形同虚设，因为用户换个页面就能读到原文。
type BillingMaskPolicy struct {
	// Active 为 true 时本次请求需要掩盖。超级管理员恒为 false（透传原文）。
	Active bool
	// StatusCode 掩盖后返回的 HTTP 状态码，已保证落在 400..599。
	StatusCode int
	// Message 掩盖后的报错文本，已保证非空且非纯空白。
	Message string

	keywords []string
}

// 掩盖状态码必须是真实的错误码。1xx 会让 net/http 当成非提交响应，客户端
// 收到 200 且 body 为空；2xx/3xx 会把失败伪装成成功，还会污染限流计数
// （middleware/model-rate-limit.go）与渠道亲和性绑定（middleware/distributor.go）。
const (
	minMaskStatusCode = http.StatusBadRequest          // 400
	maxMaskStatusCode = 599
	fallbackMaskCode  = http.StatusServiceUnavailable  // 503
)

// ResolveBillingMaskPolicy 按用户身份和全局配置解析掩盖策略。
//
// userId 取自 gin context 的 "id"。中继路径只经过 TokenAuth，context 里
// 没有 role（只有会话态的 UserAuth 才写，见 middleware/auth.go），所以
// 这里必须按 id 回查用户角色，不能读 c.GetInt("role")。
func ResolveBillingMaskPolicy(userId int) BillingMaskPolicy {
	if !common.BillingErrorMaskingEnabled {
		return BillingMaskPolicy{}
	}
	// 只有超级管理员透传原始报错，管理员和普通用户一律掩盖。
	if model.IsRootUser(userId) {
		return BillingMaskPolicy{}
	}

	keywords := parseMaskKeywords(common.BillingErrorMaskingKeywords)
	if len(keywords) == 0 {
		// 未配置关键词时不掩盖任何东西。若不在此拦截，空关键词会命中
		// 每一条报错（strings.Contains(s, "") 恒为 true）。
		return BillingMaskPolicy{}
	}

	statusCode := resolveMaskStatusCode(common.BillingErrorMaskingStatusCode)
	return BillingMaskPolicy{
		Active:     true,
		StatusCode: statusCode,
		Message:    resolveMaskMessage(common.BillingErrorMaskingMessage, statusCode),
		keywords:   keywords,
	}
}

func parseMaskKeywords(raw string) []string {
	parts := strings.Split(raw, ",")
	keywords := make([]string, 0, len(parts))
	for _, part := range parts {
		if part = strings.TrimSpace(part); part != "" {
			keywords = append(keywords, part)
		}
	}
	return keywords
}

func resolveMaskStatusCode(raw string) int {
	code, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil || code < minMaskStatusCode || code > maxMaskStatusCode {
		return fallbackMaskCode
	}
	return code
}

func resolveMaskMessage(raw string, statusCode int) string {
	// 纯空白的自定义文案等同于未配置：交付一条空白报错对用户毫无意义。
	if msg := strings.TrimSpace(raw); msg != "" {
		return msg
	}
	return "bad response status code " + strconv.Itoa(statusCode)
}

// ShouldMask 判断一段报错文本是否命中计费关键词。
func (p BillingMaskPolicy) ShouldMask(message string) bool {
	if !p.Active {
		return false
	}
	return types.ContainsBillingKeywords(message, p.keywords)
}

// Apply 就地掩盖一个报错。命中时返回 true。
//
// 检测同时覆盖 e.Err 与渲染后的消息：两者可能不同（例如
// ErrOptionWithHideErrMsg 只替换 Err，上游原文仍留在 RelayError 里，
// 而客户端拿到的正是后者）。只看其一会漏掉真正泄露的那条文本。
func (p BillingMaskPolicy) Apply(e *types.NewAPIError) bool {
	if !p.Active || e == nil {
		return false
	}
	if !p.ShouldMask(e.Error()) && !p.ShouldMask(e.RenderedMessage()) {
		return false
	}
	e.ApplyBillingMask(p.StatusCode, p.Message)
	return true
}

// MaskLogContent 掩盖要写入错误日志的文本。
//
// 错误日志是独立于 HTTP 响应的第二个出口：用户可通过 /api/log/self 读回
// 自己的日志行，而 formatUserLogs 并不删除 Content 字段。若日志不掩盖，
// 用户在响应里看到的是掩盖文案，去日志页就能看到上游计费原文。
func (p BillingMaskPolicy) MaskLogContent(content string) string {
	if !p.Active || !p.ShouldMask(content) {
		return content
	}
	return "status_code=" + strconv.Itoa(p.StatusCode) + ", " + p.Message
}
