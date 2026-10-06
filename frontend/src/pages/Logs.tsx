import { useCallback, useEffect, useMemo, useState } from 'react'
import { api } from '../api'
import { useApp } from '../AppContext'

type LogSource = 'error' | 'access' | 'process'
type Tab = LogSource | 'panel' | 'config'

type LogsResp = { source: string; lines: string[]; path: string; hint?: string }
type Status = { xrayRunning: boolean }
type ConfigIssue = { tag: string; reason: string }

const LINE_OPTS = [50, 100, 200, 500] as const

function lineLevelClass(line: string): string {
  const u = line.toUpperCase()
  if (/\bERROR\b|\bFATAL\b|\bCRITICAL\b/.test(u)) return 'log-error'
  if (/\bWARN(ING)?\b/.test(u)) return 'log-warn'
  if (/\bINFO\b|\bDEBUG\b/.test(u)) return 'log-info'
  return ''
}

export function LogsPage() {
  const { tr } = useApp()
  const [tab, setTab] = useState<Tab>('error')
  const [lines, setLines] = useState<number>(200)
  const [logLines, setLogLines] = useState<string[]>([])
  const [path, setPath] = useState('')
  const [configJson, setConfigJson] = useState('')
  const [issues, setIssues] = useState<ConfigIssue[]>([])
  const [autoRefresh, setAutoRefresh] = useState(false)
  const [running, setRunning] = useState<boolean | null>(null)
  const [busy, setBusy] = useState(false)
  const [msg, setMsg] = useState('')

  const loadStatus = useCallback(async () => {
    try {
      const st = await api<Status>('/server/status')
      setRunning(st.xrayRunning)
    } catch { /* ignore */ }
  }, [])

  const loadIssues = useCallback(async () => {
    try {
      const data = await api<{ issues: ConfigIssue[] }>('/xray/config-issues')
      setIssues(data.issues || [])
    } catch {
      setIssues([])
    }
  }, [])

  const loadLogs = useCallback(async () => {
    if (tab === 'config') return
    setBusy(true)
    setMsg('')
    try {
      const url = tab === 'panel'
        ? `/logs/panel?lines=${lines}`
        : `/xray/logs?source=${tab}&lines=${lines}`
      const data = await api<LogsResp>(url)
      setLogLines(data.lines || [])
      setPath(data.path || '')
      await loadIssues()
    } catch (e) {
      setMsg(e instanceof Error ? e.message : 'error')
    } finally {
      setBusy(false)
    }
  }, [tab, lines, loadIssues])

  const loadConfig = useCallback(async () => {
    setBusy(true)
    setMsg('')
    try {
      const cfg = await api<unknown>('/xray/config')
      setConfigJson(JSON.stringify(cfg, null, 2))
      await loadIssues()
    } catch (e) {
      setMsg(e instanceof Error ? e.message : 'error')
    } finally {
      setBusy(false)
    }
  }, [loadIssues])

  const refresh = useCallback(async () => {
    await loadStatus()
    if (tab === 'config') await loadConfig()
    else await loadLogs()
  }, [tab, loadStatus, loadConfig, loadLogs])

  useEffect(() => {
    void refresh()
  }, [refresh])

  useEffect(() => {
    if (!autoRefresh || tab === 'config') return
    const id = setInterval(() => {
      void loadLogs()
      void loadStatus()
    }, 5000)
    return () => clearInterval(id)
  }, [autoRefresh, tab, loadLogs, loadStatus])

  async function restart() {
    setMsg('')
    try {
      await api('/xray/restart', { method: 'POST' })
      await refresh()
      setMsg('OK')
    } catch (e) {
      setMsg(e instanceof Error ? e.message : 'error')
    }
  }

  async function disableInvalid() {
    setMsg('')
    try {
      const res = await api<{ count: number }>('/inbounds/disable-invalid', { method: 'POST' })
      setMsg(`${tr('disabledCount')}: ${res.count}`)
      await loadIssues()
    } catch (e) {
      setMsg(e instanceof Error ? e.message : 'error')
    }
  }

  function downloadLogs() {
    const body = tab === 'config' ? configJson : logLines.join('\n')
    const blob = new Blob([body], { type: 'text/plain;charset=utf-8' })
    const url = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url
    a.download = tab === 'config' ? 'xray-config.json' : `we1b-${tab}.log`
    a.click()
    URL.revokeObjectURL(url)
  }

  const colorize = tab === 'error' || tab === 'panel' || tab === 'process'

  const renderedLines = useMemo(() => {
    if (!colorize) return null
    return logLines.map((line, i) => (
      <div key={i} className={lineLevelClass(line)}>{line || ' '}</div>
    ))
  }, [logLines, colorize])

  const tabs: { id: Tab; label: string }[] = [
    { id: 'error', label: 'error' },
    { id: 'access', label: 'access' },
    { id: 'process', label: 'process' },
    { id: 'panel', label: tr('panelLogs') },
    { id: 'config', label: tr('config') },
  ]

  return (
    <div>
      <div className="page-head">
        <div>
          <h1 className="page-title">{tr('logs')}</h1>
          <p className="page-sub">{tr('logsHint')}</p>
        </div>
        <div className="row-actions">
          <span className={`badge ${running ? 'on' : 'off'}`}>
            {running ? tr('running') : tr('stopped')}
          </span>
          <button className="btn secondary" type="button" onClick={() => { void disableInvalid() }}>
            {tr('disableInvalidInbounds')}
          </button>
          <button className="btn secondary" type="button" onClick={() => void restart()}>{tr('restart')}</button>
        </div>
      </div>

      {issues.length > 0 ? (
        <div className="card" style={{ marginBottom: 12 }}>
          <div className="label">{tr('configIssues')}</div>
          <ul style={{ margin: '8px 0 0', paddingLeft: 18 }}>
            {issues.map((iss, i) => (
              <li key={`${iss.tag || 'issue'}-${i}`} style={{ marginBottom: 4 }}>
                <code style={{ fontFamily: 'var(--mono)' }}>{iss.tag || '—'}</code>
                {' — '}
                <span className="page-sub" style={{ margin: 0 }}>{iss.reason}</span>
              </li>
            ))}
          </ul>
        </div>
      ) : (
        <p className="page-sub" style={{ marginTop: 0 }}>{tr('noConfigIssues')}</p>
      )}

      <div className="tabs" style={{ display: 'flex', flexWrap: 'wrap', gap: 6, marginBottom: 12 }}>
        {tabs.map((t) => (
          <button
            key={t.id}
            type="button"
            className={`tab ${tab === t.id ? 'active' : ''}`}
            onClick={() => setTab(t.id)}
          >
            {t.label}
          </button>
        ))}
      </div>

      {tab !== 'config' && (
        <div className="row-actions" style={{ marginBottom: 12, flexWrap: 'wrap' }}>
          <button className="btn secondary" type="button" disabled={busy} onClick={() => void loadLogs()}>
            {tr('refresh')}
          </button>
          <button className="btn secondary" type="button" onClick={downloadLogs}>
            {tr('download')}
          </button>
          <label className="label" style={{ display: 'flex', alignItems: 'center', gap: 8, margin: 0 }}>
            <input type="checkbox" checked={autoRefresh} onChange={(e) => setAutoRefresh(e.target.checked)} />
            {tr('autoRefresh')} (5s)
          </label>
          <label className="label" style={{ display: 'flex', alignItems: 'center', gap: 8, margin: 0 }}>
            lines
            <select className="select" style={{ width: 'auto' }} value={lines} onChange={(e) => setLines(Number(e.target.value))}>
              {LINE_OPTS.map((n) => <option key={n} value={n}>{n}</option>)}
            </select>
          </label>
          {path && <span className="page-sub" style={{ margin: 0 }}>{path}</span>}
        </div>
      )}

      {tab === 'config' && (
        <div className="row-actions" style={{ marginBottom: 12 }}>
          <button className="btn secondary" type="button" disabled={busy} onClick={() => void loadConfig()}>
            {tr('refresh')}
          </button>
          <button className="btn secondary" type="button" onClick={downloadLogs}>
            {tr('download')}
          </button>
        </div>
      )}

      {tab === 'config' ? (
        <pre className="log-view">{configJson}</pre>
      ) : colorize ? (
        <div className="log-view log-view-colored">{renderedLines}</div>
      ) : (
        <pre className="log-view">{logLines.join('\n')}</pre>
      )}
      {msg && <p className="page-sub" style={{ marginTop: 10 }}>{msg}</p>}

      <style>{`
        .tab {
          border: 1px solid var(--border);
          background: transparent;
          color: var(--text-muted);
          border-radius: 999px;
          padding: 0.4rem 0.85rem;
          cursor: pointer;
          font-weight: 600;
          font-size: 0.85rem;
        }
        .tab.active {
          background: var(--accent-soft);
          color: var(--accent);
          border-color: transparent;
        }
        .log-view {
          margin: 0;
          padding: 1rem 1.1rem;
          min-height: 360px;
          max-height: min(70vh, 720px);
          overflow: auto;
          border: 1px solid var(--border);
          border-radius: 14px;
          background: color-mix(in srgb, var(--bg-sidebar) 85%, transparent);
          font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
          font-size: 0.78rem;
          line-height: 1.45;
          white-space: pre-wrap;
          word-break: break-word;
        }
        .log-view-colored { white-space: pre; }
        .log-view-colored > div { white-space: pre-wrap; word-break: break-word; }
        .log-error { color: var(--danger); }
        .log-warn { color: var(--warn); }
        .log-info { color: var(--accent); }
      `}</style>
    </div>
  )
}
