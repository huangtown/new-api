package openai

import (
	"net/http"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/constant"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// newChatStreamTestContext builds a chat-completions streaming test fixture.
// It reuses newImageTestContext for the gin/httptest plumbing and flips the
// relay mode so OaiStreamHandler takes the chat path.
func newChatStreamTestContext(t *testing.T, body string) (*gin.Context, *relaycommon.RelayInfo, *http.Response) {
	t.Helper()
	c, _, resp, info := newImageTestContext(t, body, "text/event-stream", true)
	info.RelayMode = relayconstant.RelayModeChatCompletions
	info.RelayFormat = types.RelayFormatOpenAI
	info.UpstreamModelName = "gpt-4o"
	info.SetEstimatePromptTokens(1000)
	return c, info, resp
}

// TestOaiStreamHandlerStopsOnMidStreamError locks the core fix: an upstream
// that opens the stream with 200 and then emits an error chunk must be
// treated as a failed request, not billed as a 1-token success.
func TestOaiStreamHandlerStopsOnMidStreamError(t *testing.T) {
	oldMode := gin.Mode()
	gin.SetMode(gin.TestMode)
	t.Cleanup(func() { gin.SetMode(oldMode) })
	oldTimeout := constant.StreamingTimeout
	constant.StreamingTimeout = 30
	t.Cleanup(func() { constant.StreamingTimeout = oldTimeout })

	cases := []struct {
		name       string
		errType    string
		wantStatus int
	}{
		{"server_error maps to 500", "server_error", http.StatusInternalServerError},
		{"rate_limit_error maps to 429", "rate_limit_error", http.StatusTooManyRequests},
		{"invalid_request_error maps to 400", "invalid_request_error", http.StatusBadRequest},
		{"unknown type falls back to 500", "something_new", http.StatusInternalServerError},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// A priming delta with a single space — this is what produced the
			// "completion_tokens=1" fingerprint in production — followed by
			// the error chunk, then a clean close with no [DONE].
			body := strings.Join([]string{
				`data: {"id":"x","object":"chat.completion.chunk","created":1,"model":"gpt-4o","choices":[{"index":0,"delta":{"role":"assistant","content":" "}}]}`,
				``,
				`data: {"error":{"message":"upstream exploded","type":"` + tc.errType + `","code":"boom"}}`,
				``,
			}, "\n")
			c, info, resp := newChatStreamTestContext(t, body)

			usage, err := OaiStreamHandler(c, info, resp)

			require.Nil(t, usage, "usage must be nil so the caller cannot bill")
			require.NotNil(t, err, "an upstream error chunk must surface as an error")
			require.Equal(t, tc.wantStatus, err.StatusCode)
			oaiErr := err.ToOpenAIError()
			require.Equal(t, "upstream exploded", oaiErr.Message)
			require.Equal(t, tc.errType, oaiErr.Type)

			// The error must be recorded so the consume log (if any) shows
			// status=error. EndReason is not asserted: when the error chunk is
			// the last line, the scanner goroutine's EOF and the handler's
			// sr.Stop race, and SetEndReason is first-write-wins — either
			// value is acceptable because HasErrors() is what drives the
			// "error" verdict in appendStreamStatus.
			require.NotNil(t, info.StreamStatus)
			require.True(t, info.StreamStatus.HasErrors())
		})
	}
}

// TestOaiStreamHandlerErrorAsFirstChunk covers the "immediate failure" shape:
// no content deltas at all before the error.
func TestOaiStreamHandlerErrorAsFirstChunk(t *testing.T) {
	oldMode := gin.Mode()
	gin.SetMode(gin.TestMode)
	t.Cleanup(func() { gin.SetMode(oldMode) })
	oldTimeout := constant.StreamingTimeout
	constant.StreamingTimeout = 30
	t.Cleanup(func() { constant.StreamingTimeout = oldTimeout })

	body := "data: {\"error\":{\"message\":\"nope\",\"type\":\"server_error\"}}\n\n"
	c, info, resp := newChatStreamTestContext(t, body)

	usage, err := OaiStreamHandler(c, info, resp)
	require.Nil(t, usage)
	require.NotNil(t, err)
	require.Equal(t, http.StatusInternalServerError, err.StatusCode)
}

