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
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { X } from 'lucide-react'

import {
  SettingsForm,
  SettingsSwitchContent,
  SettingsSwitchItem,
} from '../components/settings-form-layout'
import { SettingsPageFormActions } from '../components/settings-page-context'
import { SettingsSection } from '../components/settings-section'
import { useResetForm } from '../hooks/use-reset-form'
import { useUpdateOption } from '../hooks/use-update-option'

const billingErrorMaskSchema = z.object({
  enabled: z.boolean(),
  keywords: z.array(z.string()),
  status_code: z.number().int().min(100).max(599),
})

type BillingErrorMaskValues = z.infer<typeof billingErrorMaskSchema>

export type BillingErrorMaskConfig = BillingErrorMaskValues

export const DEFAULT_BILLING_ERROR_MASK_CONFIG: BillingErrorMaskConfig = {
  enabled: false,
  keywords: [],
  status_code: 524,
}

export function parseBillingErrorMaskConfig(raw: {
  enabled: boolean
  keywords: string
  statusCode: string
}): BillingErrorMaskConfig {
  const n = Number(raw.statusCode)
  return {
    enabled: Boolean(raw.enabled),
    keywords: raw.keywords
      ? raw.keywords.split(',').map((k) => k.trim()).filter(Boolean)
      : [],
    status_code:
      Number.isFinite(n) && n >= 100 && n <= 599 ? n : 200,
    message: '',
  }
}

type BillingErrorMaskSectionProps = {
  defaultValues: BillingErrorMaskConfig
}

function KeywordsInput({
  value,
  onChange,
  disabled,
}: {
  value: string[]
  onChange: (v: string[]) => void
  disabled?: boolean
}) {
  const { t } = useTranslation()
  const handleKeyDown = (e: React.KeyboardEvent<HTMLInputElement>) => {
    if (e.key === 'Enter') {
      e.preventDefault()
      const inputEl = e.currentTarget
      const keyword = inputEl.value.trim()
      if (keyword && !value.includes(keyword)) {
        onChange([...value, keyword])
        inputEl.value = ''
      }
    }
  }
  const remove = (kw: string) => onChange(value.filter((k) => k !== kw))

  return (
    <div className='space-y-2'>
      <div className='border-input bg-background flex min-h-10 flex-wrap gap-1.5 rounded-md border px-3 py-2 text-sm'>
        {value.map((kw) => (
          <Badge key={kw} variant='secondary' className='gap-1'>
            {kw}
            {!disabled && (
              <button
                type='button'
                onClick={() => remove(kw)}
                className='hover:text-foreground text-muted-foreground ml-0.5'
                aria-label={t('Remove keyword {{kw}}', { kw })}
              >
                <X className='h-3 w-3' />
              </button>
            )}
          </Badge>
        ))}
        <input
          type='text'
          disabled={disabled}
          className='bg-transparent flex-1 outline-none placeholder:text-muted-foreground min-w-[120px]'
          placeholder={t('Type a keyword and press Enter')}
          onKeyDown={handleKeyDown}
        />
      </div>
    </div>
  )
}

export function BillingErrorMaskSection({ defaultValues }: BillingErrorMaskSectionProps) {
  const { t } = useTranslation()
  const updateOption = useUpdateOption()

  const form = useForm<BillingErrorMaskValues>({
    resolver: zodResolver(billingErrorMaskSchema),
    defaultValues,
  })

  useResetForm(form, defaultValues)

  const onSubmit = async (data: BillingErrorMaskValues) => {
    await updateOption.mutateAsync({
      key: 'BillingErrorMaskingEnabled',
      value: data.enabled,
    })
    await updateOption.mutateAsync({
      key: 'BillingErrorMaskingKeywords',
      value: data.keywords.join(','),
    })
    await updateOption.mutateAsync({
      key: 'BillingErrorMaskingStatusCode',
      value: String(data.status_code),
    })
  }

  return (
    <SettingsSection title={t('Billing Error Mask')}>
      <Form {...form}>
        <SettingsForm onSubmit={form.handleSubmit(onSubmit)} autoComplete='off'>
          <SettingsPageFormActions
            onSave={form.handleSubmit(onSubmit)}
            onReset={() => form.reset(defaultValues)}
            isSaving={updateOption.isPending}
            isResetDisabled={updateOption.isPending}
            saveLabel='Save billing error mask settings'
          />

          <SettingsSwitchItem>
            <SettingsSwitchContent>
              <FormField
                control={form.control}
                name='enabled'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Enable Billing Error Mask')}</FormLabel>
                    <FormDescription>
                      {t('When enabled, error messages containing matched keywords are hidden from non-admin users.')}
                    </FormDescription>
                    <FormControl>
                      <Switch checked={field.value} onCheckedChange={field.onChange} />
                    </FormControl>
                  </FormItem>
                )}
              />
            </SettingsSwitchContent>
          </SettingsSwitchItem>

          <FormField
            control={form.control}
            name='keywords'
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('Sensitive Keywords')}</FormLabel>
                <FormDescription>
                  {t('Errors containing any of these keywords will trigger the mask rule.')}
                </FormDescription>
                <FormControl>
                  <KeywordsInput
                    value={field.value}
                    onChange={field.onChange}
                    disabled={updateOption.isPending}
                  />
                </FormControl>
                <FormMessage />
              </FormItem>
            )}
          />

          <FormField
            control={form.control}
            name='status_code'
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('Masked Status Code')}</FormLabel>
                <FormDescription>
                  {t('HTTP status code returned to the caller when the mask fires (100–599).')}
                </FormDescription>
                <FormControl>
                  <Input
                    type='number'
                    min={100}
                    max={599}
                    className='w-32'
                    {...field}
                    onChange={(e) => field.onChange(Number(e.target.value))}
                  />
                </FormControl>
                <FormMessage />
              </FormItem>
            )}
          />
        </SettingsForm>
      </Form>
    </SettingsSection>
  )
}
