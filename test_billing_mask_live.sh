#!/bin/bash
# 实测报错掩盖功能 - 向真实后端发送请求并验证响应
#
# 该脚本会 PUT /api/option/ 覆写掩盖相关配置，只指向本地或你自己的测试实例。

API_BASE="${1:-http://localhost:3000}"
ADMIN_TOKEN="${2}"
USER_TOKEN="${3}"

if [ -z "$ADMIN_TOKEN" ] || [ -z "$USER_TOKEN" ]; then
  echo "Usage: $0 <api_base> <admin_token> <user_token>"
  echo "Example: $0 http://localhost:3000 sk-admin-xxx sk-user-xxx"
  exit 1
fi

echo "=== 测试环境 ==="
echo "API: $API_BASE"
echo "管理员token: ${ADMIN_TOKEN:0:12}..."
echo "普通用户token: ${USER_TOKEN:0:12}..."
echo ""

# 1. 检查当前配置
echo "=== 1. 检查报错掩盖配置 ==="
curl -s "$API_BASE/api/option/" -H "Authorization: Bearer $ADMIN_TOKEN" | \
  jq -r '.data[] | select(.key | test("BillingErrorMask")) | "\(.key) = \(.value)"' || echo "获取配置失败"
echo ""

# 2. 临时启用功能（如果未启用）
echo "=== 2. 启用报错掩盖功能 ==="
curl -s -X PUT "$API_BASE/api/option/" \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"key":"BillingErrorMaskingEnabled","value":true}' | jq -c '.success'

curl -s -X PUT "$API_BASE/api/option/" \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"key":"BillingErrorMaskingKeywords","value":"余额,充值,RMB,quota"}' | jq -c '.success'

curl -s -X PUT "$API_BASE/api/option/" \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"key":"BillingErrorMaskingStatusCode","value":"524"}' | jq -c '.success'
echo ""

# 3. 构造一个会触发计费错误的请求（这里模拟，实际需要真实场景）
echo "=== 3. 模拟触发计费错误 ==="
echo "（实际测试需要一个会返回包含关键词报错的真实渠道）"
echo "这里演示如何检查响应掩盖逻辑："
echo ""

# 4. 用管理员token请求 - 应看到真实错误
echo "=== 4. 管理员请求（应看到真实错误）==="
RESP_ADMIN=$(curl -s -X POST "$API_BASE/v1/chat/completions" \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-3.5-turbo",
    "messages": [{"role":"user","content":"test"}],
    "max_tokens": 5
  }')
echo "$RESP_ADMIN" | jq -c 'if .error then .error else "SUCCESS" end' | head -c 200
echo ""

# 5. 用普通用户token请求 - 如果触发关键词应被掩盖
echo "=== 5. 普通用户请求（如触发关键词应被掩盖）==="
RESP_USER=$(curl -s -X POST "$API_BASE/v1/chat/completions" \
  -H "Authorization: Bearer $USER_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-3.5-turbo",
    "messages": [{"role":"user","content":"test"}],
    "max_tokens": 5
  }')
echo "$RESP_USER" | jq -c 'if .error then .error else "SUCCESS" end' | head -c 200
echo ""

echo "=== 测试说明 ==="
echo "1. 如果管理员看到包含'余额/充值'等关键词的详细错误，说明不掩盖管理员"
echo "2. 如果普通用户看到 'bad response status code 524' 且不含关键词，说明掩盖生效"
echo "3. 如果两者都正常返回（SUCCESS），说明当前没有触发计费错误"
echo ""
echo "要真正验证此功能，需要："
echo "  - 一个会返回包含'余额/充值'关键词错误的上游渠道"
echo "  - 或手动在 relay.go 中插入测试错误"
