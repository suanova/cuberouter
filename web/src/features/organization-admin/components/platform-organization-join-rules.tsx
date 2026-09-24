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
import { useQuery } from '@tanstack/react-query'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { ConfirmDialog } from '@/components/confirm-dialog'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Skeleton } from '@/components/ui/skeleton'
import { Textarea } from '@/components/ui/textarea'

import {
  createOrganizationJoinRules,
  deleteOrganizationJoinRule,
  listOrganizationJoinRules,
  type OrganizationJoinRule,
  type OrganizationJoinRuleLineError,
  type OrganizationJoinRuleNotice,
} from '@/features/organization/api'
import {
  OrganizationSection,
  OrganizationSectionRefresh,
} from '@/features/organization/components/organization-section'
import { getServerErrorMessageKey } from '@/lib/server-error-message'

type PlatformOrganizationJoinRulesProps = {
  organizationId: number
  /** A dissolved organization keeps its rules; nothing may change them. */
  readOnly: boolean
}

/**
 * Who joins this organization without being invited.
 *
 * A rule is a domain, a `*.domain` wildcard or one exact address, and an account
 * that registers with a matching address becomes a member on the spot. That
 * makes the rules the organization's membership boundary, which is why they live
 * on the platform page for the root alone rather than with the members: an
 * administrator able to widen them could hand account access to anyone.
 *
 * The whole batch is written or none of it is — the backend refuses the request
 * when any line is unusable and answers with a reason per line — so this renders
 * those reasons beside the lines that caused them and leaves the pasted text in
 * place, where the operator can fix the offending line and submit again.
 */
