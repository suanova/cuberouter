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
import { useNavigate } from '@tanstack/react-router'
import { CheckIcon } from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { Alert, AlertDescription } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { api } from '@/lib/api'

import { AuthLayout } from '../auth-layout'

export type ResetPasswordSearchParams = {
  email?: string
  token?: string
}

type ResetPasswordConfirmProps = ResetPasswordSearchParams

export function ResetPasswordConfirm({
  email,
  token,
}: ResetPasswordConfirmProps) {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const [loading, setLoading] = useState(false)
  const [resetSucceeded, setResetSucceeded] = useState(false)
  const [linkInvalidated, setLinkInvalidated] = useState(false)

  const isValidResetLink = Boolean(email && token)
  const showInvalidLink = !isValidResetLink || linkInvalidated

  async function handleSubmit() {
    if (!isValidResetLink || !email || !token) {
      toast.error(t('Invalid reset link, please request a new password reset'))
      return
    }

    setLoading(true)
    try {
      const res = await api.post('/api/user/reset', { email, token }, {
        skipBusinessError: true,
      } as Record<string, unknown>)

      if (res?.data?.success) {
        setResetSucceeded(true)
        toast.success(
          t(
            'Password reset, the new password has been sent to your email'
          )
        )
      } else {
        // 链接失效（token 缺失/过期/已消费）：后端以 code 标记，切换到
        // 无效链接态（Banner + 返回登录），不再停留在表单
        if (res?.data?.code === 'PASSWORD_RESET_LINK_INVALID') {
          setLinkInvalidated(true)
          return
        }
        // 其他业务失败（如邮件发送失败）：用后端已按请求语言翻译的消息
        // 提示；表单保留，用户可重试
        const message = res?.data?.message
        toast.error(
          typeof message === 'string' && message
            ? message
            : t('Invalid reset link, please request a new password reset')
        )
      }
    } catch {
      // Errors handled by global interceptor
    } finally {
      setLoading(false)
    }
  }

  // 无效链接（缺 email/token，或提交时后端判定 token 已失效）：
  // 仅警告横幅 + 返回登录，不渲染表单
  if (showInvalidLink) {
    return (
      <AuthLayout>
        <div className='w-full space-y-8'>
          <div className='space-y-2'>
            <h2 className='text-center text-2xl font-semibold tracking-tight sm:text-left'>
              {t('Reset password')}
            </h2>
            <p className='text-muted-foreground text-left text-sm sm:text-base'>
              {t('auth.resetPasswordConfirm.description')}
            </p>
          </div>

          <Alert variant='destructive'>
            <AlertDescription>
              {t('Invalid reset link, please request a new password reset.')}
            </AlertDescription>
          </Alert>

          <Button
            variant='link'
            className='w-full'
            onClick={() => navigate({ to: '/sign-in', replace: true })}
          >
            {t('Back to login')}
          </Button>
        </div>
      </AuthLayout>
    )
  }

  // 成功态：新密码已发送至邮箱，隐藏表单
  if (resetSucceeded) {
    return (
      <AuthLayout>
        <div className='w-full space-y-8'>
          <div className='space-y-2'>
            <h2 className='text-center text-2xl font-semibold tracking-tight sm:text-left'>
              {t('Reset password')}
            </h2>
            <p className='text-muted-foreground text-left text-sm sm:text-base'>
              {t('auth.resetPasswordConfirm.success')}
            </p>
          </div>

          <Alert variant='success'>
            <CheckIcon className='h-4 w-4' />
            <AlertDescription>
              {t(
                'Password reset, the new password has been sent to your email. Please check your email and log in with the new password.'
              )}
            </AlertDescription>
          </Alert>

          <Button
            className='w-full'
            onClick={() => navigate({ to: '/sign-in', replace: true })}
          >
            {t('auth.resetPasswordConfirm.backToLogin')}
          </Button>
        </div>
      </AuthLayout>
    )
  }

  return (
    <AuthLayout>
      <div className='w-full space-y-8'>
        <div className='space-y-2'>
          <h2 className='text-center text-2xl font-semibold tracking-tight sm:text-left'>
            {t('Reset password')}
          </h2>
          <p className='text-muted-foreground text-left text-sm sm:text-base'>
            {t('auth.resetPasswordConfirm.description')}
          </p>
        </div>

        <div className='space-y-4'>
          <div className='space-y-2'>
            <Label htmlFor='email'>{t('Email')}</Label>
            <Input
              id='email'
              type='email'
              value={email || ''}
              disabled
              placeholder={t('Waiting for email...')}
            />
          </div>

          <Button
            className='w-full'
            onClick={handleSubmit}
            disabled={loading}
          >
            {t('auth.resetPasswordConfirm.confirm')}
          </Button>

          <Button
            variant='link'
            className='w-full'
            onClick={() => navigate({ to: '/sign-in', replace: true })}
          >
            {t('Back to login')}
          </Button>
        </div>
      </div>
    </AuthLayout>
  )
}
