import { FormEvent, useState } from 'react'
import { api } from '../api'
import { useApp } from '../AppContext'

export function LoginPage() {
  const { tr, setAuthed } = useApp()
  const [username, setUsername] = useState('admin')
  const [password, setPassword] = useState('admin')
  const [error, setError] = useState('')

  async function onSubmit(e: FormEvent) {
    e.preventDefault()
    setError('')
    try {
      await api('/login', { method: 'POST', body: JSON.stringify({ username, password }) })
      setAuthed(true)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'error')
    }
  }

  return (
    <div className="login-wrap">
      <form className="card login-card" onSubmit={onSubmit}>
        <h1>{tr('brand')}</h1>
        <p className="page-sub">{tr('welcome')}</p>
        <div className="field">
          <label className="label">{tr('username')}</label>
          <input className="input" value={username} onChange={(e) => setUsername(e.target.value)} />
        </div>
        <div className="field">
          <label className="label">{tr('password')}</label>
          <input className="input" type="password" value={password} onChange={(e) => setPassword(e.target.value)} />
        </div>
        {error && <p className="error">{error}</p>}
        <button className="btn" type="submit">
          {tr('login')}
        </button>
      </form>
      <style>{`
        .login-wrap { min-height: 100vh; display: grid; place-items: center; padding: 1rem; background: var(--bg); }
        .login-card { width: min(400px, 100%); }
        .login-card h1 { margin: 0; font-size: 2rem; font-weight: 800; color: var(--accent); letter-spacing: -0.04em; }
        .login-card .btn { width: 100%; margin-top: 0.4rem; }
      `}</style>
    </div>
  )
}
