import { useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { api } from '../api'
import { useApp } from '../AppContext'

type Status = {
  version: string
  xrayRunning: boolean
  cpu: number
  memory: number
  tcpCount?: number
  udpCount?: number
  xrayUptime?: number
  goroutines?: number
  extra: { inboundId: number; running: boolean; name: string }[]
}

function formatUptime(sec?: number): string {
  if (!sec || sec <= 0) return '—'
  const h = Math.floor(sec / 3600)
  const m = Math.floor((sec % 3600) / 60)
  const s = sec % 60
  if (h > 0) return `${h}h ${m}m`
  if (m > 0) return `${m}m ${s}s`
  return `${s}s`
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
        <Link to="/xray" className="stat stat-link">
          <div className="k">{tr('status')} Xray</div>
          <div className="v">
            <span className={`badge ${st?.xrayRunning ? 'on' : 'off'}`}>
              {st?.xrayRunning ? tr('running') : tr('stopped')}
            </span>
          </div>
          <div className="hint">
            uptime {formatUptime(st?.xrayUptime)} · panel v{st?.version || '—'}
          </div>
        </Link>
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
        <Link to="/logs" className="stat stat-link">
          <div className="k">TCP / UDP</div>
          <div className="v">{st?.tcpCount ?? 0} / {st?.udpCount ?? 0}</div>
          <div className="hint">
            connections{st?.goroutines != null ? ` · ${st.goroutines} goroutines` : ''}
          </div>
        </Link>
        <div className="stat">
          <div className="k">Extra cores</div>
          <div className="v">{st?.extra?.length ?? 0}</div>
          <div className="hint">mtg / tuic / hy2 / tg</div>
        </div>
      </div>
      {st && !st.xrayRunning && (
        <div className="alert-warn" style={{ marginTop: 16 }}>
          Xray {tr('stopped')}.{' '}
          <Link to="/logs">{tr('logs')}</Link>
          {' · '}
          <Link to="/xray">Xray</Link>
        </div>
      )}
      {msg && <p className="page-sub" style={{ marginTop: 14 }}>{msg}</p>}
      <style>{`
        .stat-link {
          text-decoration: none;
          color: inherit;
          transition: border-color 0.15s, background 0.15s;
        }
        .stat-link:hover {
          border-color: color-mix(in srgb, var(--accent) 45%, var(--border));
          background: color-mix(in srgb, var(--accent) 6%, transparent);
        }
        .alert-warn {
          padding: 0.85rem 1rem;
          border-radius: 12px;
          border: 1px solid color-mix(in srgb, var(--danger) 40%, var(--border));
          background: color-mix(in srgb, var(--danger) 10%, transparent);
          color: var(--text);
          font-weight: 600;
        }
        .alert-warn a { color: var(--accent); }
      `}</style>
    </div>
  )
}
