package controller

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

// TestBillingErrorMaskE2E 端到端测试报错掩盖功能
func TestBillingErrorMaskE2E(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// 保存原配置以便恢复
	origEnabled := common.BillingErrorMaskingEnabled
	origKeywords := common.BillingErrorMaskingKeywords
	origStatusCode := common.BillingErrorMaskingStatusCode
	defer func() {
		common.BillingErrorMaskingEnabled = origEnabled
		common.BillingErrorMaskingKeywords = origKeywords
		common.BillingErrorMaskingStatusCode = origStatusCode
	}()

	// 启用报错掩盖功能
	common.BillingErrorMaskingEnabled = true
	common.BillingErrorMaskingKeywords = "余额,充值,RMB,quota"
	common.BillingErrorMaskingStatusCode = "524"

	tests := []struct {
		name              string
		userId            int
		userRole          int
		errorMsg          string
		expectStatus      int
		expectContains    string
		expectNotContains string
	}{
		{
			name:              "普通用户触发计费关键词 - 应被掩盖",
			userId:            100,
			userRole:          1, // 普通用户
			errorMsg:          "账户余额不足，请充值",
			expectStatus:      524,
			expectContains:    "bad response status code 524",
			expectNotContains: "余额",
		},
		{
			name:              "普通管理员触发计费关键词 - 应被掩盖（新需求）",
			userId:            10,
			userRole:          10, // 普通管理员
			errorMsg:          "账户余额不足，请充值",
			expectStatus:      524,
			expectContains:    "bad response status code 524",
			expectNotContains: "余额",
		},
		{
			name:              "超级管理员触发计费关键词 - 应看到真实错误",
			userId:            1,
			userRole:          100, // 超级管理员
			errorMsg:          "账户余额不足，请充值",
			expectStatus:      400,
			expectContains:    "余额",
			expectNotContains: "bad response status code",
		},
		{
			name:              "普通用户触发无关错误 - 不应被掩盖",
			userId:            100,
			userRole:          1,
			errorMsg:          "upstream channel timeout",
			expectStatus:      500,
			expectContains:    "timeout",
			expectNotContains: "bad response status code 524",
		},
		{
			name:              "大小写不敏感匹配（普通管理员）",
			userId:            10,
			userRole:          10,
			errorMsg:          "insufficient rmb balance",
			expectStatus:      524,
			expectContains:    "bad response status code 524",
			expectNotContains: "rmb",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// 创建测试上下文
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Set("id", tc.userId)
			c.Set("role", tc.userRole)

			// 模拟 relay.go:115-125 中的错误掩盖逻辑
			newAPIError := types.NewError(
				errStr(tc.errorMsg),
				types.ErrorCodeInvalidRequest,
			)
			if tc.errorMsg == "upstream channel timeout" {
				newAPIError.StatusCode = 500
			} else {
				newAPIError.StatusCode = 400
			}

			// 应用掩盖逻辑。注意 relay 路径只过 TokenAuth，context 里没有
			// role，所以生产代码用 model.IsRootUser(userId) 回查数据库；
			// 这里直接用用例声明的角色模拟其返回值。
			isRootUser := tc.userRole >= common.RoleRootUser
			keywords := strings.Split(common.BillingErrorMaskingKeywords, ",")
			statusCode := 524
			maskMessage := "bad response status code " + common.BillingErrorMaskingStatusCode
			newAPIError.MaskBillingErrorForNonAdmin(isRootUser, common.BillingErrorMaskingEnabled, keywords, statusCode, maskMessage)

			// 返回JSON（模拟relay.go的响应）
			c.JSON(newAPIError.StatusCode, gin.H{
				"error": newAPIError.ToOpenAIError(),
			})

			// 验证HTTP状态码
			assert.Equal(t, tc.expectStatus, w.Code, "HTTP status code mismatch")

			// 验证响应体
			body := w.Body.String()
			assert.Contains(t, body, tc.expectContains, "response should contain expected string")
			if tc.expectNotContains != "" {
				assert.NotContains(t, body, tc.expectNotContains, "response leaked sensitive info")
			}

			t.Logf("✓ HTTP %d | Body contains %q | Does NOT leak %q",
				w.Code, tc.expectContains, tc.expectNotContains)
		})
	}
}

type errStr string

func (e errStr) Error() string { return string(e) }

// TestEmptyKeywordsRegression 回归测试：空关键词不应掩盖所有错误
func TestEmptyKeywordsRegression(t *testing.T) {
	origEnabled := common.BillingErrorMaskingEnabled
	origKeywords := common.BillingErrorMaskingKeywords
	origStatusCode := common.BillingErrorMaskingStatusCode
	defer func() {
		common.BillingErrorMaskingEnabled = origEnabled
		common.BillingErrorMaskingKeywords = origKeywords
		common.BillingErrorMaskingStatusCode = origStatusCode
	}()

	common.BillingErrorMaskingEnabled = true
	common.BillingErrorMaskingKeywords = "" // 空配置
	common.BillingErrorMaskingStatusCode = "524"

	err := types.NewError(
		errStr("upstream channel timeout after 30s"),
		types.ErrorCodeInvalidRequest,
	)
	err.StatusCode = 500

	keywords := strings.Split(common.BillingErrorMaskingKeywords, ",")
	err.MaskBillingErrorForNonAdmin(false, true, keywords, 524, "bad response status code 524")

	assert.NotEqual(t, 524, err.StatusCode, "BUG REGRESSION: empty keywords masked unrelated error")
	assert.Equal(t, 500, err.StatusCode, "should preserve original status code")
	assert.Contains(t, err.Error(), "timeout", "should preserve original message")
}

// TestInvalidStatusCodeFallback 测试非法状态码回退机制
func TestInvalidStatusCodeFallback(t *testing.T) {
	tests := []struct {
		configValue    string
		expectFallback bool
		expectCode     int
	}{
		{"524", false, 524}, // 合法值
		{"200", false, 200}, // 合法值
		{"", true, 503},     // 空字符串 -> fallback
		{"abc", true, 503},  // 非数字 -> fallback
		{"999", true, 503},  // 超出范围 -> fallback
		{"0", true, 503},    // 小于100 -> fallback
	}

	for _, tc := range tests {
		t.Run("config="+tc.configValue, func(t *testing.T) {
			// 模拟 relay.go:119-123 的解析逻辑
			statusCode, err := strconv.Atoi(tc.configValue)
			if err != nil || statusCode < 100 || statusCode > 599 {
				statusCode = http.StatusServiceUnavailable // 503
			}

			assert.Equal(t, tc.expectCode, statusCode, "status code mismatch")
		})
	}
}
