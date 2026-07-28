import assert from 'node:assert/strict'
import { describe, test } from 'node:test'

import {
  parseChannelRelayTimeouts,
  serializeChannelRelayTimeouts,
} from './channel-relay-timeouts.ts'

describe('channel relay timeout serialization', () => {
  test('parses and sorts valid channel timeout entries', () => {
    assert.deepEqual(parseChannelRelayTimeouts('{"59":600,"12":120}'), [
      { channelId: 12, timeoutSeconds: 120 },
      { channelId: 59, timeoutSeconds: 600 },
    ])
  })

  test('returns an empty list for invalid persisted data', () => {
    assert.deepEqual(parseChannelRelayTimeouts('[]'), [])
    assert.deepEqual(parseChannelRelayTimeouts('invalid'), [])
  })

  test('serializes entries in channel ID order', () => {
    assert.equal(
      serializeChannelRelayTimeouts([
        { channelId: 59, timeoutSeconds: 600 },
        { channelId: 12, timeoutSeconds: 120 },
      ]),
      '{"12":120,"59":600}'
    )
  })
})
