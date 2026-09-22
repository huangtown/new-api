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
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/relaykit/types"

	"github.com/stretchr/testify/require"
)

// An in-stream authentication_error maps to 401, which is exactly the default
// AutomaticDisableStatusCodeRanges entry. The upstream already answered 200 and
// then reported a semantic error inside the SSE body, so that status is a
// client-facing hint rather than proof the channel key is dead — a transient
// blip must not pull the channel out of rotation.
func TestInStreamAuthErrorDoesNotAutoDisableChannel(t *testing.T) {
	saved := common.AutomaticDisableChannelEnabled
	common.AutomaticDisableChannelEnabled = true
	t.Cleanup(func() { common.AutomaticDisableChannelEnabled = saved })

	inStream := types.WithOpenAIError(
		types.OpenAIError{Type: "authentication_error", Message: "token refresh blip"},
		http.StatusUnauthorized,
		types.ErrOptionWithSkipAutoDisable(),
	)
	require.True(t, types.IsSkipAutoDisableError(inStream))
	require.False(t, ShouldDisableChannel(inStream),
		"an in-stream 401 must not auto-disable the channel")
}

// A genuine 401 on the HTTP response itself still means the credentials were
// rejected, so auto-disable must continue to fire there.
func TestResponseLevelAuthErrorStillAutoDisablesChannel(t *testing.T) {
	saved := common.AutomaticDisableChannelEnabled
	common.AutomaticDisableChannelEnabled = true
	t.Cleanup(func() { common.AutomaticDisableChannelEnabled = saved })

	rejected := types.WithOpenAIError(
		types.OpenAIError{Type: "authentication_error", Message: "invalid api key"},
		http.StatusUnauthorized,
	)
	require.False(t, types.IsSkipAutoDisableError(rejected))
	require.True(t, ShouldDisableChannel(rejected),
		"a rejected request must still disable the channel")
}
