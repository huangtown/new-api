package claude

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// newClaudeStreamTestContext builds a /v1/messages streaming fixture: a gin
// context backed by an httptest recorder, an upstream *http.Response whose
// body is the given SSE text, and a RelayInfo configured for the native
// Claude relay format so ClaudeStreamHandler takes the Claude passthrough
// branch (the one /v1/messages uses against Anthropic).
func newClaudeStreamTestContext(t *testing.T, body string) (*gin.Context, *http.Response, *relaycommon.RelayInfo) {
	t.Helper()
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)

	resp := &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader(body)),
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
	}
	info := &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{UpstreamModelName: "claude-opus-5"},
		IsStream:    true,
		RelayFormat: types.RelayFormatClaude,
	}
	info.SetEstimatePromptTokens(1000)
	return c, resp, info
}

func setStreamTestMode(t *testing.T) {
	t.Helper()
	oldMode := gin.Mode()
	gin.SetMode(gin.TestMode)
	t.Cleanup(func() { gin.SetMode(oldMode) })
	oldTimeout := constant.StreamingTimeout
	constant.StreamingTimeout = 30
	t.Cleanup(func() { constant.StreamingTimeout = oldTimeout })
}

// Real Anthropic wire shapes. message_start reports input_tokens and a
// placeholder output_tokens of 1 — that placeholder is exactly what surfaced
// as completion_tokens=1 in the production log this test reproduces.
const (
	sseMessageStart = `event: message_start
data: {"type":"message_start","message":{"id":"msg_01","type":"message","role":"assistant","model":"claude-opus-5","content":[],"stop_reason":null,"stop_sequence":null,"usage":{"input_tokens":1234,"output_tokens":1}}}

`
	sseContentBlockStart = `event: content_block_start
data: {"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}

`
	sseContentDelta = `event: content_block_delta
data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"Hello"}}

`
	sseContentBlockStop = `event: content_block_stop
data: {"type":"content_block_stop","index":0}

`
	sseMessageDelta = `event: message_delta
data: {"type":"message_delta","delta":{"stop_reason":"end_turn","stop_sequence":null},"usage":{"output_tokens":42}}

`
	sseMessageStop = `event: message_stop
data: {"type":"message_stop"}

`
	sseErrorEvent = `event: error
data: {"type":"error","error":{"type":"overloaded_error","message":"Overloaded"}}

`
)

// TestClaudeStreamHandlerRejectsTruncatedStream reproduces the production
// incident: upstream sends message_start (so PromptTokens and the placeholder
// CompletionTokens=1 are populated), streams a little content, then the
// backend dies and the TCP connection closes with no error event and no
// message_delta / message_stop. Before the fix this was billed as a success
// with completion_tokens=1 and stream_status.status="ok".
func TestClaudeStreamHandlerRejectsTruncatedStream(t *testing.T) {
	setStreamTestMode(t)

	cases := []struct {
		name string
		body string
	}{
		{"after message_start only", sseMessageStart},
		{"after content_block_start", sseMessageStart + sseContentBlockStart},
		{"after one content delta", sseMessageStart + sseContentBlockStart + sseContentDelta},
		{"after content_block_stop, before message_delta", sseMessageStart + sseContentBlockStart + sseContentDelta + sseContentBlockStop},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c, resp, info := newClaudeStreamTestContext(t, tc.body)

			usage, err := ClaudeStreamHandler(c, resp, info)

			require.Nil(t, usage, "truncated stream must not yield billable usage")
			require.NotNil(t, err, "truncated stream must surface as an error")
			require.Equal(t, http.StatusBadGateway, err.StatusCode)
			require.Contains(t, err.Error(), "before message_delta")
			require.Contains(t, err.Error(), "end_reason=eof",
				"error must carry the scanner's end reason for diagnosis")
			require.NotNil(t, info.StreamStatus)
			require.Equal(t, relaycommon.StreamEndReasonEOF, info.StreamStatus.EndReason)
		})
	}
}

