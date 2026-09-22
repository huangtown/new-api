import type { TFunction } from 'i18next'
import { z } from 'zod'

import { parseHttpStatusCodeRules } from '@/lib/http-status-code-rules'

import {
  parseChannelRelayTimeouts,
  serializeChannelRelayTimeouts,
} from './channel-relay-timeouts'

export function createRoutingPolicySchema(t: TFunction) {
  return z.object({
    RetryTimes: z.number().int().min(0).max(99),
    ChannelRelayTimeouts: z
      .array(
        z.object({
          channelId: z.number(),
          timeoutSeconds: z.number(),
        })
      )
      .superRefine((entries, context) => {
        const seen = new Set<number>()
        let hasInvalidChannelId = false
        let hasInvalidTimeout = false
        let hasDuplicateChannelId = false

        for (const entry of entries) {
          if (!Number.isInteger(entry.channelId) || entry.channelId <= 0) {
            hasInvalidChannelId = true
          }
          if (
            !Number.isInteger(entry.timeoutSeconds) ||
            entry.timeoutSeconds <= 0 ||
            entry.timeoutSeconds > 86400
          ) {
            hasInvalidTimeout = true
          }
          if (seen.has(entry.channelId)) {
            hasDuplicateChannelId = true
          }
          seen.add(entry.channelId)
        }

        if (hasInvalidChannelId) {
          context.addIssue({
            code: 'custom',
            message: t('Channel IDs must be positive integers'),
          })
        }
        if (hasInvalidTimeout) {
          context.addIssue({
            code: 'custom',
            message: t('Timeouts must be integers between 1 and 86400 seconds'),
          })
        }
        if (hasDuplicateChannelId) {
          context.addIssue({
            code: 'custom',
            message: t('Each channel ID can only be configured once'),
          })
        }
      }),
    AutomaticRetryStatusCodes: z
      .string()
      .refine(
        (value) => parseHttpStatusCodeRules(value).ok,
        t('Invalid status code rules')
      ),
    channel_affinity_setting: z.object({
      enabled: z.boolean(),
      session_mode: z.enum(['', 'off', 'prefer', 'strict']),
      switch_on_success: z.boolean(),
      keep_on_channel_disabled: z.boolean(),
      max_entries: z.number().int().min(0),
      default_ttl_seconds: z.number().int().min(0),
      rules: z.string().superRefine((value, context) => {
        try {
          const rules: unknown = JSON.parse(value)
          if (!Array.isArray(rules)) {
            context.addIssue({
              code: 'custom',
              message: t('Rules JSON must be an array'),
            })
          }
        } catch {
          context.addIssue({
            code: 'custom',
            message: t('Invalid rules JSON format'),
          })
        }
      }),
    }),
  })
}

export type RoutingPolicyFormValues = z.infer<
  ReturnType<typeof createRoutingPolicySchema>
>

export function routingPolicyFormValues(
  options: Record<string, string>
): RoutingPolicyFormValues {
  return {
    RetryTimes: Number(options.RetryTimes),
    ChannelRelayTimeouts: parseChannelRelayTimeouts(
      options.ChannelRelayTimeouts
    ),
    AutomaticRetryStatusCodes: options.AutomaticRetryStatusCodes,
    channel_affinity_setting: {
      enabled: options['channel_affinity_setting.enabled'] === 'true',
      session_mode: (options['channel_affinity_setting.session_mode'] ||
        '') as RoutingPolicyFormValues['channel_affinity_setting']['session_mode'],
      switch_on_success:
        options['channel_affinity_setting.switch_on_success'] === 'true',
      keep_on_channel_disabled:
        options['channel_affinity_setting.keep_on_channel_disabled'] === 'true',
      max_entries: Number(options['channel_affinity_setting.max_entries']),
      default_ttl_seconds: Number(
        options['channel_affinity_setting.default_ttl_seconds']
      ),
      rules: options['channel_affinity_setting.rules'] || '[]',
    },
  }
}

export function routingPolicyOptions(
  values: RoutingPolicyFormValues
): Record<string, string> {
  return {
    RetryTimes: String(values.RetryTimes),
    ChannelRelayTimeouts: serializeChannelRelayTimeouts(
      values.ChannelRelayTimeouts
    ),
    AutomaticRetryStatusCodes: values.AutomaticRetryStatusCodes,
    ...Object.fromEntries(
      Object.entries(values.channel_affinity_setting).map(([key, value]) => [
        `channel_affinity_setting.${key}`,
        String(value),
      ])
    ),
  }
}
