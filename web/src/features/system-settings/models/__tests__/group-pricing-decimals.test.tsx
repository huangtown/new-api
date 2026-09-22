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
import { fireEvent, render, screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { useState } from 'react'
import { assert, expect, test } from 'vitest'

import { GroupRatioVisualEditor } from '../group-ratio-visual-editor'

function PricingFixture() {
  const [settings, setSettings] = useState<Record<string, string>>({
    GroupRatio: '{"default":1}',
    TopupGroupRatio: '{}',
    GroupCacheReadAmplificationRatio: '{}',
    UserUsableGroups: '{}',
  })
  return (
    <>
      <GroupRatioVisualEditor
        section='pricing'
        onSectionChange={() => {}}
        defaultUseAutoGroupField={null}
        groupRatio={settings.GroupRatio}
        topupGroupRatio={settings.TopupGroupRatio}
        groupCacheReadAmplificationRatio={
          settings.GroupCacheReadAmplificationRatio
        }
        userUsableGroups={settings.UserUsableGroups}
        groupGroupRatio='{}'
        autoGroups='[]'
        maxTokenAutoGroupsField={null}
        groupSpecialUsableGroup='{}'
        onChange={(field, value) =>
          setSettings((current) => ({ ...current, [field]: value }))
        }
      />
      <output aria-label='Saved ratios'>{JSON.stringify(settings)}</output>
    </>
  )
}

test.each([
  ['GroupRatio', 0],
  ['TopupGroupRatio', 1],
] as const)(
  '%s preserves typed decimals and accepts them as valid numeric ratios',
  async (key, index) => {
    const user = userEvent.setup()
    render(<PricingFixture />)
    const row = screen.getByDisplayValue('default').closest('tr')
    assert(row)
    const input = within(row).getAllByRole('spinbutton')[
      index
    ] as HTMLInputElement
    fireEvent.change(input, { target: { value: '0.0' } })
    expect(input.value).toBe('0.0')
    await user.clear(input)
    await user.type(input, '0.04')
    expect(input).toHaveValue(0.04)
    await user.tab()
    expect(input.checkValidity()).toBe(true)
    const saved = JSON.parse(
      screen.getByRole('status', { name: 'Saved ratios' }).textContent ?? '{}'
    )
    expect(JSON.parse(saved[key])).toEqual({ default: 0.04 })
    await user.clear(input)
    await user.type(input, '0.0001')
    expect(input).toHaveValue(0.0001)
    expect(input.checkValidity()).toBe(true)
    await user.clear(input)
    await user.type(input, '0.00001')
    expect(input.validity.stepMismatch).toBe(true)
    await user.clear(input)
    await user.type(input, '-0.04')
    expect(input.validity.rangeUnderflow).toBe(true)
  }
)

// The cache read multiplier is optional per group: the resolver only honours
// a stored ratio when it is > 0, so a blank cell must be omitted from the
// payload rather than written as 0, which would persist a value that behaves
// like "unset" but reads as a real configuration.
test('cache read multiplier round-trips and omits blank cells', async () => {
  const user = userEvent.setup()
  render(<PricingFixture />)
  const row = screen.getByDisplayValue('default').closest('tr')
  assert(row)
  const input = within(row).getAllByRole('spinbutton')[2] as HTMLInputElement

  await user.type(input, '2.5')
  await user.tab()
  const saved = JSON.parse(
    screen.getByRole('status', { name: 'Saved ratios' }).textContent ?? '{}'
  )
  expect(JSON.parse(saved.GroupCacheReadAmplificationRatio)).toEqual({
    default: 2.5,
  })

  await user.clear(input)
  const cleared = JSON.parse(
    screen.getByRole('status', { name: 'Saved ratios' }).textContent ?? '{}'
  )
  expect(JSON.parse(cleared.GroupCacheReadAmplificationRatio)).toEqual({})
})
