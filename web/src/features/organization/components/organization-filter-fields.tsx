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
import { useTranslation } from 'react-i18next'

import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
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

/** Stands in for "every value" in the selects below. */
const ANY_VALUE = ''

/**
 * One of the closed-set filters.
 *
 * A single select rather than a faceted box, because the endpoint compares one
 * value exactly. The options carry the same labels the tables badge their cells
 * with, so a state cannot be named one way in the column and another in the
 * filter that selects it.
 *
 * "Every value" is the absence of a choice rather than a choice of its own: the
 * sentinel below never reaches the URL, it clears the filter.
 */
export function OrganizationSelectFilter(props: {
  value?: string
  placeholder: string
  /** The value the endpoint compares, and the label to show for it. */
  options: Array<{ value: string; label: string }>
  allLabel: string
  onChange: (value: string) => void
}) {
  const { t } = useTranslation()

  // The trigger resolves its text from the items the root is given, so the list
  // is built once here and the options are drawn from it.
  const items = [
    { value: ANY_VALUE, label: props.allLabel },
    ...props.options.map((option) => ({
      value: option.value,
      label: t(option.label),
    })),
  ]

  return (
    <Select
      items={items}
      value={props.value ?? ANY_VALUE}
      onValueChange={(next) => props.onChange(next ?? ANY_VALUE)}
    >
      <SelectTrigger className='h-8 w-full' size='sm'>
        <SelectValue placeholder={props.placeholder} />
      </SelectTrigger>
      <SelectContent>
        {items.map((item) => (
          <SelectItem key={item.value} value={item.value}>
            {item.label}
          </SelectItem>
        ))}
      </SelectContent>
    </Select>
  )
}
