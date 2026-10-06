import { useEffect, useState } from 'react'
import { api } from '../api'
import { useApp } from '../AppContext'

type Status = {
  version: string
  xrayRunning: boolean
  cpu: number
  memory: number
  extra: { inboundId: number; running: boolean; name: string }[]
}

export function DashboardPage() {
  const { tr } = useApp()
  const [st, setSt] = useState<Status | null>(null)
  const [msg, setMsg] = useState('')

  async function load() {
    setSt(await api<Status>('/server/status'))
  }

  useEffect(() => {
    load().catch(console.error)
    const t = setInterval(() => load().catch(() => {}), 5000)
    return () => clearInterval(t)
  }, [])

  async function restart() {
    setMsg('')
    try {
      await api('/xray/restart', { method: 'POST' })
      await load()
      setMsg('OK')
    } catch (e) {
      setMsg(e instanceof Error ? e.message : 'error')
    }
  }

  return (
    <div>
      <h1 className="page-title">{tr('dashboard')}</h1>
      <p className="page-sub">{tr('welcome')}</p>
      <div className="grid2">
        <div className="card">
          <div className="label">{tr('status')} Xray</div>
          <div style={{ fontSize: '1.4rem', fontWeight: 800 }}>
            {st?.xrayRunning ? tr('running') : tr('stopped')}
          </div>
          <div style={{ color: 'var(--text-muted)', marginTop: 8 }}>v{st?.version || '—'}</div>
          <button className="btn" style={{ marginTop: 12 }} onClick={restart}>
            {tr('restart')}
          </button>
          {msg && <p className="page-sub">{msg}</p>}
        </div>
        <div className="card">
          <div className="label">CPU / RAM</div>
          <div style={{ fontSize: '1.4rem', fontWeight: 800 }}>
            {(st?.cpu ?? 0).toFixed(1)}% / {(st?.memory ?? 0).toFixed(1)}%
          </div>
          <div style={{ marginTop: 12, color: 'var(--text-muted)' }}>
            Extra processes: {st?.extra?.length ?? 0}
          </div>
        </div>
      </div>
    </div>
  )
}
