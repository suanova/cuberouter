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
import { useTranslation } from 'react-i18next'

import { SectionPageLayout } from '@/components/layout'
import { Tabs, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { isAdmin } from '@/lib/role-guards'
import { useAuthStore } from '@/stores/auth-store'

import { ApiKeysDialogs } from './components/api-keys-dialogs'
import { ApiKeysPrimaryButtons } from './components/api-keys-primary-buttons'
import {
  ApiKeysProvider,
  type ApiKeysViewScope,
  useApiKeys,
} from './components/api-keys-provider'
import { ApiKeysTable } from './components/api-keys-table'

function ApiKeysContent() {
  const { t } = useTranslation()
  const { viewScope, setViewScope } = useApiKeys()
  const role = useAuthStore((state) => state.auth.user?.role)
  const canSwitchScope = isAdmin(role)

  const handleScopeChange = (scope: string) => {
    if (scope === 'all' || scope === 'self') {
      setViewScope(scope as ApiKeysViewScope)
    }
  }

  return (
    <SectionPageLayout fixedContent>
      <SectionPageLayout.Title>{t('API Keys')}</SectionPageLayout.Title>
      <SectionPageLayout.Actions>
        <div className='flex items-center gap-2'>
          {canSwitchScope && (
            <Tabs value={viewScope} onValueChange={handleScopeChange}>
              <TabsList>
                <TabsTrigger value='all'>{t('All')}</TabsTrigger>
                <TabsTrigger value='self'>{t('Only Mine')}</TabsTrigger>
              </TabsList>
            </Tabs>
          )}
          {viewScope === 'self' && <ApiKeysPrimaryButtons />}
        </div>
      </SectionPageLayout.Actions>
      <SectionPageLayout.Content>
        <ApiKeysTable />
      </SectionPageLayout.Content>
    </SectionPageLayout>
  )
}

export function ApiKeys() {
  return (
    <ApiKeysProvider>
      <ApiKeysContent />
      <ApiKeysDialogs />
    </ApiKeysProvider>
  )
}
