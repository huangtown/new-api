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
export type ChannelRelayTimeoutEntry = {
  channelId: number
  timeoutSeconds: number
}

export function parseChannelRelayTimeouts(
  value: string
): ChannelRelayTimeoutEntry[] {
  try {
    const parsed: unknown = JSON.parse(value || '{}')
    if (!parsed || typeof parsed !== 'object' || Array.isArray(parsed)) {
      return []
    }

    return Object.entries(parsed)
      .map(([channelId, timeoutSeconds]) => ({
        channelId: Number(channelId),
        timeoutSeconds: Number(timeoutSeconds),
      }))
      .filter(
        (entry) =>
          Number.isInteger(entry.channelId) &&
          entry.channelId > 0 &&
          Number.isInteger(entry.timeoutSeconds) &&
          entry.timeoutSeconds > 0
      )
      .sort((left, right) => left.channelId - right.channelId)
  } catch {
    return []
  }
}

export function serializeChannelRelayTimeouts(
  entries: ChannelRelayTimeoutEntry[]
): string {
  const sortedEntries = [...entries].sort(
    (left, right) => left.channelId - right.channelId
  )
  const timeouts: Record<string, number> = {}
  for (const entry of sortedEntries) {
    timeouts[String(entry.channelId)] = entry.timeoutSeconds
  }
  return JSON.stringify(timeouts)
}
