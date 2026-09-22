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
package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"

	"github.com/stretchr/testify/require"
)

// EditWithTx writes an explicit column whitelist rather than the whole struct.
// visible_groups was missing from it, so the admin UI could submit a group
// whitelist and the update would silently succeed while persisting nothing —
// the admin kept seeing every channel group.
func TestEditWithTxPersistsVisibleGroups(t *testing.T) {
	truncateTables(t)
	require.NoError(t, DB.Exec("DELETE FROM users").Error)

	user := User{
		Id:       1,
		Username: "scoped-admin",
		Password: "x",
		Role:     common.RoleAdminUser,
		AffCode:  "aff-scoped",
	}
	require.NoError(t, DB.Create(&user).Error)

	groups := `["alpha","beta"]`
	update := User{Id: user.Id, Username: user.Username, VisibleGroups: &groups}
	require.NoError(t, update.EditWithTx(DB, false))

	stored, err := GetUserById(user.Id, false)
	require.NoError(t, err)
	require.NotNil(t, stored.VisibleGroups)
	require.Equal(t, groups, *stored.VisibleGroups)
	require.Equal(t, []string{"alpha", "beta"}, stored.GetVisibleGroups())
}

// A nil pointer means the caller did not submit the field at all. Treating that
// as an empty value would silently clear a configured whitelist whenever any
// other field (say, the remark) was edited.
func TestEditWithTxLeavesVisibleGroupsAloneWhenAbsent(t *testing.T) {
	truncateTables(t)
	require.NoError(t, DB.Exec("DELETE FROM users").Error)

	groups := `["alpha"]`
	user := User{
		Id:            1,
		Username:      "scoped-admin",
		Password:      "x",
		Role:          common.RoleAdminUser,
		AffCode:       "aff-scoped",
		VisibleGroups: &groups,
	}
	require.NoError(t, DB.Create(&user).Error)

	update := User{Id: user.Id, Username: user.Username, Remark: "edited"}
	require.NoError(t, update.EditWithTx(DB, false))

	stored, err := GetUserById(user.Id, false)
	require.NoError(t, err)
	require.NotNil(t, stored.VisibleGroups)
	require.Equal(t, groups, *stored.VisibleGroups)
}

// An empty string is a real submitted value: it clears the restriction, so the
// admin goes back to seeing every group.
func TestEditWithTxClearsVisibleGroupsOnEmptyString(t *testing.T) {
	truncateTables(t)
	require.NoError(t, DB.Exec("DELETE FROM users").Error)

	groups := `["alpha"]`
	user := User{
		Id:            1,
		Username:      "scoped-admin",
		Password:      "x",
		Role:          common.RoleAdminUser,
		AffCode:       "aff-scoped",
		VisibleGroups: &groups,
	}
	require.NoError(t, DB.Create(&user).Error)

	cleared := ""
	update := User{Id: user.Id, Username: user.Username, VisibleGroups: &cleared}
	require.NoError(t, update.EditWithTx(DB, false))

	stored, err := GetUserById(user.Id, false)
	require.NoError(t, err)
	require.Empty(t, stored.GetVisibleGroups())
	require.True(t, stored.CanViewGroup("anything"))
}
