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
import { expect, test } from 'vitest'

import {
  USER_FORM_DEFAULT_VALUES,
  transformFormDataToPayload,
  transformUserToFormDefaults,
} from '../user-form'
import type { User } from '../../types'

const baseUser = {
  id: 7,
  username: 'scoped-admin',
  display_name: 'Scoped Admin',
  role: 10,
  status: 1,
  quota: 0,
  used_quota: 0,
  request_count: 0,
  group: 'default',
} as unknown as User

// The whitelist is stored as a JSON array string. The backend reads an absent
// or unparseable value as "no restriction", so the form must surface that as
// an empty selection rather than throwing or showing a stale value.
test.each([
  ['["alpha","beta"]', ['alpha', 'beta']],
  ['[]', []],
  ['', []],
  [null, []],
  ['not json', []],
  ['{"alpha":true}', []],
  ['["alpha",2,null]', ['alpha']],
])('parses stored visible_groups %j into %j', (stored, expected) => {
  const defaults = transformUserToFormDefaults({
    ...baseUser,
    visible_groups: stored,
  } as User)
  expect(defaults.visible_groups).toEqual(expected)
})

// Only root may edit the whitelist. When the caller is not allowed to, the
// field must be omitted entirely: sending "[]" would clear a configured
// restriction that the current admin was never shown.
test('omits visible_groups when the caller may not edit it', () => {
  const payload = transformFormDataToPayload(
    { ...USER_FORM_DEFAULT_VALUES, visible_groups: ['alpha'] },
    7,
    undefined,
    false
  )
  expect(payload).not.toHaveProperty('visible_groups')
})

test('serializes visible_groups as a JSON array when editable', () => {
  const payload = transformFormDataToPayload(
    { ...USER_FORM_DEFAULT_VALUES, visible_groups: ['alpha', 'beta'] },
    7,
    undefined,
    true
  )
  expect(payload.visible_groups).toBe('["alpha","beta"]')
})

// An empty selection is a real instruction: it clears the restriction. It must
// serialize to "[]" rather than being dropped as falsy.
test('serializes an empty selection as an explicit empty array', () => {
  const payload = transformFormDataToPayload(
    { ...USER_FORM_DEFAULT_VALUES, visible_groups: [] },
    7,
    undefined,
    true
  )
  expect(payload.visible_groups).toBe('[]')
})

// Create goes through a different branch that never sends update-only fields.
test('never sends visible_groups when creating a user', () => {
  const payload = transformFormDataToPayload(
    { ...USER_FORM_DEFAULT_VALUES, visible_groups: ['alpha'] },
    undefined,
    undefined,
    true
  )
  expect(payload).not.toHaveProperty('visible_groups')
})
