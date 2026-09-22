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
import { zodResolver } from '@hookform/resolvers/zod'
import { useForm } from 'react-hook-form'
import { useTranslation } from 'react-i18next'
import * as z from 'zod'

import {
  Form,
  FormControl,
  FormDescription,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from '@/components/ui/form'
import { Input } from '@/components/ui/input'
import { Switch } from '@/components/ui/switch'
import { Textarea } from '@/components/ui/textarea'

import {
  SettingsForm,
  SettingsSwitchContent,
  SettingsSwitchItem,
} from '../components/settings-form-layout'
import { SettingsPageFormActions } from '../components/settings-page-context'
import { SettingsSection } from '../components/settings-section'
import { useResetForm } from '../hooks/use-reset-form'
import { useUpdateOption } from '../hooks/use-update-option'

const fallbackSchema = z.object({
  FallbackEnabled: z.boolean(),
  FallbackChannelIDs: z.string(),
  FallbackStatusCodes: z.string(),
  FallbackTriggerKeywords: z.string(),
  GroupFallbackChannelIDs: z.string(),
  GroupFallbackBillingRates: z.string().refine((value) => {
    if (!value.trim()) return true
    try {
      const parsed = JSON.parse(value)
      if (!parsed || typeof parsed !== 'object' || Array.isArray(parsed)) return false
      return Object.values(parsed).every((items) => Array.isArray(items) && items.every((item: any) =>
        item && typeof item === 'object' && /^\d+$/.test(String(item.channel)) &&
        typeof item.rate === 'number' && Number.isFinite(item.rate) && item.rate > 0
      ))
    } catch { return false }
  }, 'Invalid JSON: expected group arrays with positive numeric channel rates'),
})

type FallbackFormValues = z.infer<typeof fallbackSchema>

type FallbackSettingsSectionProps = {
  defaultValues: FallbackFormValues
}

export function FallbackSettingsSection({
  defaultValues,
}: FallbackSettingsSectionProps) {
  const { t } = useTranslation()
  const updateOption = useUpdateOption()

  const form = useForm({
    resolver: zodResolver(fallbackSchema),
    defaultValues,
  })

  useResetForm(form, defaultValues)

  const onSubmit = async (data: FallbackFormValues) => {
    const updates = Object.entries(data).filter(
      ([key, value]) => value !== defaultValues[key as keyof FallbackFormValues]
    )

    for (const [key, value] of updates) {
      await updateOption.mutateAsync({ key, value })
    }
  }

  return (
    <SettingsSection title={t('Channel Fallback')}>
      <Form {...form}>
        <SettingsForm onSubmit={form.handleSubmit(onSubmit)} autoComplete='off'>
          <SettingsPageFormActions
            onSave={form.handleSubmit(onSubmit)}
            onReset={() => form.reset(defaultValues)}
            isSaving={updateOption.isPending}
            isResetDisabled={updateOption.isPending}
            saveLabel='Save fallback settings'
          />
          <SettingsSwitchItem>
              <SettingsSwitchContent>
                <FormField
                  control={form.control}
                  name="FallbackEnabled"
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>{t('Enable Fallback')}</FormLabel>
                      <FormDescription>
                        {t('When a primary channel returns a matching error, automatically switch to a fallback channel.')}
                      </FormDescription>
                      <FormControl>
                        <Switch
                          checked={field.value}
                          onCheckedChange={field.onChange}
                        />
                      </FormControl>
                    </FormItem>
                  )}
                />
              </SettingsSwitchContent>
            </SettingsSwitchItem>

            <FormField
              control={form.control}
              name="FallbackChannelIDs"
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Global Fallback Channel IDs')}</FormLabel>
                  <FormDescription>
                    {t('Comma-separated channel IDs to use as fallback for all groups (e.g. "10,11").')}
                  </FormDescription>
                  <FormControl>
                    <Input
                      placeholder="10,11"
                      {...field}
                    />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />

            <FormField
              control={form.control}
              name="FallbackStatusCodes"
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Trigger Status Codes')}</FormLabel>
                  <FormDescription>
                    {t('HTTP status codes that trigger fallback, comma-separated (e.g. "400,429").')}
                  </FormDescription>
                  <FormControl>
                    <Input
                      placeholder="400"
                      {...field}
                    />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />

            <FormField
              control={form.control}
              name="FallbackTriggerKeywords"
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Trigger Keywords')}</FormLabel>
                  <FormDescription>
                    {t('Error message keywords that trigger fallback, comma-separated (e.g. "too long,context length").')}
                  </FormDescription>
                  <FormControl>
                    <Input
                      placeholder="too long,context length,maximum context"
                      {...field}
                    />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />

            <FormField
              control={form.control}
              name="GroupFallbackChannelIDs"
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Per-Group Fallback Channel IDs')}</FormLabel>
                  <FormDescription>
                    {t('JSON mapping of group names to comma-separated fallback channel IDs. Overrides global setting for matching groups. Example: {"default":"10,11","vip":"20"}')}
                  </FormDescription>
                  <FormControl>
                    <Textarea
                      placeholder='{"default":"10,11","vip":"20,21"}'
                      className="font-mono text-sm"
                      rows={3}
                      {...field}
                    />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />
            <FormField control={form.control} name="GroupFallbackBillingRates" render={({ field }) => (
              <FormItem><FormLabel>{t('Fallback Billing Rates')}</FormLabel>
                <FormDescription>{t('JSON absolute standard-price multipliers, e.g. {"default":[{"channel":"10","rate":1.5}]}')}</FormDescription>
                <FormControl><Textarea className="font-mono text-sm" rows={3} placeholder='{"default":[{"channel":"10","rate":1.5}]}' {...field} /></FormControl><FormMessage />
              </FormItem>
            )} />
        </SettingsForm>
      </Form>
    </SettingsSection>
  )
}
