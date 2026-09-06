# 流式请求上游失败被误计费 — 事故分析与修复

- **影响范围**：`/v1/messages` 流式请求，上游为原生 Anthropic 或 AWS Bedrock（Claude 系列）
- **现象**：上游记录为 HTTP 500，本站记录为成功扣费、`completion_tokens=1`
- **修复提交**：`9b2df74b`（主修复）、`7a80ea1d`（同类漏洞，OpenAI 路径）
- **状态**：已修复并推送到 `main`

---

## 1. 现象

一条来自客户的 `/v1/messages` 流式请求 X，本站转发给上游渠道 A 后：

| 位置 | 记录 |
|---|---|
| 上游 A 的站点 | 请求 XA，**HTTP 500** |
| 本站消费日志 | 请求 X，**成功**、有输入/输出 token、**已扣费**、`is_stream=true`、`completion_tokens=1` |

同一次调用，两边结论相反。用户按失败的请求付了钱。

### 日志证据

本站日志 `other` 字段（已脱敏）：

```json
{
  "admin_info": {
    "local_count_tokens": true,
    "usage_billing_path": "local"
  },
  "frt": 43805,
  "request_conversion": ["Claude Messages"],
  "request_path": "/v1/messages",
  "stream_status": { "end_reason": "eof", "status": "ok" },
  "usage_semantic": "anthropic"
}
```

四个字段互相印证：

| 字段 | 值 | 说明 |
|---|---|---|
| `stream_status.end_reason` | `eof` | 上游**没发 `message_stop` 就关了连接** |
| `stream_status.status` | `ok` | 代码把 `eof` 当作正常结束 |
| `admin_info.local_count_tokens` | `true` | 输出 token 是**本地估算的兜底值**，上游从未返回最终 usage |
| `frt` | 43805 ms | 首帧等了 43.8 秒，上游本身已在挣扎 |

---

## 2. 根因

### 2.1 Anthropic 流式协议的时序

一个正常的 Messages 流：

```
event: message_start        ← 携带 input_tokens（真实值）和 output_tokens: 1（占位符）
event: content_block_start
event: content_block_delta  ← 0 到 N 个
event: content_block_stop
event: message_delta        ← 携带 stop_reason 和最终 output_tokens   ★ 必有
event: message_stop
```

关键点：**`input_tokens` 在第一帧就给了，最终 `output_tokens` 在倒数第二帧才给**。`message_start` 里的 `output_tokens: 1` 是 Anthropic 的占位值，不是真实输出量。

### 2.2 上游失败时发生了什么

Anthropic 后端在流中途崩溃时，常见行为是**直接关闭 TCP 连接，不发任何 `error` 事件**。于是本站收到：

```
event: message_start        ← PromptTokens = 1234，CompletionTokens = 1（占位）
event: content_block_start
event: content_block_delta  ← 可能有一两帧
<TCP FIN>                   ← 没有 message_delta，没有 message_stop，没有 error
```

### 2.3 代码为什么把它当成功

六道防线依次失守，每一道单独都能拦住，但一道都没拦：

| # | 位置 | 行为 | 问题 |
|---|---|---|---|
| 1 | `relay/helper/stream_scanner.go:289` | 连接干净关闭 → `EndReason = eof` | 无法区分"完成"和"截断" |
| 2 | `relay/common/stream_status.go:93` | `IsNormalEnd()` 把 `eof` 列为正常 | 截断被判定为正常结束 |
| 3 | `relay/channel/claude/relay-claude.go:120` | `GetClaudeError()` 只在收到 `error` 事件时触发 | 上游根本没发 `error` 事件，检测无从触发 |
| 4 | `relay/channel/claude/relay-claude.go:162` `HandleStreamFinalResponse` | `!claudeInfo.Done` 时用本地估算合成 usage | 这是兜底逻辑，但它把"上游失败"也兜进去了，且保留了占位的 `CompletionTokens=1` |
| 5 | `relay/channel/claude/relay-claude.go:226`（修复前） | 无条件 `return claudeInfo.Usage, nil` | 从未检查流是否真正完成 |
| 6 | `service/text_quota.go` `PostTextConsumeQuota` | 只看 `TotalTokens != 0` 就扣费 | `PromptTokens` 来自 `message_start`，永远非零 |

原作者其实知道这个问题——`relay-claude.go:161-163` 留着一个空的 `if` 块：

```go
if claudeInfo.Usage.PromptTokens == 0 {
    //上游出错
}
```

### 2.4 `completion_tokens=1` 的来源

不是估算出来的。`message_start` 里 Anthropic 直接发 `"output_tokens": 1`，被写进 `claudeInfo.Usage.CompletionTokens`。后续兜底逻辑看到它非零，就原样保留了。

### 2.5 请求 ID 变化（X → XA）不是 bug

本站出站请求**不带任何 request-id header**（`relay/channel/api_request.go` 中无相关写入）。上游看到的 XA 是上游自己生成的。两个 ID 格式相似只是时间戳前缀的巧合。

---

## 3. 修复方案

### 3.1 核心判据

`claudeInfo.Done` 只在 `FormatClaudeResponseInfo` 处理 `message_delta` 时置为 `true`（`service/relayconvert/internal/claude_messages/to_oai_chat_resp.go:389`）。每一个正常完成的 Anthropic 流**必有** `message_delta`。因此：

> **流结束时 `Done == false` = 上游未正常完成，不应计费。**

这个判据比检查 `end_reason` 更可靠：它不依赖 TCP 层如何关闭，只看协议语义。

### 3.2 改动

**`relay/channel/claude/relay-claude.go`**

