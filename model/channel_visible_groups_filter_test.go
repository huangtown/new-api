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
	"gorm.io/gorm"
)

// The visible-groups predicate must use the per-dialect quoting, not a
// hardcoded backtick, or PostgreSQL rejects the identifier.
func TestVisibleGroupsFilterUsesPortableQuoting(t *testing.T) {
	saved := commonGroupCol
	t.Cleanup(func() { commonGroupCol = saved })

	for _, tc := range []struct{ col, want string }{
		{`"group"`, `"group" IN`},
		{"`group`", "`group` IN"},
	} {
		commonGroupCol = tc.col
		sql := DB.ToSQL(func(tx *gorm.DB) *gorm.DB {
			return ApplyChannelVisibleGroupsFilter(tx.Model(&Channel{}), []string{"vip"}).Find(&[]Channel{})
		})
		require.Contains(t, sql, tc.want, "quoting must follow commonGroupCol")
	}

	// An empty list must not add a predicate at all.
	commonGroupCol = saved
	sql := DB.ToSQL(func(tx *gorm.DB) *gorm.DB {
		return ApplyChannelVisibleGroupsFilter(tx.Model(&Channel{}), nil).Find(&[]Channel{})
	})
	require.NotContains(t, sql, " IN ")
}

// A whitelist we cannot parse must fail closed. CanViewGroup reads an empty
// slice as "no restriction", so returning one on corrupt data would hand the
// full channel list to exactly the admin the whitelist existed to constrain.
func TestGetVisibleGroupsFailsClosedOnMalformedJSON(t *testing.T) {
	for _, stored := range []string{"not json", `{"alpha":true}`, `["alpha"`} {
		raw := stored
		user := &User{Id: 1, Role: common.RoleAdminUser, VisibleGroups: &raw}
		groups := user.GetVisibleGroups()
		require.NotEmpty(t, groups, "corrupt value must not read as unrestricted")
		require.False(t, user.CanViewGroup("alpha"), "must not grant access on %q", stored)
		require.False(t, user.CanViewGroup("default"), "must not grant access on %q", stored)
	}
}

// The legitimate unset case must stay permissive for backward compatibility.
func TestGetVisibleGroupsUnsetStaysUnrestricted(t *testing.T) {
	empty := ""
	for _, user := range []*User{
		{Id: 1, Role: common.RoleAdminUser},
		{Id: 1, Role: common.RoleAdminUser, VisibleGroups: &empty},
	} {
		require.Empty(t, user.GetVisibleGroups())
		require.True(t, user.CanViewGroup("anything"))
	}
}
