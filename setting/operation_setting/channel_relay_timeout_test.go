package operation_setting

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUpdateChannelRelayTimeoutsByJSONString(t *testing.T) {
	original := ChannelRelayTimeouts2JSONString()
	t.Cleanup(func() {
		require.NoError(t, UpdateChannelRelayTimeoutsByJSONString(original))
	})

	require.NoError(t, UpdateChannelRelayTimeoutsByJSONString(`{"59":600,"42":120}`))

	timeoutSeconds, ok := GetChannelRelayTimeout(59)
	require.True(t, ok)
	assert.Equal(t, 600, timeoutSeconds)

	timeoutSeconds, ok = GetChannelRelayTimeout(42)
	require.True(t, ok)
	assert.Equal(t, 120, timeoutSeconds)

	_, ok = GetChannelRelayTimeout(7)
	assert.False(t, ok)
}

func TestUpdateChannelRelayTimeoutsRejectsInvalidValuesWithoutReplacingCurrentConfig(t *testing.T) {
	original := ChannelRelayTimeouts2JSONString()
	t.Cleanup(func() {
		require.NoError(t, UpdateChannelRelayTimeoutsByJSONString(original))
	})

	require.NoError(t, UpdateChannelRelayTimeoutsByJSONString(`{"59":600}`))

	invalidValues := []string{
		`[]`,
		`{"channel":600}`,
		`{"0":600}`,
		`{"59":0}`,
		`{"59":86401}`,
		`{"59":"600"}`,
		`{"59":600," 59":300}`,
	}
	for _, value := range invalidValues {
		assert.Error(t, UpdateChannelRelayTimeoutsByJSONString(value), value)
	}

	timeoutSeconds, ok := GetChannelRelayTimeout(59)
	require.True(t, ok)
	assert.Equal(t, 600, timeoutSeconds)
}

func TestUpdateChannelRelayTimeoutsCanClearConfig(t *testing.T) {
	original := ChannelRelayTimeouts2JSONString()
	t.Cleanup(func() {
		require.NoError(t, UpdateChannelRelayTimeoutsByJSONString(original))
	})

	require.NoError(t, UpdateChannelRelayTimeoutsByJSONString(`{"59":600}`))
	require.NoError(t, UpdateChannelRelayTimeoutsByJSONString(`{}`))

	_, ok := GetChannelRelayTimeout(59)
	assert.False(t, ok)
}
