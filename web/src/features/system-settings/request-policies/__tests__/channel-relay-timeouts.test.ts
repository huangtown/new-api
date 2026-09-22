import { describe, expect, test } from 'vitest'

import {
  parseChannelRelayTimeouts,
  serializeChannelRelayTimeouts,
} from '../channel-relay-timeouts.ts'

describe('channel relay timeout serialization', () => {
  test('parses and sorts valid channel timeout entries', () => {
    expect(parseChannelRelayTimeouts('{"59":600,"12":120}')).toEqual([
      { channelId: 12, timeoutSeconds: 120 },
      { channelId: 59, timeoutSeconds: 600 },
    ])
  })

  test('returns an empty list for invalid persisted data', () => {
    expect(parseChannelRelayTimeouts('[]')).toEqual([])
    expect(parseChannelRelayTimeouts('invalid')).toEqual([])
  })

  test('serializes entries in channel ID order', () => {
    expect(
      serializeChannelRelayTimeouts([
        { channelId: 59, timeoutSeconds: 600 },
        { channelId: 12, timeoutSeconds: 120 },
      ])
    ).toBe('{"12":120,"59":600}')
  })
})
