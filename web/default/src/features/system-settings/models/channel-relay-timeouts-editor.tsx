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
import { Add01Icon, Delete02Icon } from '@hugeicons/core-free-icons'
import { HugeiconsIcon } from '@hugeicons/react'
import { useTranslation } from 'react-i18next'

import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'

import type { ChannelRelayTimeoutEntry } from './channel-relay-timeouts'

type ChannelRelayTimeoutsEditorProps = {
  value: ChannelRelayTimeoutEntry[]
  onChange: (value: ChannelRelayTimeoutEntry[]) => void
}

export function ChannelRelayTimeoutsEditor(
  props: ChannelRelayTimeoutsEditorProps
) {
  const { t } = useTranslation()

  const updateEntry = (
    index: number,
    field: keyof ChannelRelayTimeoutEntry,
    rawValue: string
  ) => {
    const next = props.value.map((entry, entryIndex) =>
      entryIndex === index
        ? { ...entry, [field]: rawValue === '' ? 0 : Number(rawValue) }
        : entry
    )
    props.onChange(next)
  }

  const removeEntry = (index: number) => {
    props.onChange(
      props.value.filter((_entry, entryIndex) => entryIndex !== index)
    )
  }

  return (
    <div className='overflow-hidden rounded-xl border border-border bg-background'>
      <div className='hidden grid-cols-[minmax(0,1fr)_minmax(0,1fr)_2.25rem] gap-3 border-b border-border bg-muted/40 px-3 py-2 text-xs font-medium text-muted-foreground sm:grid'>
        <span>{t('Channel ID')}</span>
        <span>{t('Timeout (seconds)')}</span>
        <span className='sr-only'>{t('Actions')}</span>
      </div>

      {props.value.length === 0 ? (
        <div className='px-4 py-7 text-center text-sm text-muted-foreground'>
          {t('No channel-specific timeouts configured.')}
        </div>
      ) : (
        <div className='divide-y divide-border'>
          {props.value.map((entry, index) => (
            <div
              key={index}
              className='grid gap-3 px-3 py-3 sm:grid-cols-[minmax(0,1fr)_minmax(0,1fr)_2.25rem] sm:items-center'
            >
              <label className='grid gap-1.5'>
                <span className='text-xs font-medium text-muted-foreground sm:hidden'>
                  {t('Channel ID')}
                </span>
                <div className='relative'>
                  <span
                    className='pointer-events-none absolute inset-y-0 left-3 flex items-center font-mono text-sm text-muted-foreground'
                    aria-hidden='true'
                  >
                    #
                  </span>
                  <Input
                    type='number'
                    min={1}
                    step={1}
                    className='pl-7 font-mono tabular-nums'
                    value={entry.channelId || ''}
                    onChange={(event) =>
                      updateEntry(index, 'channelId', event.target.value)
                    }
                  />
                </div>
              </label>

              <label className='grid gap-1.5'>
                <span className='text-xs font-medium text-muted-foreground sm:hidden'>
                  {t('Timeout (seconds)')}
                </span>
                <div className='relative'>
                  <Input
                    type='number'
                    min={1}
                    max={86400}
                    step={1}
                    className='pr-9 font-mono tabular-nums'
                    value={entry.timeoutSeconds || ''}
                    onChange={(event) =>
                      updateEntry(index, 'timeoutSeconds', event.target.value)
                    }
                  />
                  <span
                    className='pointer-events-none absolute inset-y-0 right-3 flex items-center text-xs text-muted-foreground'
                    aria-hidden='true'
                  >
                    s
                  </span>
                </div>
              </label>

              <Button
                type='button'
                variant='ghost'
                size='icon'
                className='justify-self-end text-muted-foreground hover:text-destructive sm:justify-self-auto'
                aria-label={t('Remove channel timeout')}
                onClick={() => removeEntry(index)}
              >
                <HugeiconsIcon icon={Delete02Icon} strokeWidth={1.8} />
              </Button>
            </div>
          ))}
        </div>
      )}

      <div className='flex items-center justify-between gap-3 border-t border-border bg-muted/20 px-3 py-2.5'>
        <span className='text-xs text-muted-foreground'>
          {t('Unlisted channels use the global RELAY_TIMEOUT value.')}
        </span>
        <Button
          type='button'
          variant='outline'
          size='sm'
          onClick={() =>
            props.onChange([
              ...props.value,
              { channelId: 0, timeoutSeconds: 600 },
            ])
          }
        >
          <HugeiconsIcon icon={Add01Icon} strokeWidth={1.8} />
          {t('Add channel')}
        </Button>
      </div>
    </div>
  )
}
