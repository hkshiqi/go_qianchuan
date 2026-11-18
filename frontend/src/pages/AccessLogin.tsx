import { FormEvent, useState } from 'react'
import { ArrowRight, LockKeyhole, ShieldCheck } from 'lucide-react'
import { useNavigate } from 'react-router-dom'
import { useAccessStore } from '@/store/accessStore'

export default function AccessLogin() {
  const navigate = useNavigate()
  const login = useAccessStore((state) => state.login)
  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')
  const [submitting, setSubmitting] = useState(false)
  const [error, setError] = useState('')

  const handleSubmit = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault()
    setError('')
    setSubmitting(true)
    try {
      await login(username.trim(), password)
      navigate('/login', { replace: true })
    } catch (err) {
      const message = err instanceof Error ? err.message : '登录失败，请重试'
      setError(message)
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <main className="min-h-screen bg-[#f1efe9] px-4 py-6 sm:px-6 lg:px-8">
      <div className="mx-auto flex min-h-[calc(100vh-3rem)] max-w-5xl items-center justify-center">
        <div className="grid w-full overflow-hidden rounded-[24px] border border-black/10 bg-white shadow-[0_24px_80px_-32px_rgba(20,22,27,0.35)] lg:grid-cols-[0.9fr_1.1fr]">
          <section className="relative flex min-h-[320px] flex-col justify-between bg-[#17181b] p-8 text-white sm:p-10 lg:min-h-[560px]">
            <div>
              <div className="inline-flex h-11 w-11 items-center justify-center rounded-full border border-white/15 bg-white/5">
                <ShieldCheck className="h-5 w-5 text-[#FC8805]" />
              </div>
              <p className="mt-8 text-xs font-medium uppercase tracking-[0.2em] text-white/45">Private workspace</p>
              <h1 className="mt-3 max-w-sm text-3xl font-semibold leading-tight sm:text-4xl">千川 SDK 管理平台</h1>
              <p className="mt-4 max-w-sm text-sm leading-6 text-white/58">此入口仅开放给获授权的运营成员。访问登录通过后，才可发起千川账户授权。</p>
            </div>
            <p className="mt-10 text-xs leading-5 text-white/35">服务端会话隔离 · HttpOnly Cookie · 同源 API</p>
          </section>

          <section className="flex items-center bg-white p-7 sm:p-10 lg:p-14">
            <form className="mx-auto w-full max-w-sm" onSubmit={handleSubmit}>
              <p className="text-sm font-medium text-[#FC8805]">访问控制</p>
              <h2 className="mt-2 text-2xl font-semibold tracking-tight text-[#17181B]">登录工作台</h2>
              <p className="mt-2 text-sm leading-6 text-[#626873]">请输入本系统访问账号，不是千川账号。</p>

              <label className="mt-8 block text-sm font-medium text-[#30343B]" htmlFor="access-username">账号</label>
              <input
                id="access-username"
                autoComplete="username"
                value={username}
                onChange={(event) => setUsername(event.target.value)}
                className="mt-2 h-11 w-full rounded-xl border border-[#DDE0E5] bg-[#F8F8F6] px-3.5 text-sm text-[#17181B] outline-none transition focus:border-[#FC8805] focus:ring-4 focus:ring-[#FC8805]/10"
                placeholder="访问账号"
                required
              />

              <label className="mt-5 block text-sm font-medium text-[#30343B]" htmlFor="access-password">密码</label>
              <div className="relative mt-2">
                <LockKeyhole className="pointer-events-none absolute left-3.5 top-1/2 h-4 w-4 -translate-y-1/2 text-[#8B909A]" />
                <input
                  id="access-password"
                  type="password"
                  autoComplete="current-password"
                  value={password}
                  onChange={(event) => setPassword(event.target.value)}
                  className="h-11 w-full rounded-xl border border-[#DDE0E5] bg-[#F8F8F6] pl-10 pr-3.5 text-sm text-[#17181B] outline-none transition focus:border-[#FC8805] focus:ring-4 focus:ring-[#FC8805]/10"
                  placeholder="访问密码"
                  required
                />
              </div>

              {error && <p role="alert" className="mt-4 rounded-xl bg-[#FFF0EE] px-3 py-2.5 text-sm text-[#B83A30]">{error}</p>}

              <button
                type="submit"
                disabled={submitting}
                className="mt-7 flex h-11 w-full items-center justify-center gap-2 rounded-xl bg-[#FC8805] px-4 text-sm font-semibold text-[#17181B] transition hover:bg-[#F5A13F] disabled:cursor-wait disabled:opacity-65"
              >
                {submitting ? '正在验证…' : '进入工作台'}
                {!submitting && <ArrowRight className="h-4 w-4" />}
              </button>
            </form>
          </section>
        </div>
      </div>
    </main>
  )
}
