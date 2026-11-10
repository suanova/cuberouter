/*
Copyright (C) 2023-2026 QuantumNous
Copyright (C) 2026 CubeRouter

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
import * as z from 'zod'

const IPV4_PATTERN =
  /^(0|[1-9]\d{0,2})\.(0|[1-9]\d{0,2})\.(0|[1-9]\d{0,2})\.(0|[1-9]\d{0,2})$/

function isIPv4(value: string) {
  const match = IPV4_PATTERN.exec(value)
  if (!match) return false
  return match.slice(1).every((octet) => Number(octet) <= 255)
}

function isIPv6(value: string) {
  // The URL parser stops at the closing bracket, so "::1]#" would otherwise
  // parse as a valid host. Restrict the characters before using it.
  if (!value.includes(':') || !/^[0-9a-fA-F:.]+$/.test(value)) return false
  try {
    new URL(`http://[${value}]`)
    return true
  } catch {
    return false
  }
}

export function isIPOrCIDR(value: string) {
  const [address, prefix, ...rest] = value.split('/')
  if (rest.length > 0) return false
  if (prefix !== undefined && !/^\d{1,3}$/.test(prefix)) return false
  if (isIPv4(address)) return prefix === undefined || Number(prefix) <= 32
  if (isIPv6(address)) return prefix === undefined || Number(prefix) <= 128
  return false
}

export function splitWhitelist(value: string) {
  return value
    .split('\n')
    .map((entry) => entry.trim())
    .filter(Boolean)
}

// The message is an i18n key; FormMessage translates whatever zod reports.
export const whitelistSchema = z.object({
  whitelist: z.string().superRefine((value, ctx) => {
    if (splitWhitelist(value).some((entry) => !isIPOrCIDR(entry))) {
      ctx.addIssue({
        code: 'custom',
        message: 'Enter an IP address or CIDR range on each line',
      })
    }
  }),
})

export type WhitelistFormValues = z.infer<typeof whitelistSchema>
