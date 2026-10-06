import { useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { api, type ServerHistory } from '../api'
import { useApp } from '../AppContext'
import { BackupModal } from '../components/BackupModal'
import { PanelUpdateModal } from '../components/PanelUpdateModal'

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

type XrayVer = { current: string; bin: string }

function formatUptime(sec?: number): string {
  if (!sec || sec <= 0) return '—'
  const h = Math.floor(sec / 3600)
  const m = Math.floor((sec % 3600) / 60)
  const s = sec % 60
  if (h > 0) return `${h}h ${m}m`
  if (m > 0) return `${m}m ${s}s`
  return `${s}s`
}

function Sparkline({ values, color = 'var(--accent)' }: { values: number[]; color?: string }) {
  if (!values.length) {
    return <div className="sparkline empty" />
  }
  const w = 120
  const h = 28
  const min = Math.min(...values)
  const max = Math.max(...values)
  const span = max - min || 1
  const pts = values
    .map((v, i) => {
      const x = values.length === 1 ? w / 2 : (i / (values.length - 1)) * w
      const y = h - ((v - min) / span) * (h - 4) - 2
      return `${x.toFixed(1)},${y.toFixed(1)}`
    })
    .join(' ')
  return (
    <svg className="sparkline" viewBox={`0 0 ${w} ${h}`} width={w} height={h} aria-hidden>
      <polyline fill="none" stroke={color} strokeWidth="1.5" points={pts} />
    </svg>
  )
}

export function DashboardPage() {
  const { tr } = useApp()
  const [st, setSt] = useState<Status | null>(null)
  const [hist, setHist] = useState<ServerHistory | null>(null)
  const [xrayVer, setXrayVer] = useState<XrayVer | null>(null)
  const [msg, setMsg] = useState('')
  const [backupOpen, setBackupOpen] = useState(false)
  const [updateOpen, setUpdateOpen] = useState(false)
  const [installBusy, setInstallBusy] = useState(false)

  async function load() {
    setSt(await api<Status>('/server/status'))
  }

  async function loadHistory() {
    setHist(await api<ServerHistory>('/server/history'))
  }

  async function loadXrayVer() {
    try {
      setXrayVer(await api<XrayVer>('/server/xray-version'))
    } catch { /* ignore */ }
  }

  useEffect(() => {
    load().catch(console.error)
    loadHistory().catch(() => {})
    loadXrayVer().catch(() => {})
    const t = setInterval(() => {
      load().catch(() => {})
      loadHistory().catch(() => {})
    }, 5000)
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

  async function installXray() {
    if (!confirm(tr('installXrayConfirm'))) return
    setInstallBusy(true)
    setMsg('')
    try {
      const r = await api<{ message?: string; current?: string }>('/server/install-xray', {
        method: 'POST',
        body: JSON.stringify({}),
      })
      setMsg(r.message || 'OK')
      await loadXrayVer()
      await load()
    } catch (e) {
      setMsg(e instanceof Error ? e.message : 'error')
    } finally {
      setInstallBusy(false)
    }
  }

  return (
    <div>
      <div className="page-head">
        <div>
          <h1 className="page-title">{tr('dashboard')}</h1>
          <p className="page-sub">{tr('welcome')}</p>
        </div>
        <div className="row-actions">
          <button className="btn secondary" type="button" onClick={() => setBackupOpen(true)}>
            {tr('backupRestore')}
          </button>
          <button className="btn secondary" type="button" onClick={() => setUpdateOpen(true)}>
            {tr('panelUpdate')}
          </button>
          <button className="btn secondary" type="button" disabled={installBusy} onClick={() => void installXray()}>
            {tr('installXray')}
          </button>
          <button className="btn" type="button" onClick={restart}>{tr('restart')}</button>
        </div>
      </div>

      <div className="toolbar" style={{ marginBottom: '1.1rem' }}>
        <Link className="btn secondary" to="/inbounds">{tr('inbounds')}</Link>
        <Link className="btn secondary" to="/clients">{tr('clients')}</Link>
        <Link className="btn secondary" to="/logs">{tr('logs')}</Link>
        <span className="toolbar-spacer" />
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
            {xrayVer?.current ? `xray ${xrayVer.current} · ` : ''}
            uptime {formatUptime(st?.xrayUptime)} · panel v{st?.version || '—'}
          </div>
        </Link>
        <div className="stat">
          <div className="k">CPU</div>
          <div className="v">{(st?.cpu ?? 0).toFixed(1)}%</div>
          <Sparkline values={hist?.cpu || []} />
          <div className="hint">10s · last {hist?.cpu?.length ?? 0}</div>
        </div>
        <div className="stat">
          <div className="k">RAM</div>
          <div className="v">{(st?.memory ?? 0).toFixed(1)}%</div>
          <Sparkline values={hist?.mem || []} color="color-mix(in srgb, var(--accent) 70%, #8cf)" />
          <div className="hint">host memory</div>
        </div>
        <Link to="/logs" className="stat stat-link">
          <div className="k">TCP / UDP</div>
          <div className="v">{st?.tcpCount ?? 0} / {st?.udpCount ?? 0}</div>
          <div style={{ display: 'flex', gap: 8, marginTop: 4 }}>
            <Sparkline values={(hist?.tcp || []).map(Number)} />
            <Sparkline values={(hist?.udp || []).map(Number)} color="color-mix(in srgb, var(--danger) 50%, var(--accent))" />
          </div>
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

      <BackupModal open={backupOpen} onClose={() => setBackupOpen(false)} />
      <PanelUpdateModal open={updateOpen} onClose={() => setUpdateOpen(false)} />

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
        .sparkline {
          display: block;
          margin-top: 6px;
          opacity: 0.9;
        }
        .sparkline.empty {
          height: 28px;
          margin-top: 6px;
          background: color-mix(in srgb, var(--border) 40%, transparent);
          border-radius: 4px;
        }
      `}</style>
    </div>
  )
}
