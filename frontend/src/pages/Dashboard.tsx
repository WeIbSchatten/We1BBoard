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
      <div className="page-head">
        <div>
          <h1 className="page-title">{tr('dashboard')}</h1>
          <p className="page-sub">{tr('welcome')}</p>
        </div>
        <button className="btn" onClick={restart}>{tr('restart')}</button>
      </div>

      <div className="stat-grid">
        <div className="stat">
          <div className="k">{tr('status')} Xray</div>
          <div className="v">
            <span className={`badge ${st?.xrayRunning ? 'on' : 'off'}`}>
              {st?.xrayRunning ? tr('running') : tr('stopped')}
            </span>
          </div>
          <div className="hint">panel v{st?.version || '—'}</div>
        </div>
        <div className="stat">
          <div className="k">CPU</div>
          <div className="v">{(st?.cpu ?? 0).toFixed(1)}%</div>
          <div className="hint">live · 5s</div>
        </div>
        <div className="stat">
          <div className="k">RAM</div>
          <div className="v">{(st?.memory ?? 0).toFixed(1)}%</div>
          <div className="hint">host memory</div>
        </div>
        <div className="stat">
          <div className="k">Extra cores</div>
          <div className="v">{st?.extra?.length ?? 0}</div>
          <div className="hint">mtg / tuic / hy2 / tg</div>
        </div>
      </div>
      {msg && <p className="page-sub" style={{ marginTop: 14 }}>{msg}</p>}
    </div>
  )
}
