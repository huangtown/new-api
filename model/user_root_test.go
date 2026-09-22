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

// IsRootUser 决定报错掩盖是否透传原始上游报错，是一道权限边界：
// 判错会把计费信息泄露给不该看到的人，或把超管的排障信息掩盖掉。
//
// 中继路径只经过 TokenAuth，它不会往 gin context 写 role（只有会话态的
// UserAuth 会，见 middleware/auth.go），所以身份判定只能按 user id 回查。
func TestIsRootUser(t *testing.T) {
	truncateTables(t)
	require.NoError(t, DB.Exec("DELETE FROM users").Error)

	users := []User{
		{Id: 1, Username: "root-user", Password: "x", Role: common.RoleRootUser, AffCode: "aff-root"},
		{Id: 2, Username: "admin-user", Password: "x", Role: common.RoleAdminUser, AffCode: "aff-admin"},
		{Id: 3, Username: "common-user", Password: "x", Role: common.RoleCommonUser, AffCode: "aff-common"},
		{Id: 4, Username: "guest-user", Password: "x", Role: common.RoleGuestUser, AffCode: "aff-guest"},
	}
	for i := range users {
		require.NoError(t, DB.Create(&users[i]).Error)
	}

	cases := []struct {
		name   string
		userId int
		want   bool
	}{
		{"超级管理员透传原文", 1, true},
		{"普通管理员必须被掩盖", 2, false},
		{"普通用户必须被掩盖", 3, false},
		{"访客必须被掩盖", 4, false},
		{"userId 为 0（未认证）必须被掩盖", 0, false},
		{"不存在的用户必须被掩盖", 999, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, IsRootUser(tc.userId))
		})
	}
}

// IsAdmin 与 IsRootUser 的边界不同，不可互换：管理员在 IsAdmin 下为真，
// 但在报错掩盖语义下必须被掩盖。
func TestIsRootUserIsStricterThanIsAdmin(t *testing.T) {
	truncateTables(t)
	require.NoError(t, DB.Exec("DELETE FROM users").Error)

	admin := User{Id: 10, Username: "admin-only", Password: "x", Role: common.RoleAdminUser}
	require.NoError(t, DB.Create(&admin).Error)

	require.True(t, IsAdmin(admin.Id), "前提：该用户是管理员")
	require.False(t, IsRootUser(admin.Id),
		"管理员不是超级管理员——掩盖若用 IsAdmin 判定会向管理员泄露计费原文")
}
