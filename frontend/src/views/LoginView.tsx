import { useState, type FormEvent } from 'react'
import { motion } from 'motion/react'
import { AuthError, login, signup } from '../api/services'
import { Button } from '../components/Button'
import { tr } from '../i18n'

const field =
  'w-full rounded-field bg-surface px-2.5 py-1.5 text-sm text-fg shadow-[var(--shadow-border)] outline-none ' +
  'transition-[box-shadow] duration-150 ease-out ' +
  'focus-visible:shadow-[0_0_0_1px_var(--ring-focus),0_0_0_3px_color-mix(in_srgb,var(--ring-focus)_25%,transparent)]'

// LoginView é o portão de autenticação do desktop: sem token válido, é a única
// tela renderizada (o App troca a paleta por ela). Faz login OU cadastro contra
// o gix-server (POST /v1/auth/{login,signup}); ao ter sucesso, os tokens ficam
// no client e onAuthed() libera a paleta.
export function LoginView({ lang, onAuthed }: { lang: string; onAuthed: () => void }) {
  const [mode, setMode] = useState<'login' | 'signup'>('login')
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')

  const submit = async (e: FormEvent) => {
    e.preventDefault()
    if (busy) return
    setError('')
    if (password.length < 8) { setError(tr(lang, 'password_too_short')); return }
    setBusy(true)
    try {
      if (mode === 'login') await login(email.trim(), password)
      else await signup(email.trim(), password)
      onAuthed()
    } catch (err) {
      if (err instanceof AuthError) setError(tr(lang, 'login_failed'))
      else if (err instanceof TypeError) setError(tr(lang, 'login_offline'))
      else setError(tr(lang, mode === 'login' ? 'login_failed' : 'signup_failed'))
      setBusy(false)
    }
  }

  const toggle = () => {
    setMode((m) => (m === 'login' ? 'signup' : 'login'))
    setError('')
  }

  const submitLabel = busy
    ? tr(lang, mode === 'login' ? 'login_loading' : 'signup_loading')
    : tr(lang, mode === 'login' ? 'login_submit' : 'signup_submit')

  return (
    <motion.form
      onSubmit={submit}
      initial={{ opacity: 0, y: 6, filter: 'blur(3px)' }}
      animate={{ opacity: 1, y: 0, filter: 'blur(0px)' }}
      transition={{ duration: 0.3, ease: 'easeOut' }}
      className="flex flex-col gap-3 p-4 text-fg"
    >
      <div className="space-y-1">
        <h1 className="text-base font-semibold [text-wrap:balance]">
          {tr(lang, mode === 'login' ? 'login_title' : 'signup_title')}
        </h1>
        <p className="text-xs text-muted">
          {tr(lang, mode === 'login' ? 'login_subtitle' : 'signup_subtitle')}
        </p>
      </div>

      <label className="flex flex-col gap-1">
        <span className="text-sm text-muted">{tr(lang, 'email')}</span>
        <input
          autoFocus
          type="email"
          autoComplete="email"
          className={field}
          value={email}
          onChange={(e) => setEmail(e.target.value)}
        />
      </label>

      <label className="flex flex-col gap-1">
        <span className="text-sm text-muted">{tr(lang, 'password')}</span>
        <input
          type="password"
          autoComplete={mode === 'login' ? 'current-password' : 'new-password'}
          className={field}
          value={password}
          onChange={(e) => setPassword(e.target.value)}
        />
      </label>

      {error && <span className="font-mono text-xs text-red-500">{error}</span>}

      <div className="flex items-center gap-2 pt-1">
        <Button type="submit" variant="accent" disabled={busy || !email || !password}>
          {submitLabel}
        </Button>
        <Button type="button" variant="ghost" static onClick={toggle}>
          {tr(lang, mode === 'login' ? 'go_to_signup' : 'go_to_login')}
        </Button>
      </div>
    </motion.form>
  )
}
