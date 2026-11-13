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
import { useFormContext } from 'react-hook-form'
import { useTranslation } from 'react-i18next'

import {
  FormControl,
  FormDescription,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from '@/components/ui/form'
import { Input } from '@/components/ui/input'
import { Switch } from '@/components/ui/switch'

import { SettingsCard } from '../components/settings-card'
import {
  SettingsSwitchContent,
  SettingsSwitchItem,
} from '../components/settings-form-layout'
import { safeNumberFieldProps } from '../utils/numeric-field'
import { ChannelRelayTimeoutsEditor } from './channel-relay-timeouts-editor'
import type { RoutingPolicyFormValues } from './routing-form'

export function RetrySection() {
  const { t } = useTranslation()
  const form = useFormContext<RoutingPolicyFormValues>()
  return (
    <SettingsCard title={t('Retry budget')} className='shadow-none'>
      <div className='space-y-4'>
        <FormField
          control={form.control}
          name='RetryTimes'
          render={({ field }) => (
            <FormItem>
              <FormLabel>{t('Maximum retries')}</FormLabel>
              <FormControl>
                <Input
                  type='number'
                  min={0}
                  max={99}
                  step={1}
                  {...safeNumberFieldProps(field)}
                />
              </FormControl>
              <FormDescription>
                {t('Excludes the first attempt. Counted per group.')}
              </FormDescription>
              <FormMessage />
            </FormItem>
          )}
        />
        <FormField
          control={form.control}
          name='AutomaticRetryStatusCodes'
          render={({ field }) => (
            <FormItem>
              <FormLabel>{t('Auto-retry status codes')}</FormLabel>
              <FormControl>
                <Input
                  {...field}
                  placeholder={t('e.g. 401, 403, 429, 500-599')}
                />
              </FormControl>
              <FormDescription>
                {t('2xx, 504 and 524 are always excluded.')}
              </FormDescription>
              <FormMessage />
            </FormItem>
          )}
        />
        <FormField
          control={form.control}
          name='ChannelRelayTimeoutEnabled'
          render={({ field }) => (
            <SettingsSwitchItem>
              <SettingsSwitchContent>
                <FormLabel>{t('Enable per-channel relay timeout')}</FormLabel>
                <FormDescription>
                  {t(
                    'Applies the per-channel relay timeout below to non-streaming requests. When off, those requests use the global RELAY_TIMEOUT instead.'
                  )}
                </FormDescription>
              </SettingsSwitchContent>
              <FormControl>
                <Switch
                  checked={field.value}
                  onCheckedChange={field.onChange}
                />
              </FormControl>
            </SettingsSwitchItem>
          )}
        />
        <FormField
          control={form.control}
          name='ChannelRelayTimeouts'
          render={({ field }) => (
            <FormItem>
              <FormLabel>{t('Per-channel relay timeout')}</FormLabel>
              <FormControl>
                <ChannelRelayTimeoutsEditor
                  value={field.value}
                  onChange={field.onChange}
                  disabled={!form.watch('ChannelRelayTimeoutEnabled')}
                />
              </FormControl>
              <FormDescription>
                {t(
                  'Overrides the global relay timeout for the listed channels. Leave empty to use the global value. Ignored for streaming requests, which use the first token timeout instead.'
                )}
              </FormDescription>
              <FormMessage />
            </FormItem>
          )}
        />
        <FormField
          control={form.control}
          name='ChannelFirstTokenTimeoutEnabled'
          render={({ field }) => (
            <SettingsSwitchItem>
              <SettingsSwitchContent>
                <FormLabel>
                  {t('Enable per-channel first token timeout')}
                </FormLabel>
                <FormDescription>
                  {t(
                    'Applies the first token timeout below to streaming requests. When off, streaming requests get no first token deadline.'
                  )}
                </FormDescription>
              </SettingsSwitchContent>
              <FormControl>
                <Switch
                  checked={field.value}
                  onCheckedChange={field.onChange}
                />
              </FormControl>
            </SettingsSwitchItem>
          )}
        />
        <FormField
          control={form.control}
          name='ChannelFirstTokenTimeouts'
          render={({ field }) => (
            <FormItem>
              <FormLabel>{t('Per-channel first token timeout')}</FormLabel>
              <FormControl>
                <ChannelRelayTimeoutsEditor
                  value={field.value}
                  onChange={field.onChange}
                  disabled={!form.watch('ChannelFirstTokenTimeoutEnabled')}
                />
              </FormControl>
              <FormDescription>
                {t(
                  'Streaming only: how long to wait for the first data chunk before giving up. Idle time after the first chunk is bounded separately by STREAMING_TIMEOUT.'
                )}
              </FormDescription>
              <FormMessage />
            </FormItem>
          )}
        />
        <FormField
          control={form.control}
          name='ChannelResponseHeaderTimeoutEnabled'
          render={({ field }) => (
            <SettingsSwitchItem>
              <SettingsSwitchContent>
                <FormLabel>
                  {t('Enable per-channel response timeout')}
                </FormLabel>
                <FormDescription>
                  {t(
                    'Applies the response timeout below to the wait for upstream response headers. When off, the global RELAY_RESPONSE_HEADER_TIMEOUT applies instead.'
                  )}
                </FormDescription>
              </SettingsSwitchContent>
              <FormControl>
                <Switch
                  checked={field.value}
                  onCheckedChange={field.onChange}
                />
              </FormControl>
            </SettingsSwitchItem>
          )}
        />
        <FormField
          control={form.control}
          name='ChannelResponseHeaderTimeouts'
          render={({ field }) => (
            <FormItem>
              <FormLabel>{t('Per-channel response timeout')}</FormLabel>
              <FormControl>
                <ChannelRelayTimeoutsEditor
                  value={field.value}
                  onChange={field.onChange}
                  disabled={!form.watch('ChannelResponseHeaderTimeoutEnabled')}
                />
              </FormControl>
              <FormDescription>
                {t(
                  'How long to wait for upstream response headers before giving up. This covers slow upstreams that accept the connection and return nothing — the phase neither the total timeout nor the first token timeout bounds.'
                )}
              </FormDescription>
              <FormMessage />
            </FormItem>
          )}
        />
      </div>
    </SettingsCard>
  )
}
