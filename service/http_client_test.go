package service

import (
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWithHttpClientTimeoutReusesTransportWithoutMutatingBaseClient(t *testing.T) {
	transport := &http.Transport{}
	baseClient := &http.Client{
		Transport: transport,
		Timeout:   30 * time.Second,
	}

	client := WithHttpClientTimeout(baseClient, 10*time.Minute)

	require.NotSame(t, baseClient, client)
	assert.Same(t, transport, client.Transport)
	assert.Equal(t, 10*time.Minute, client.Timeout)
	assert.Equal(t, 30*time.Second, baseClient.Timeout)
}

func TestWithHttpClientTimeoutLeavesClientUnchangedForInvalidTimeout(t *testing.T) {
	baseClient := &http.Client{Timeout: 30 * time.Second}

	assert.Same(t, baseClient, WithHttpClientTimeout(baseClient, 0))
	assert.Nil(t, WithHttpClientTimeout(nil, time.Minute))
}
