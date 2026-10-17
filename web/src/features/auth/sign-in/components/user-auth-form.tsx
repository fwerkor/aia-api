import { useStatus } from '@/hooks/use-status'
import { OAuthProviders } from '@/features/auth/components/oauth-providers'
import type { AuthFormProps } from '@/features/auth/types'

export function UserAuthForm({ redirectTo }: AuthFormProps) {
  const { status } = useStatus()
  const forum = status ? {
    ...status,
    github_oauth: false,
    discord_oauth: false,
    linuxdo_oauth: false,
    telegram_oauth: false,
    wechat_login: false,
    custom_oauth_providers: [],
    oidc_enabled: true,
    oidc_display_name: 'AIA 论坛',
  } : null

  return (
    <div className='space-y-4'>
      <p className='text-center text-sm text-muted-foreground'>使用 AIA 论坛账号登录</p>
      <OAuthProviders status={forum} redirectTo={redirectTo} />
    </div>
  )
}