export function PlatformOrganizationJoinRules(
  props: PlatformOrganizationJoinRulesProps
) {
  const { t } = useTranslation()
  const [patterns, setPatterns] = useState('')
  const [reason, setReason] = useState('')
  const [isSubmitting, setIsSubmitting] = useState(false)
  const [lineErrors, setLineErrors] = useState<OrganizationJoinRuleLineError[]>(
    []
  )
  const [notices, setNotices] = useState<OrganizationJoinRuleNotice[]>([])
  const [pendingDelete, setPendingDelete] = useState<OrganizationJoinRule | null>(
    null
  )

  const {
    data: rules = [],
    isLoading,
    isFetching,
    refetch,
  } = useQuery({
    queryKey: ['platform-organization-join-rules', props.organizationId],
    queryFn: async (): Promise<OrganizationJoinRule[]> => {
      const result = await listOrganizationJoinRules(props.organizationId)
      return result.data ?? []
    },
  })

  const submittedPatterns = patterns
    .split('\n')
    .map((line) => line.trim())
    .filter(Boolean)

  // The preview describes each distinct rule once: the same line pasted twice is
  // one rule, since the backend keeps the first and ignores the rest.
  const previewPatterns = [...new Set(submittedPatterns)]

  const onSubmit = async () => {
    setIsSubmitting(true)
    setLineErrors([])
    setNotices([])
    try {
      const result = await createOrganizationJoinRules(props.organizationId, {
        patterns: submittedPatterns,
        reason: reason.trim(),
      })
      setPatterns('')
      setReason('')
      setNotices(result.data?.notices ?? [])
      toast.success(t('Join rules saved'))
      await refetch()
    } catch (error) {
      // The interceptor is off for this request (`skipErrorHandler`), so the
      // refusal is read here: a batch that was refused line by line carries its
      // reasons in the 400, and only a refusal without them is worth a toast.
      const payload = (
        error as {
          response?: {
            data?: {
              line_errors?: OrganizationJoinRuleLineError[]
              notices?: OrganizationJoinRuleNotice[]
              message?: string
            }
          }
        }
      )?.response?.data
      setLineErrors(payload?.line_errors ?? [])
      setNotices(payload?.notices ?? [])
      if (!payload?.line_errors?.length) {
        // The backend answers a stable code plus English text by this subsystem's
        // convention, so the copy the operator reads comes from the shared code
        // table — the same one the axios interceptor uses. Only a refusal whose
        // code has no entry there falls back to the backend's own message.
        const messageKey = getServerErrorMessageKey(error)
        if (messageKey) {
          toast.error(t(messageKey))
        } else {
          toast.error(payload?.message || t('Failed to save the join rules'))
        }
      }
    } finally {
      setIsSubmitting(false)
    }
  }

  const [deleteReason, setDeleteReason] = useState('')
  const [isDeleting, setIsDeleting] = useState(false)

  const onDelete = async () => {
    if (!pendingDelete) return
    setIsDeleting(true)
    try {
      const result = await deleteOrganizationJoinRule(
        props.organizationId,
        pendingDelete.id,
        deleteReason.trim()
      )
      if (!result.success) {
        toast.error(result.message || t('Failed to delete the join rule'))
        return
      }
      setPendingDelete(null)
      setDeleteReason('')
      toast.success(t('Join rule deleted'))
      await refetch()
    } catch {
      // Reported by the interceptor. The dialog stays open so the operator can
      // try again, and the reason they wrote is still in it.
    } finally {
      setIsDeleting(false)
    }
  }

  /** What one rule will actually match, worked out before it is written: a
   * wildcard is the easiest line to paste while meaning "only this domain". */
  const matchPreview = (pattern: string): string => {
    if (pattern.includes('@')) {
      return t('Will match only {{address}}', { address: pattern })
    }
    if (pattern.startsWith('*.')) {
      const apex = pattern.slice(2)
      return t('Will match {{apex}} and any subdomain, e.g. {{sample}}', {
        apex,
        sample: `someone@mail.${apex}`,
      })
    }
    return t('Will match only {{domain}}', { domain: pattern })
  }

  return (
    <OrganizationSection
      icon='settings'
      title={t('Join Rules')}
      description={t(
        'Accounts that register with a matching email address join this organization automatically. Only a platform root can change these rules.'
      )}
      count={rules.length}
      actions={
        <OrganizationSectionRefresh
          onClick={() => void refetch()}
          isFetching={isFetching}
        />
      }
      scroll
    >
      <Card>
        <CardHeader>
          <CardTitle>{t('Add rules')}</CardTitle>
          <CardDescription>
            {t('One entry per line: a domain, *.domain, or one exact address.')}
          </CardDescription>
        </CardHeader>
        <CardContent className='space-y-3'>
          <div className='flex flex-col gap-2'>
            <Label htmlFor='join-rule-patterns'>{t('Patterns')}</Label>
            <Textarea
              id='join-rule-patterns'
              rows={6}
              value={patterns}
              disabled={props.readOnly || isSubmitting}
              onChange={(event) => setPatterns(event.target.value)}
              placeholder={'*.enterprise.com\nuser-a@enterprise.com'}
            />
          </div>
          {previewPatterns.length > 0 ? (
            <ul className='text-muted-foreground space-y-1 text-xs'>
              {previewPatterns.map((pattern) => (
                <li key={pattern}>{matchPreview(pattern)}</li>
              ))}
            </ul>
          ) : null}
          <div className='flex flex-col gap-2'>
            <Label htmlFor='join-rule-reason'>{t('Reason')}</Label>
            <Input
              id='join-rule-reason'
              value={reason}
              disabled={props.readOnly || isSubmitting}
              onChange={(event) => setReason(event.target.value)}
              placeholder={t('Why are these rules being added?')}
            />
          </div>

          {lineErrors.length > 0 ? (
            <Alert variant='destructive'>
              <AlertTitle>{t('These lines cannot be saved')}</AlertTitle>
              <AlertDescription>
                <ul className='space-y-1'>
                  {lineErrors.map((error) => (
                    <li key={`${error.line}-${error.pattern}`}>
                      {t('Line {{line}}: {{pattern}}', {
                        line: error.line,
                        pattern: error.pattern,
                      })}
                      {' — '}
                      {error.kind === 'conflict' && error.organization_name
                        ? t('It conflicts with a rule of {{organization}}.', {
                            organization: error.organization_name,
                          })
                        : t('It is not a usable domain or email address.')}
                    </li>
                  ))}
                </ul>
              </AlertDescription>
            </Alert>
          ) : null}

          {notices.length > 0 ? (
            <Alert>
              <AlertTitle>{t('Saved, with a note')}</AlertTitle>
              <AlertDescription>
                <ul className='space-y-1'>
                  {notices.map((notice) => (
                    <li key={`${notice.pattern}-${notice.pattern_conflict}`}>
                      {/* A domain rule on a public mailbox provider is allowed but
                          admits everyone who registers there, which is why the
                          backend flags it with its own kind: the provider domain
                          travels in `pattern_conflict`, and no organization is on
                          the other side of this note. */}
                      {notice.kind === 'public_mailbox_provider'
                        ? t(
                            '{{pattern}} admits anyone who registers with a {{provider}} mailbox. List exact addresses instead if you only mean specific people.',
                            {
                              pattern: notice.pattern,
                              provider: notice.pattern_conflict,
                            }
                          )
                        : t('{{pattern}} overlaps {{organization}} rule {{conflict}}.', {
                            pattern: notice.pattern,
                            organization: notice.organization_name,
                            conflict: notice.pattern_conflict,
                          })}
                    </li>
                  ))}
                </ul>
              </AlertDescription>
            </Alert>
          ) : null}

          <div className='flex justify-end'>
            <Button
              type='button'
              disabled={
                props.readOnly ||
                isSubmitting ||
                submittedPatterns.length === 0 ||
                reason.trim().length === 0
              }
              onClick={() => void onSubmit()}
            >
              {isSubmitting ? t('Saving...') : t('Save rules')}
            </Button>
          </div>
        </CardContent>
      </Card>

      {/* Still reading, nothing configured, or a list of rules — three states,
          and only one of them renders at a time. */}
      {isLoading && <Skeleton className='h-32 w-full' />}
      {!isLoading && rules.length === 0 && (
        <p className='text-muted-foreground text-sm'>
          {t('No join rules yet. Nobody joins this organization automatically.')}
        </p>
      )}
      {!isLoading && rules.length > 0 && (
        <ul className='space-y-2'>
          {rules.map((rule) => (
            <li
              key={rule.id}
              className='flex items-center justify-between gap-3 rounded-xl border px-3 py-2.5'
            >
              <div className='min-w-0'>
                <p className='truncate font-medium'>{rule.pattern}</p>
                <p className='text-muted-foreground text-xs'>
                  {rule.match_type === 'email'
                    ? t('Exact address')
                    : t('Email domain')}
                  {' · '}
                  {rule.creator_username || rule.creator_display_name || '-'}
                </p>
              </div>
              <Button
                type='button'
                variant='outline'
                size='sm'
                disabled={props.readOnly}
                onClick={() => setPendingDelete(rule)}
              >
                {t('Delete')}
              </Button>
            </li>
          ))}
        </ul>
      )}

      <ConfirmDialog
        open={pendingDelete !== null}
        onOpenChange={(open) => {
          if (open) return
          // The reason is written for the rule this dialog was opened for, so it
          // leaves with it: a cancelled dialog must not arm the next one with a
          // free-text reason that would then be recorded against another rule.
          setPendingDelete(null)
          setDeleteReason('')
        }}
        title={t('Delete join rule')}
        desc={t(
          'Registrations matching {{pattern}} will no longer join this organization. It does not remove members who already joined.',
          { pattern: pendingDelete?.pattern ?? '' }
        )}
        confirmText={t('Delete')}
        destructive
        disabled={deleteReason.trim().length === 0}
        isLoading={isDeleting}
        handleConfirm={() => void onDelete()}
      >
        <Label htmlFor='join-rule-delete-reason'>{t('Reason')}</Label>
        <Textarea
          id='join-rule-delete-reason'
          rows={3}
          value={deleteReason}
          onChange={(event) => setDeleteReason(event.target.value)}
        />
      </ConfirmDialog>
    </OrganizationSection>
  )
}
