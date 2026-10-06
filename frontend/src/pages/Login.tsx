import { FormEvent, useState } from 'react'
import { api } from '../api'
import { useApp } from '../AppContext'

export function LoginPage() {
  const { tr, setAuthed } = useApp()
  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')
  const [code, setCode] = useState('')
  const [need2fa, setNeed2fa] = useState(false)
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)

  async function onSubmit(e: FormEvent) {
    e.preventDefault()
    setError('')
    setBusy(true)
    try {
      const body: Record<string, string> = { username, password }
      if (code.trim()) body.code = code.trim()
      await api('/login', { method: 'POST', body: JSON.stringify(body) })
      setAuthed(true)
    } catch (err) {
      const msg = err instanceof Error ? err.message : 'error'
      if (/two-factor/i.test(msg)) setNeed2fa(true)
      setError(msg)
    } finally {
      setBusy(false)
    }
  }

  return (
    <div className="login-stage">
      <div className="login-atmosphere" aria-hidden />
      <form className="login-panel" onSubmit={onSubmit}>
        <div className="login-brand">We1BBoard</div>
        <p className="login-lead">{tr('welcome')}</p>
        <div className="field">
          <label className="login-label" htmlFor="login-user">{tr('username')}</label>
          <input id="login-user" className="input" autoComplete="username" value={username} onChange={(e) => setUsername(e.target.value)} required />
        </div>
        <div className="field">
          <label className="login-label" htmlFor="login-pass">{tr('password')}</label>
          <input id="login-pass" className="input" type="password" autoComplete="current-password" value={password} onChange={(e) => setPassword(e.target.value)} required />
        </div>
        {(need2fa || code) && (
          <div className="field">
            <label className="login-label" htmlFor="login-2fa">{tr('twoFactorCode')}</label>
            <input
              id="login-2fa"
              className="input"
              inputMode="numeric"
              autoComplete="one-time-code"
              value={code}
              onChange={(e) => setCode(e.target.value)}
              placeholder="123456"
              required={need2fa}
            />
          </div>
        )}
        {error && <p className="error">{error}</p>}
        <button className="btn login-btn" type="submit" disabled={busy}>
          {busy ? '…' : tr('login')}
        </button>
      </form>
      <style>{`
        .login-stage {
          min-height: 100vh;
          display: grid;
          place-items: center;
          padding: 1.5rem;
          position: relative;
          overflow: hidden;
        }
        .login-atmosphere {
          position: absolute; inset: 0;
          background:
            radial-gradient(900px 520px at 18% 20%, var(--glow), transparent 60%),
            radial-gradient(700px 480px at 88% 10%, rgba(14,165,233,.16), transparent 55%),
            linear-gradient(160deg, var(--bg-deep), var(--bg));
          animation: drift 14s ease-in-out infinite alternate;
        }
        @keyframes drift {
          from { transform: scale(1) translateY(0); }
          to { transform: scale(1.04) translateY(-12px); }
        }
        .login-panel {
          position: relative;
          z-index: 1;
          width: min(420px, 100%);
          padding: 2rem 1.7rem 1.7rem;
          border-radius: 22px;
          border: 1px solid var(--border);
          background: color-mix(in srgb, var(--bg-solid) 82%, transparent);
          backdrop-filter: blur(18px);
          animation: rise .35s ease;
        }
        .login-brand {
          font-size: clamp(2.1rem, 5vw, 2.65rem);
          font-weight: 800;
          letter-spacing: -0.05em;
          line-height: 1;
          margin-bottom: 0.55rem;
          background: linear-gradient(120deg, var(--text) 30%, var(--accent));
          -webkit-background-clip: text;
          background-clip: text;
          color: transparent;
        }
        .login-lead {
          margin: 0 0 1.4rem;
          color: var(--text-muted);
          font-size: 1rem;
        }
        .login-label {
          display: block;
          margin-bottom: 0.4rem;
          color: var(--text-muted);
          font-size: 0.88rem;
          font-weight: 600;
        }
        .login-btn { width: 100%; margin-top: 0.25rem; padding: 0.8rem 1rem; font-size: 1rem; }
      `}</style>
    </div>
  )
}
