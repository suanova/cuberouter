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
export const TOOL_STATUS_KEYS: Record<string, string> = {
  get_my_account: 'Checking your account…',
  query_my_usage: 'Checking your usage…',
  explain_my_charge: 'Checking that charge…',
  list_my_keys: 'Checking your API keys…',
  get_my_tasks: 'Checking your tasks…',
  search_help_docs: 'Searching help docs…',
  get_org_summary: 'Checking organization summary…',
  query_org_usage: 'Checking organization usage…',
  list_org_members: 'Checking organization members…',
}

export const ASSISTANT_ERROR_KEYS: Record<string, string> = {
  disabled: 'The account assistant is not enabled.',
  rate_limited: 'Too many requests. Please try again later.',
  daily_limit: 'Daily question limit reached. Please contact human support.',
  token_budget: 'Daily assistant quota reached. Please contact human support.',
  empty_message: 'Please enter a question.',
  empty_answer:
    'The assistant could not generate an answer. Please contact human support.',
  unavailable:
    'The assistant is temporarily unavailable. Please try again later or contact human support.',
}

export const ASSISTANT_ERROR_FALLBACK =
  'The assistant is temporarily unavailable. Please try again later or contact human support.'

export const ASSISTANT_REFUSAL_TEXT =
  "I can't answer this question from your account data. Please contact human support."