// TestOaiStreamHandlerNormalStreamUnaffected is the regression guard: a
// well-formed stream with [DONE] and a trailing usage chunk must still
// succeed and return the upstream usage untouched.
func TestOaiStreamHandlerNormalStreamUnaffected(t *testing.T) {
	oldMode := gin.Mode()
	gin.SetMode(gin.TestMode)
	t.Cleanup(func() { gin.SetMode(oldMode) })
	oldTimeout := constant.StreamingTimeout
	constant.StreamingTimeout = 30
	t.Cleanup(func() { constant.StreamingTimeout = oldTimeout })

	body := strings.Join([]string{
		`data: {"id":"x","object":"chat.completion.chunk","created":1,"model":"gpt-4o","choices":[{"index":0,"delta":{"role":"assistant","content":"Hello"}}]}`,
		``,
		`data: {"id":"x","object":"chat.completion.chunk","created":1,"model":"gpt-4o","choices":[{"index":0,"delta":{"content":" world"}}]}`,
		``,
		`data: {"id":"x","object":"chat.completion.chunk","created":1,"model":"gpt-4o","choices":[{"index":0,"delta":{},"finish_reason":"stop"}],"usage":{"prompt_tokens":10,"completion_tokens":2,"total_tokens":12}}`,
		``,
		`data: [DONE]`,
		``,
	}, "\n")
	c, info, resp := newChatStreamTestContext(t, body)

	usage, err := OaiStreamHandler(c, info, resp)
	require.Nil(t, err)
	require.NotNil(t, usage)
	require.Equal(t, 10, usage.PromptTokens)
	require.Equal(t, 2, usage.CompletionTokens)
	require.Equal(t, relaycommon.StreamEndReasonDone, info.StreamStatus.EndReason)
	require.False(t, info.StreamStatus.HasErrors())
}

// TestOaiStreamHandlerEmptyErrorObjectNotTreatedAsError guards the false-
// positive edge: a chunk with a structurally-present but empty "error" must
// not abort an otherwise healthy stream.
func TestOaiStreamHandlerEmptyErrorObjectNotTreatedAsError(t *testing.T) {
	oldMode := gin.Mode()
	gin.SetMode(gin.TestMode)
	t.Cleanup(func() { gin.SetMode(oldMode) })
	oldTimeout := constant.StreamingTimeout
	constant.StreamingTimeout = 30
	t.Cleanup(func() { constant.StreamingTimeout = oldTimeout })

	body := strings.Join([]string{
		`data: {"id":"x","object":"chat.completion.chunk","created":1,"model":"gpt-4o","choices":[{"index":0,"delta":{"content":"ok"}}],"error":{}}`,
		``,
		`data: [DONE]`,
		``,
	}, "\n")
	c, info, resp := newChatStreamTestContext(t, body)

	usage, err := OaiStreamHandler(c, info, resp)
	require.Nil(t, err, "empty error object must not abort the stream")
	require.NotNil(t, usage)
}

func TestDetectOpenAIStreamError(t *testing.T) {
	cases := []struct {
		name string
		data string
		want bool
	}{
		{"nil for empty", "", false},
		{"nil for [DONE]", "[DONE]", false},
		{"nil for non-json", "not json", false},
		{"nil for normal chunk", `{"choices":[{"delta":{"content":"hi"}}]}`, false},
		{"nil for empty error object", `{"error":{}}`, false},
		{"nil for null error", `{"error":null}`, false},
		{"hit for message-only error", `{"error":{"message":"x"}}`, true},
		{"hit for type-only error", `{"error":{"type":"server_error"}}`, true},
		{"hit for code-only error", `{"error":{"code":500}}`, true},
		{"hit for string error", `{"error":"boom"}`, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := detectOpenAIStreamError(tc.data)
			require.Equal(t, tc.want, got != nil, "data=%q", tc.data)
		})
	}
}