新增 `CheckClaudeStreamTruncated(info, claudeInfo)`（第 254 行）：

- `Done == false` → 返回 `502 Bad Gateway`，错误码 `ErrorCodeBadResponse`
- 错误消息带上 `end_reason`，便于日志诊断
- 标记 `ErrOptionWithSkipRetry()`——见 3.3

`ClaudeStreamHandler`（第 222 行）在 `HandleStreamFinalResponse` 之前调用它。命中时返回 `(nil, err)`：

- 调用方 `claude_handler.go` 看到 `newAPIError != nil`，跳过 `PostTextConsumeQuota`
- `controller/relay.go:191-200` 退回预扣额度
- 日志 `stream_status.status` 变为 `error`

**`relay/channel/aws/relay-aws.go`**（Bedrock）

- `awsStreamHandler` 事件循环结束后补查 `stream.Err()`（第 291 行）。AWS SDK 的 `Events()` channel 在正常完成和传输失败时都会关闭，不查 `Err()` 就分不清
- 同样调用 `CheckClaudeStreamTruncated`（第 295 行）

### 3.3 为什么必须 skip-retry

检测到截断时，SSE 响应头和 `message_start` **已经发给客户端了**。502 落在默认重试范围 `500-503` 内，`ErrorCodeBadResponse` 又不在 `alwaysSkipRetryCodes` 里。不加标记的话，`controller/relay.go` 的 `shouldRetry` 会换一个渠道重发，把第二条流**拼接到客户端正在读的响应里**。

### 3.4 明确不改的地方

| 不改 | 理由 |
|---|---|
| `IsNormalEnd()` 对 `eof` 的判定 | 有些非标上游确实不发 `[DONE]` 就关连接，改这里会误伤 |
| `HandleStreamFinalResponse` 的兜底估算 | `Done == true` 但 `CompletionTokens == 0` 是合法场景（tool_use 无文本输出），兜底逻辑对它是正确的 |
| 显式 `error` 事件的处理 | 已经正确，且在 scanner 回调里就 `sr.Stop`，优先级高于新增检查 |

---

## 4. 修复后的行为

| 场景 | 修复前 | 修复后 |
|---|---|---|
| 上游中途断连，无 `message_delta` | 成功、扣费、`completion_tokens=1`、`status=ok` | **502、不扣费、退回预扣、`status=error`、不重试** |
| 上游发 `error` 事件 | 按错误类型映射状态码、不扣费 | 不变 |
| 正常完成 | 按 `message_delta` 的 `output_tokens` 计费 | 不变 |
| `message_delta` 报 `output_tokens=0`（tool_use） | 兜底估算后计费 | 不变 |
| 收到 `message_delta` 后、`message_stop` 前断连 | 计费 | 不变（`message_delta` 已带完整计费数据，这是对的） |

客户端视角：之前收到截断的流 + 伪造的 `message_stop`；现在流在截断处停止。HTTP 状态码因为已经 flush 无法改变，这是 SSE 协议的固有限制。

---

## 5. 验证

### 单元测试

`relay/channel/claude/stream_truncation_test.go`，使用真实 Anthropic 线上格式（`event:` + `data:` + 空行）：

| 测试 | 覆盖 |
|---|---|
| `TestClaudeStreamHandlerRejectsTruncatedStream` | 4 个截断点（`message_start` 后 / `content_block_start` 后 / 一个 delta 后 / `content_block_stop` 后），全部返回 nil usage + 502 + `end_reason=eof` |
| `TestClaudeStreamHandlerNormalStreamUnaffected` | 完整流返回 `message_delta` 的 `output_tokens=42`，不是占位的 1 |
| `TestClaudeStreamHandlerZeroOutputTokensStillCompletes` | tool_use 场景 `output_tokens=0` 不被误杀 |
| `TestClaudeStreamHandlerErrorEventStillPropagates` | 显式 `overloaded_error` 仍映射为 529 |
| `TestCheckClaudeStreamTruncated` | nil 守卫、`end_reason` 透传、skip-retry 标记 |

### 回归

- `go build ./...` 通过
- `go test ./relay/... ./service/... ./dto/... ./controller/...` 无新增失败
- 仅剩 2 个 pre-existing 失败（`relay/channel/aws` 的 `TestDoAwsClientRequest_*` 和 `service` 的 `TestObserveChannelAffinityUsageCacheByRelayFormat_MixedMode`），在 clean `main` 上同样失败，与本次改动无关

### 上线后如何确认

再次遇到上游 500 时，本站日志应变为：

```json
"stream_status": { "end_reason": "eof", "status": "error", ... }
```

且**不产生消费记录**。

---

## 6. 相关：OpenAI 路径的同类漏洞（`7a80ea1d`）

排查过程中发现 `OaiStreamHandler`（OpenAI 兼容上游的流式路径）有一个平行漏洞：`dto.ChatCompletionsStreamResponse` 没有 `Error` 字段，上游在流中发的 `data: {"error":{...}}` 会被 JSON 解码器静默丢弃，同样导致误计费。已在 `7a80ea1d` 修复（加字段 + `detectOpenAIStreamError` + `sr.Stop`）。

该提交**对本次 Anthropic 场景无效**（路径不同），但漏洞真实存在，保留。

---

## 7. 遗留观察

- `IsNormalEnd()` 把 `eof` 和 `handler_stop` 都列为正常，语义上有歧义，但被多处依赖，未动
- `StreamScannerHandler` 没有返回值，所有错误信号都靠 `info.StreamStatus` 侧信道传递，调用方可以选择不看。这是本类 bug 的结构性温床，但重构范围太大，未在本次处理
- Gemini 流式路径未审计，可能有类似模式
