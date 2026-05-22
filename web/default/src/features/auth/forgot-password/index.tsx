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
import { Link } from '@tanstack/react-router'
import { useTranslation } from 'react-i18next'
import { useStatus } from '@/hooks/use-status'
import { AuthLayout } from '../auth-layout'
import { ForgotPasswordForm } from './components/forgot-password-form'

export function ForgotPassword() {
  const { t } = useTranslation()
  const { status } = useStatus()
  const registerEnabled =
    status?.register_enabled ??
    status?.data?.register_enabled ??
    true
  const showSignUpLink =
    !status?.self_use_mode_enabled && registerEnabled

  return (
    <AuthLayout>
      <div className='w-full space-y-10'>
        <div className='space-y-3 text-center'>
          <h2 className='text-3xl font-bold tracking-tight text-gray-900 dark:text-foreground'>
            {t('Forgot password')}
          </h2>
          <p className='text-muted-foreground text-sm sm:text-[0.9375rem]'>
            {t(
              'Enter your registered email and we will send you a link to reset your password.'
            )}
          </p>
          {showSignUpLink && (
            <p className='text-sm text-gray-500 sm:text-base dark:text-muted-foreground'>
              {t("Don't have an account?")}{' '}
              <Link
                to='/sign-up'
                className='font-medium text-indigo-600 hover:text-indigo-700 dark:text-indigo-400 dark:hover:text-indigo-300'
              >
                {t('Sign up')}
              </Link>
            </p>
          )}
        </div>

        <ForgotPasswordForm className='space-y-0' />
      </div>
    </AuthLayout>
  )
}
