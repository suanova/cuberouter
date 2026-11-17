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

import { Alert, AlertDescription } from '@/components/ui/alert'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'

import { SettingsSwitchField } from '../components/settings-form-layout'
import { useSystemOptions } from '../hooks/use-system-options'

export interface PaymentAsiaSettingsValues {
  PaymentAsiaSandbox: boolean
  PaymentAsiaMerchantToken: string
  PaymentAsiaSecret: string
  PaymentAsiaSandboxMerchantToken: string
  PaymentAsiaSandboxSecret: string
  PaymentAsiaCurrency: string
  PaymentAsiaUnitPrice: number
  PaymentAsiaMinTopUp: number
  PaymentAsiaNotifyUrl: string
  PaymentAsiaReturnUrl: string
}

type PaymentAsiaSettingsSectionProps = {
  values: PaymentAsiaSettingsValues
  onValueChange: <K extends keyof PaymentAsiaSettingsValues>(
    key: K,
    value: PaymentAsiaSettingsValues[K]
  ) => void
}

export function PaymentAsiaSettingsSection(
  props: PaymentAsiaSettingsSectionProps
) {
  const { t } = useTranslation()
  const { data } = useSystemOptions()
  const configuredServerAddress =
    data?.data?.find((option) => option.key === 'ServerAddress')?.value ?? ''
  const serverAddress =
    configuredServerAddress.trim().replace(/\/+$/, '') ||
    (typeof window === 'undefined' ? '' : window.location.origin)

  return (
    <div className='space-y-4 pt-4'>
      <div>
        <h3 className='text-lg font-medium'>{t('PaymentAsia Gateway')}</h3>
        <p className='text-muted-foreground text-sm'>
          {t(
            'PaymentAsia hosted payment page gateway for Hong Kong payment channels.'
          )}
        </p>
      </div>
      <Alert>
        <AlertDescription className='text-xs'>
          {t(
            'Fill in the credentials of the active environment from the PaymentAsia merchant portal, then add PaymentAsia channels under Payment methods.'
          )}
        </AlertDescription>
      </Alert>

      <div className='grid gap-4 sm:grid-cols-2'>
        <SettingsSwitchField
          checked={props.values.PaymentAsiaSandbox}
          onCheckedChange={(value) =>
            props.onValueChange('PaymentAsiaSandbox', value)
          }
          label={t('Sandbox mode')}
          description={t(
            'Use the PaymentAsia sandbox gateway and sandbox credentials for testing.'
          )}
          className='py-0'
        />
      </div>

      <div className='grid grid-cols-2 gap-4'>
        <div className='grid gap-3'>
          <div className='grid gap-1.5'>
            <Label>{t('Merchant Token')}</Label>
            <Input
              type='password'
              autoComplete='new-password'
              value={props.values.PaymentAsiaMerchantToken}
              onChange={(event) =>
                props.onValueChange(
                  'PaymentAsiaMerchantToken',
                  event.target.value
                )
              }
            />
          </div>
          <div className='grid gap-1.5'>
            <Label>{t('Merchant Secret')}</Label>
            <Input
              type='password'
              autoComplete='new-password'
              value={props.values.PaymentAsiaSecret}
              onChange={(event) =>
                props.onValueChange('PaymentAsiaSecret', event.target.value)
              }
            />
          </div>
        </div>
        <div className='grid gap-3'>
          <div className='grid gap-1.5'>
            <Label>{t('Sandbox Merchant Token')}</Label>
            <Input
              type='password'
              autoComplete='new-password'
              value={props.values.PaymentAsiaSandboxMerchantToken}
              onChange={(event) =>
                props.onValueChange(
                  'PaymentAsiaSandboxMerchantToken',
                  event.target.value
                )
              }
            />
          </div>
          <div className='grid gap-1.5'>
            <Label>{t('Sandbox Merchant Secret')}</Label>
            <Input
              type='password'
              autoComplete='new-password'
              value={props.values.PaymentAsiaSandboxSecret}
              onChange={(event) =>
                props.onValueChange(
                  'PaymentAsiaSandboxSecret',
                  event.target.value
                )
              }
            />
          </div>
        </div>
      </div>

      <div className='grid grid-cols-3 gap-4'>
        <div className='grid gap-1.5'>
          <Label>{t('Currency')}</Label>
          <Select
            items={[
              { value: 'HKD', label: t('HKD') },
              { value: 'USD', label: t('USD') },
            ]}
            value={props.values.PaymentAsiaCurrency}
            onValueChange={(value) => {
              if (value) props.onValueChange('PaymentAsiaCurrency', value)
            }}
          >
            <SelectTrigger>
              <SelectValue placeholder={t('Select currency')} />
            </SelectTrigger>
            <SelectContent alignItemWithTrigger={false}>
              <SelectGroup>
                <SelectItem value='HKD'>{t('HKD')}</SelectItem>
                <SelectItem value='USD'>{t('USD')}</SelectItem>
              </SelectGroup>
            </SelectContent>
          </Select>
        </div>
        <div className='grid gap-1.5'>
          <Label>{t('Unit price')}</Label>
          <Input
            type='number'
            step={0.01}
            min={0}
            value={props.values.PaymentAsiaUnitPrice}
            onChange={(event) =>
              props.onValueChange(
                'PaymentAsiaUnitPrice',
                event.target.value === '' ? 0 : event.target.valueAsNumber
              )
            }
          />
          <p className='text-muted-foreground text-xs'>
            {t(
              'Amount charged in the configured payment currency for one display unit.'
            )}
          </p>
        </div>
        <div className='grid gap-1.5'>
          <Label>{t('Minimum top-up quantity')}</Label>
          <Input
            type='number'
            min={1}
            value={props.values.PaymentAsiaMinTopUp}
            onChange={(event) =>
              props.onValueChange(
                'PaymentAsiaMinTopUp',
                event.target.value === '' ? 1 : event.target.valueAsNumber
              )
            }
          />
          <p className='text-muted-foreground text-xs'>
            {t('Global minimum top-up quantity for PaymentAsia.')}
          </p>
        </div>
      </div>

      <div className='grid grid-cols-2 gap-4'>
        <div className='grid gap-1.5'>
          <Label>{t('Callback notification URL')}</Label>
          <Input
            placeholder='https://example.com/api/paymentasia/notify'
            value={props.values.PaymentAsiaNotifyUrl}
            onChange={(event) =>
              props.onValueChange('PaymentAsiaNotifyUrl', event.target.value)
            }
          />
          <p className='text-muted-foreground text-xs'>
            {t('Leave blank to use the default notify URL.')}
          </p>
        </div>
        <div className='grid gap-1.5'>
          <Label>{t('Payment return URL')}</Label>
          <Input
            placeholder='https://example.com/wallet'
            value={props.values.PaymentAsiaReturnUrl}
            onChange={(event) =>
              props.onValueChange('PaymentAsiaReturnUrl', event.target.value)
            }
          />
          <p className='text-muted-foreground text-xs'>
            {t('Leave blank to use the default wallet return URL.')}
          </p>
        </div>
      </div>

      <div className='bg-muted/30 rounded-md border p-3'>
        <p className='text-muted-foreground text-xs'>
          {t('Default callback URL')}
        </p>
        <code className='font-mono text-xs break-all'>
          {`${serverAddress}/api/paymentasia/notify`}
        </code>
      </div>
    </div>
  )
}
