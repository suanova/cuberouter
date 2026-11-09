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

/**
 * Keep only positive integer organization IDs, deduplicated and sorted so the
 * saved value and the change comparison are stable regardless of pick order.
 */
export function normalizeDshOrganizationIds(values: unknown[]): number[] {
  const ids = new Set<number>()
  for (const value of values) {
    const id = typeof value === 'number' ? value : Number(value)
    if (Number.isInteger(id) && id > 0) {
      ids.add(id)
    }
  }
  return [...ids].sort((a, b) => a - b)
}

/**
 * Add one organization to the selection without disturbing the rest. Binding a
 * channel also allow-lists the organization on the server, so the local chip
 * has to follow or the next save would drop it again.
 */
export function mergeDshOrganizationSelection(
  selected: string[],
  organizationId: number
): string[] {
  if (!Number.isInteger(organizationId) || organizationId <= 0) {
    return selected
  }
  const value = String(organizationId)
  return selected.includes(value) ? selected : [...selected, value]
}

export type DshChannelChoice = {
  // 0 stands for "unbound"; the channel ids are always positive.
  id: number
  name: string
  disabled: boolean
  reason: 'disabled' | 'bound_elsewhere' | 'missing' | null
}

type DshChannelChoiceInput = {
  id: number
  name: string
  enabled: boolean
  boundOrganizationId: number
}

/**
 * Build the channel dropdown for one organization: "unbound" first, then every
 * DSH channel. A channel that is stopped or already owned by another
 * organization stays visible but cannot be picked — except for the channel this
 * organization is currently bound to, which must remain selectable so the row
 * reflects reality even when that channel was since disabled or deleted.
 */
export function buildDshChannelChoices(
  channels: DshChannelChoiceInput[],
  organizationId: number,
  currentChannelId: number
): DshChannelChoice[] {
  const choices: DshChannelChoice[] = [
    { id: 0, name: '', disabled: false, reason: null },
  ]
  let hasCurrent = currentChannelId <= 0
  for (const channel of channels) {
    if (channel.id === currentChannelId) {
      hasCurrent = true
      choices.push({
        id: channel.id,
        name: channel.name,
        disabled: false,
        reason: null,
      })
      continue
    }
    let reason: DshChannelChoice['reason'] = null
    if (!channel.enabled) {
      reason = 'disabled'
    } else if (
      channel.boundOrganizationId > 0 &&
      channel.boundOrganizationId !== organizationId
    ) {
      reason = 'bound_elsewhere'
    }
    choices.push({
      id: channel.id,
      name: channel.name,
      disabled: reason !== null,
      reason,
    })
  }
  if (!hasCurrent) {
    // The bound channel was deleted: keep its id visible so an administrator
    // can tell what the organization points at before re-binding.
    choices.splice(1, 0, {
      id: currentChannelId,
      name: '',
      disabled: false,
      reason: 'missing',
    })
  }
  return choices
}
