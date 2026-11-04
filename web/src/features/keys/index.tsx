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
import { getRouteApi } from '@tanstack/react-router'
import { useTranslation } from 'react-i18next'

import { SectionPageLayout } from '@/components/layout'
import { Tabs, TabsList, TabsTrigger } from '@/components/ui/tabs'

import { ApiKeysDialogs } from './components/api-keys-dialogs'
import { ApiKeysPrimaryButtons } from './components/api-keys-primary-buttons'
import { ApiKeysProvider } from './components/api-keys-provider'
import { ApiKeysTable } from './components/api-keys-table'
import { SelfOrganizationTokensTable } from './components/self-organization-tokens-table'

const route = getRouteApi('/_authenticated/keys/')

export function ApiKeys() {
  const { t } = useTranslation()
  const search = route.useSearch()
  const navigate = route.useNavigate()
  const tab = search.tab ?? 'personal'

  return (
    <ApiKeysProvider>
      <SectionPageLayout fixedContent>
        <SectionPageLayout.Title>{t('API Keys')}</SectionPageLayout.Title>
        <SectionPageLayout.Actions>
          {tab === 'personal' ? <ApiKeysPrimaryButtons /> : null}
        </SectionPageLayout.Actions>
        <SectionPageLayout.Content>
          <div className='flex h-full min-h-0 flex-col gap-3'>
            <Tabs
              value={tab}
              onValueChange={(value) =>
                navigate({
                  search: (previous) => ({
                    ...previous,
                    tab: value as 'personal' | 'organization',
                  }),
                })
              }
              className='shrink-0'
            >
              <TabsList>
                <TabsTrigger value='personal'>{t('Personal keys')}</TabsTrigger>
                <TabsTrigger value='organization'>
                  {t('Organization keys')}
                </TabsTrigger>
              </TabsList>
            </Tabs>
            <div className='min-h-0 flex-1'>
              {tab === 'personal' ? (
                <ApiKeysTable />
              ) : (
                <SelfOrganizationTokensTable />
              )}
            </div>
          </div>
        </SectionPageLayout.Content>
      </SectionPageLayout>

      <ApiKeysDialogs />
    </ApiKeysProvider>
  )
}
