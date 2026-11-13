package channel

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func newHeaderTimeoutTestContext(t *testing.T) *gin.Context {
	t.Helper()
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	c.Request = req
	return c
}

// The response header budget must only bound the wait for headers. Once they
// arrive the body has to stay readable: cancelling the request context on
// success aborts the body, which silently turns every response into an empty
// one.
func TestResponseHeaderTimeoutDoesNotAbortBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		w.(http.Flusher).Flush()
		time.Sleep(400 * time.Millisecond)
		_, _ = io.WriteString(w, "data: hello\n\n")
	}))
	defer srv.Close()

	c := newHeaderTimeoutTestContext(t)
	info := &common.RelayInfo{
		ChannelMeta: &common.ChannelMeta{ChannelId: 910001},
	}
	require.NoError(t, operation_setting.UpdateChannelResponseHeaderTimeoutsByJSONString(`{"910001":5}`))
	t.Cleanup(func() {
		require.NoError(t, operation_setting.UpdateChannelResponseHeaderTimeoutsByJSONString(`{}`))
	})

	req, err := http.NewRequestWithContext(c.Request.Context(), http.MethodPost, srv.URL, strings.NewReader(`{}`))
	require.NoError(t, err)

	resp, err := doRequest(c, req, info)
	require.NoError(t, err)
	defer resp.Body.Close()

	// Well past the header budget: if the timer were left armed, or the context
	// were cancelled on the success path, this read would fail or come back empty.
	body, readErr := io.ReadAll(resp.Body)
	require.NoError(t, readErr, "body read must not be aborted after headers arrived")
	require.Equal(t, "data: hello\n\n", string(body))
}

// A slow upstream that never returns headers must be cut off at the per-channel
// budget instead of waiting out the global default.
func TestResponseHeaderTimeoutFiresOnSlowUpstream(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(3 * time.Second)
	}))
	defer srv.Close()

	c := newHeaderTimeoutTestContext(t)
	info := &common.RelayInfo{
		ChannelMeta: &common.ChannelMeta{ChannelId: 910002},
	}
	require.NoError(t, operation_setting.UpdateChannelResponseHeaderTimeoutsByJSONString(`{"910002":1}`))
	t.Cleanup(func() {
		require.NoError(t, operation_setting.UpdateChannelResponseHeaderTimeoutsByJSONString(`{}`))
	})

	req, err := http.NewRequestWithContext(c.Request.Context(), http.MethodPost, srv.URL, strings.NewReader(`{}`))
	require.NoError(t, err)

	start := time.Now()
	_, err = doRequest(c, req, info)
	elapsed := time.Since(start)

	require.Error(t, err, "slow upstream must be cut off")
	require.Less(t, elapsed, 2500*time.Millisecond,
		"the 1s per-channel budget must fire well before the upstream's 3s delay")
}

// Disabling the switch must remove the per-request bound.
func TestResponseHeaderTimeoutSwitchOff(t *testing.T) {
	require.NoError(t, operation_setting.UpdateChannelResponseHeaderTimeoutsByJSONString(`{"910003":1}`))
	operation_setting.SetChannelResponseHeaderTimeoutEnabled(false)
	t.Cleanup(func() {
		operation_setting.SetChannelResponseHeaderTimeoutEnabled(true)
		require.NoError(t, operation_setting.UpdateChannelResponseHeaderTimeoutsByJSONString(`{}`))
	})

	_, ok := operation_setting.GetChannelResponseHeaderTimeout(910003)
	require.False(t, ok, "a disabled switch must not expose a per-channel budget")
}
