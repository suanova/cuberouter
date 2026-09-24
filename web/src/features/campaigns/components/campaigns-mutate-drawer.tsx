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
import { zodResolver } from '@hookform/resolvers/zod'
import { useMutation } from '@tanstack/react-query'
import { useEffect } from 'react'
import { useForm, type Resolver } from 'react-hook-form'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import {
  SideDrawerSection,
  SideDrawerSectionHeader,
  sideDrawerContentClassName,
  sideDrawerFooterClassName,
  sideDrawerFormClassName,
  sideDrawerHeaderClassName,
} from '@/components/drawer-layout'
import { Button } from '@/components/ui/button'
import {
  Form,
  FormControl,
  FormDescription,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from '@/components/ui/form'
import { Input } from '@/components/ui/input'
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import {
  Sheet,
  SheetClose,
  SheetContent,
  SheetDescription,
  SheetFooter,
  SheetHeader,
  SheetTitle,
} from '@/components/ui/sheet'
import { Textarea } from '@/components/ui/textarea'

import { handleServerError } from '@/lib/handle-server-error'

import { createCampaign, updateCampaign } from '../api'
import {
  CAMPAIGN_CODE_COUNT_MAX,
  CAMPAIGN_STATUSES,
  CAMPAIGN_TYPE,
  CAMPAIGN_TYPES,
} from '../constants'
import {
  buildCampaignPayload,
  campaignToFormDefaults,
  getCampaignFormSchema,
  type CampaignFormData,
} from '../lib/campaign-form'
import type { Campaign } from '../types'
import { useCampaigns } from './campaigns-provider'

type CampaignsMutateDrawerProps = {
  open: boolean
  onOpenChange: (open: boolean) => void
  currentRow?: Campaign
}

export function CampaignsMutateDrawer({
  open,
  onOpenChange,
  currentRow,
}: CampaignsMutateDrawerProps) {
  const { t } = useTranslation()
  const isUpdate = !!currentRow
  const { refresh } = useCampaigns()

  const form = useForm<CampaignFormData>({
    resolver: zodResolver(getCampaignFormSchema(t)) as Resolver<
      CampaignFormData,
      unknown,
      CampaignFormData
    >,
    defaultValues: campaignToFormDefaults(null),
  })

  // Load existing data when updating
  useEffect(() => {
    if (open) {
      form.reset(campaignToFormDefaults(isUpdate ? currentRow : null))
    }
  }, [open, isUpdate, currentRow, form])

  const handleOpenChange = (value: boolean) => {
    onOpenChange(value)
    if (!value) {
      form.reset()
    }
  }

  const saveMutation = useMutation({
    mutationFn: (payload: Record<string, unknown>) =>
      isUpdate ? updateCampaign(payload) : createCampaign(payload),
    onSuccess: (res) => {
      if (res.success) {
        toast.success(isUpdate ? t('Campaign updated') : t('Campaign created'))
        if (res.warning) {
          toast.warning(res.warning)
        }
        handleOpenChange(false)
        refresh()
      } else {
        toast.error(res.message || t('Failed to save campaign'))
      }
    },
    onError: handleServerError,
  })

  const onSubmit = (values: CampaignFormData) => {
    saveMutation.mutate(
      buildCampaignPayload(values, isUpdate ? currentRow?.id : undefined)
    )
  }

  const campaignType = form.watch('type')
  const isInvitation = campaignType === CAMPAIGN_TYPE.INVITATION

  // The two selects resolve their trigger text from the items the root is
  // given, so each list is built once here and both the options and the
  // registry are drawn from it.
  const typeItems = Object.entries(CAMPAIGN_TYPES).map(([value, config]) => ({
    value,
    label: t(config.labelKey),
  }))
  const statusItems = Object.entries(CAMPAIGN_STATUSES).map(
    ([value, config]) => ({ value, label: t(config.labelKey) })
  )

  return (
    <Sheet open={open} onOpenChange={handleOpenChange}>
      <SheetContent className={sideDrawerContentClassName('sm:max-w-[600px]')}>
        <SheetHeader className={sideDrawerHeaderClassName()}>
          <SheetTitle>
            {isUpdate ? t('Update Campaign') : t('Create Campaign')}
          </SheetTitle>
          <SheetDescription>
            {isUpdate
              ? t('Update the campaign by providing necessary info.')
              : t('Add a new campaign by providing necessary info.')}{' '}
            {t('Click save when you&apos;re done.')}
          </SheetDescription>
        </SheetHeader>
        <Form {...form}>
          <form
            id='campaign-form'
            onSubmit={form.handleSubmit(onSubmit)}
            className={sideDrawerFormClassName()}
          >
            <SideDrawerSection>
              <SideDrawerSectionHeader
                title={t('Basic Info')}
                description={t('Campaign name, type and active window.')}
              />
              <FormField
                control={form.control}
                name='name'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Name')}</FormLabel>
                    <FormControl>
                      <Input {...field} placeholder={t('Enter a name')} />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />

              <FormField
                control={form.control}
                name='description'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Description')}</FormLabel>
                    <FormControl>
                      <Textarea
                        {...field}
                        rows={3}
                        placeholder={t('Describe this campaign')}
                      />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />

              <div className='grid grid-cols-1 gap-4 sm:grid-cols-2'>
                <FormField
                  control={form.control}
                  name='type'
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>{t('Type')}</FormLabel>
                      <Select
                        items={typeItems}
                        onValueChange={field.onChange}
                        value={field.value}
                        disabled={isUpdate}
                      >
                        <FormControl>
                          <SelectTrigger>
                            <SelectValue placeholder={t('Select campaign type')}>
                              {CAMPAIGN_TYPES[field.value]
                                ? t(CAMPAIGN_TYPES[field.value].labelKey)
                                : field.value}
                            </SelectValue>
                          </SelectTrigger>
                        </FormControl>
                        <SelectContent alignItemWithTrigger={false}>
                          <SelectGroup>
                            {typeItems.map((item) => (
                              <SelectItem key={item.value} value={item.value}>
                                {item.label}
                              </SelectItem>
                            ))}
                          </SelectGroup>
                        </SelectContent>
                      </Select>
                      <FormDescription>
                        {isUpdate
                          ? t('Campaign type cannot be changed after creation')
                          : t('Which event grants the reward')}
                      </FormDescription>
                      <FormMessage />
                    </FormItem>
                  )}
                />

                <FormField
                  control={form.control}
                  name='status'
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>{t('Status')}</FormLabel>
                      <Select
                        items={statusItems}
                        onValueChange={(value) => field.onChange(Number(value))}
                        value={String(field.value)}
                      >
                        <FormControl>
                          <SelectTrigger>
                            <SelectValue placeholder={t('Select status')}>
                              {CAMPAIGN_STATUSES[field.value]
                                ? t(CAMPAIGN_STATUSES[field.value].labelKey)
                                : String(field.value)}
                            </SelectValue>
                          </SelectTrigger>
                        </FormControl>
                        <SelectContent alignItemWithTrigger={false}>
                          <SelectGroup>
                            {statusItems.map((item) => (
                              <SelectItem key={item.value} value={item.value}>
                                {item.label}
                              </SelectItem>
                            ))}
                          </SelectGroup>
                        </SelectContent>
                      </Select>
                      <FormMessage />
                    </FormItem>
                  )}
                />
              </div>

              <div className='grid grid-cols-1 gap-4 sm:grid-cols-2'>
                <FormField
                  control={form.control}
                  name='start_at'
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>{t('Start Time')}</FormLabel>
                      <FormControl>
                        <Input {...field} type='datetime-local' />
                      </FormControl>
                      <FormDescription>
                        {t('Leave empty to start immediately')}
                      </FormDescription>
                      <FormMessage />
                    </FormItem>
                  )}
                />

                <FormField
                  control={form.control}
                  name='end_at'
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>{t('End Time')}</FormLabel>
                      <FormControl>
                        <Input {...field} type='datetime-local' />
                      </FormControl>
                      <FormDescription>
                        {t('Leave empty for never expires')}
                      </FormDescription>
                      <FormMessage />
                    </FormItem>
                  )}
                />
              </div>
            </SideDrawerSection>

            <SideDrawerSection>
              <SideDrawerSectionHeader
                title={t('Reward Config')}
                description={t(
                  'Quota and limits for the generated redemption codes.'
                )}
              />
              <div className='grid grid-cols-1 gap-4 sm:grid-cols-2'>
                <FormField
                  control={form.control}
                  name='quota'
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>{t('Quota')}</FormLabel>
                      <FormControl>
                        <Input
                          {...field}
                          type='number'
                          min='0'
                          onChange={(e) =>
                            field.onChange(
                              Number.parseInt(e.target.value, 10) || 0
                            )
                          }
                        />
                      </FormControl>
                      <FormDescription>
                        {t('Quota granted per reward redemption code')}
                      </FormDescription>
                      <FormMessage />
                    </FormItem>
                  )}
                />

                <FormField
                  control={form.control}
                  name='redemption_name'
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>{t('Redemption Name')}</FormLabel>
                      <FormControl>
                        <Input
                          {...field}
                          placeholder={t('Name for generated redemption codes')}
                        />
                      </FormControl>
                      <FormDescription>
                        {t('Display name for the generated codes')}
                      </FormDescription>
                      <FormMessage />
                    </FormItem>
                  )}
                />

                <FormField
                  control={form.control}
                  name='expire_days'
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>{t('Expire Days')}</FormLabel>
                      <FormControl>
                        <Input
                          {...field}
                          type='number'
                          min='0'
                          onChange={(e) =>
                            field.onChange(
                              Number.parseInt(e.target.value, 10) || 0
                            )
                          }
                        />
                      </FormControl>
                      <FormDescription>
                        {t('Days until generated codes expire; 0 means never')}
                      </FormDescription>
                      <FormMessage />
                    </FormItem>
                  )}
                />

                <FormField
                  control={form.control}
                  name='max_participants'
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>{t('Max Participants')}</FormLabel>
                      <FormControl>
                        <Input
                          {...field}
                          type='number'
                          min='0'
                          onChange={(e) =>
                            field.onChange(
                              Number.parseInt(e.target.value, 10) || 0
                            )
                          }
                        />
                      </FormControl>
                      <FormDescription>
                        {t('0 means unlimited participants')}
                      </FormDescription>
                      <FormMessage />
                    </FormItem>
                  )}
                />

                <FormField
                  control={form.control}
                  name='max_rewards_per_user'
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>{t('Max Rewards Per User')}</FormLabel>
                      <FormControl>
                        <Input
                          {...field}
                          type='number'
                          min='0'
                          onChange={(e) =>
                            field.onChange(
                              Number.parseInt(e.target.value, 10) || 0
                            )
                          }
                        />
                      </FormControl>
                      <FormDescription>
                        {t('0 means unlimited rewards per user')}
                      </FormDescription>
                      <FormMessage />
                    </FormItem>
                  )}
                />
              </div>
            </SideDrawerSection>

            {isInvitation && (
              <SideDrawerSection>
                <SideDrawerSectionHeader
                  title={t('Invitation Config')}
                  description={t(
                    'Invitee account and the redemption code pool.'
                  )}
                />
                <div className='grid grid-cols-1 gap-4 sm:grid-cols-2'>
                  <FormField
                    control={form.control}
                    name='invitee_user_id'
                    render={({ field }) => (
                      <FormItem>
                        <FormLabel>{t('Invitee User ID')}</FormLabel>
                        <FormControl>
                          <Input
                            {...field}
                            type='number'
                            min='0'
                            onChange={(e) =>
                              field.onChange(
                                Number.parseInt(e.target.value, 10) || 0
                              )
                            }
                          />
                        </FormControl>
                        <FormDescription>
                          {t('The user account invited by this campaign')}
                        </FormDescription>
                        <FormMessage />
                      </FormItem>
                    )}
                  />

                  <FormField
                    control={form.control}
                    name='invitee_username'
                    render={({ field }) => (
                      <FormItem>
                        <FormLabel>{t('Invitee Username')}</FormLabel>
                        <FormControl>
                          <Input
                            {...field}
                            placeholder={t('Snapshot of the invitee username')}
                          />
                        </FormControl>
                        <FormDescription>
                          {t('Recorded at creation for display only')}
                        </FormDescription>
                        <FormMessage />
                      </FormItem>
                    )}
                  />

                  <FormField
                    control={form.control}
                    name='code_count'
                    render={({ field }) => (
                      <FormItem>
                        <FormLabel>{t('Code Pool Size')}</FormLabel>
                        <FormControl>
                          <Input
                            {...field}
                            type='number'
                            min='1'
                            max={CAMPAIGN_CODE_COUNT_MAX}
                            onChange={(e) =>
                              field.onChange(
                                Number.parseInt(e.target.value, 10) || 1
                              )
                            }
                          />
                        </FormControl>
                        <FormDescription>
                          {t(
                            'Pool size, max 1000; codes are generated incrementally'
                          )}
                        </FormDescription>
                        <FormMessage />
                      </FormItem>
                    )}
                  />
                </div>
              </SideDrawerSection>
            )}
          </form>
        </Form>
        <SheetFooter className={sideDrawerFooterClassName()}>
          <SheetClose render={<Button variant='outline' />}>
            {t('Close')}
          </SheetClose>
          <Button
            form='campaign-form'
            type='submit'
            disabled={saveMutation.isPending}
          >
            {saveMutation.isPending ? t('Saving...') : t('Save changes')}
          </Button>
        </SheetFooter>
      </SheetContent>
    </Sheet>
  )
}