// TestClaudeStreamHandlerNormalStreamUnaffected is the regression guard: a
// complete stream with message_delta + message_stop must still succeed and
// return the upstream-reported usage verbatim (not the placeholder).
func TestClaudeStreamHandlerNormalStreamUnaffected(t *testing.T) {
	setStreamTestMode(t)

	body := sseMessageStart + sseContentBlockStart + sseContentDelta + sseContentBlockStop + sseMessageDelta + sseMessageStop
	c, resp, info := newClaudeStreamTestContext(t, body)

	usage, err := ClaudeStreamHandler(c, resp, info)

	require.Nil(t, err)
	require.NotNil(t, usage)
	require.Equal(t, 1234, usage.PromptTokens, "input_tokens from message_start")
	require.Equal(t, 42, usage.CompletionTokens, "output_tokens from message_delta, not the placeholder 1")
	require.Equal(t, 1276, usage.TotalTokens)
	require.NotNil(t, info.StreamStatus)
	require.False(t, info.StreamStatus.HasErrors())
}

// TestClaudeStreamHandlerZeroOutputTokensStillCompletes guards the tool_use /
// empty-text case: message_delta arrives with output_tokens=0. Done is true,
// so the truncation guard must NOT fire; the existing fallback in
// HandleStreamFinalResponse handles it as before.
func TestClaudeStreamHandlerZeroOutputTokensStillCompletes(t *testing.T) {
	setStreamTestMode(t)

	zeroDelta := `event: message_delta
data: {"type":"message_delta","delta":{"stop_reason":"tool_use","stop_sequence":null},"usage":{"output_tokens":0}}

`
	body := sseMessageStart + zeroDelta + sseMessageStop
	c, resp, info := newClaudeStreamTestContext(t, body)

	usage, err := ClaudeStreamHandler(c, resp, info)

	require.Nil(t, err, "message_delta with output_tokens=0 is a legitimate completion")
	require.NotNil(t, usage)
	require.Equal(t, 1234, usage.PromptTokens)
}

// TestClaudeStreamHandlerErrorEventStillPropagates confirms the pre-existing
// path — an explicit SSE error event — is untouched and still wins over the
// truncation guard with the mapped status code.
func TestClaudeStreamHandlerErrorEventStillPropagates(t *testing.T) {
	setStreamTestMode(t)

	body := sseMessageStart + sseErrorEvent
	c, resp, info := newClaudeStreamTestContext(t, body)

	usage, err := ClaudeStreamHandler(c, resp, info)

	require.Nil(t, usage)
	require.NotNil(t, err)
	require.Equal(t, 529, err.StatusCode, "overloaded_error maps to 529")
	require.Contains(t, err.Error(), "Overloaded")
}

func TestCheckClaudeStreamTruncated(t *testing.T) {
	t.Run("nil claudeInfo is a no-op", func(t *testing.T) {
		require.Nil(t, CheckClaudeStreamTruncated(nil, nil))
	})
	t.Run("Done=true passes", func(t *testing.T) {
		require.Nil(t, CheckClaudeStreamTruncated(nil, &ClaudeResponseInfo{Done: true, Usage: &dto.Usage{}}))
	})
	t.Run("Done=false with nil info still errors", func(t *testing.T) {
		err := CheckClaudeStreamTruncated(nil, &ClaudeResponseInfo{Done: false, Usage: &dto.Usage{}})
		require.NotNil(t, err)
		require.Equal(t, http.StatusBadGateway, err.StatusCode)
		require.Contains(t, err.Error(), "end_reason=)")
	})
	t.Run("Done=false with StreamStatus reports end reason", func(t *testing.T) {
		info := &relaycommon.RelayInfo{StreamStatus: relaycommon.NewStreamStatus()}
		info.StreamStatus.SetEndReason(relaycommon.StreamEndReasonEOF, nil)
		err := CheckClaudeStreamTruncated(info, &ClaudeResponseInfo{Done: false, Usage: &dto.Usage{}})
		require.NotNil(t, err)
		require.Contains(t, err.Error(), "end_reason=eof")
	})
	t.Run("truncation error is skip-retry", func(t *testing.T) {
		// SSE headers and message_start are already on the wire when this
		// fires; a retry on another channel would splice a second stream
		// into the client's response. 502 is inside the default retry range,
		// so the flag is load-bearing.
		err := CheckClaudeStreamTruncated(nil, &ClaudeResponseInfo{Done: false, Usage: &dto.Usage{}})
		require.NotNil(t, err)
		require.True(t, types.IsSkipRetryError(err), "truncation must not trigger channel retry")
	})
}