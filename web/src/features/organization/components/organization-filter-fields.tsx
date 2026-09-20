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
import {
  LogsFilterField,
  LogsFilterInput,
} from '@/features/usage-logs/components/logs-filter-toolbar'

/**
 * A keyword box for the organization tables.
 *
 * Each one is its own filter: the endpoints match a keyword against a named
 * field, so a box per field is the whole filter — there is no free-text search
 * across them.
 */
export function OrganizationTextFilter(props: {
  value?: string
  placeholder: string
  onChange: (value: string) => void
}) {
  return (
    <LogsFilterField>
      <LogsFilterInput
        value={props.value ?? ''}
        placeholder={props.placeholder}
        onChange={(event) => props.onChange(event.target.value)}
      />
    </LogsFilterField>
  )
}
