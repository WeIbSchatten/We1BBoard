import { FormEvent, Fragment, useEffect, useState } from 'react'
import { api, type Node } from '../api'
import { useApp } from '../AppContext'
import { ConfirmModal } from '../components/ConfirmModal'

type NodeHistory = { online: number[]; latency: number[] }

function Sparkline({ values, color }: { values: number[]; color: string }) {
  const w = 120
  const h = 28
  if (!values.length) {
    return <svg width={w} height={h} aria-hidden><text x={4} y={18} fill="currentColor" fontSize={10}>—</text></svg>
  }
  const max = Math.max(...values, 1)
  const min = Math.min(...values, 0)
  const span = Math.max(max - min, 1)
  const pts = values.map((v, i) => {
    const x = values.length === 1 ? w / 2 : (i / (values.length - 1)) * (w - 2) + 1
    const y = h - 2 - ((v - min) / span) * (h - 4)
    return `${x},${y}`
  }).join(' ')
  return (
    <svg width={w} height={h} viewBox={`0 0 ${w} ${h}`} aria-hidden style={{ display: 'block' }}>
      <polyline fill="none" stroke={color} strokeWidth="1.5" points={pts} />
    </svg>
  )
}

export function NodesPage() {
  const { tr } = useApp()
  const [rows, setRows] = useState<Node[]>([])
  const [open, setOpen] = useState(false)
  const [expanded, setExpanded] = useState<number | null>(null)
  const [hist, setHist] = useState<Record<number, NodeHistory>>({})
  const [form, setForm] = useState({ name: '', url: '', token: '', tlsMode: 'verify', region: 'eu', enable: true })
  const [confirmId, setConfirmId] = useState<number | null>(null)

  async function load() { setRows(await api<Node[]>('/nodes')) }
  useEffect(() => { load().catch(console.error) }, [])

  async function create(e: FormEvent) {
    e.preventDefault()
    await api('/nodes', { method: 'POST', body: JSON.stringify(form) })
    setOpen(false)
    await load()
  }

  async function ping() {
    setRows(await api<Node[]>('/nodes/ping', { method: 'POST' }))
    if (expanded != null) await loadHistory(expanded)
  }

  async function remove(id: number) {
    await api(`/nodes/${id}`, { method: 'DELETE' })
    await load()
  }

  async function loadHistory(id: number) {
    try {
      const h = await api<NodeHistory>(`/nodes/${id}/history`)
      setHist((prev) => ({ ...prev, [id]: h }))
    } catch { /* ignore */ }
  }

  async function toggleExpand(id: number) {
    if (expanded === id) {
      setExpanded(null)
      return
    }
    setExpanded(id)
    await loadHistory(id)
  }

  return (
    <div>
      <div className="page-head">
        <div>
          <h1 className="page-title">{tr('nodes')}</h1>
          <p className="page-sub">Remote We1BBoard instances (multi-node)</p>
        </div>
        <div className="toolbar">
          <button className="btn secondary btn-sm" onClick={ping}>Ping</button>
          <button className="btn btn-sm" onClick={() => setOpen(true)}>{tr('create')}</button>
        </div>
      </div>
      <div className="card">
        <table className="table">
          <thead>
            <tr><th>Name</th><th>URL</th><th>Region</th><th>Online</th><th>{tr('actions')}</th></tr>
          </thead>
          <tbody>
            {rows.length === 0 && <tr><td colSpan={5}>{tr('empty')}</td></tr>}
            {rows.map((n) => (
              <Fragment key={n.id}>
                <tr style={{ cursor: 'pointer' }} onClick={() => { void toggleExpand(n.id) }}>
                  <td>{n.name}</td>
                  <td>{n.url}</td>
                  <td>{n.region}</td>
                  <td><span className={`badge ${n.online ? 'on' : 'off'}`}>{n.online ? 'online' : 'offline'}</span></td>
                  <td onClick={(e) => e.stopPropagation()}>
                    <button className="btn btn-sm danger" onClick={() => setConfirmId(n.id)}>{tr('delete')}</button>
                  </td>
                </tr>
                {expanded === n.id && (
                  <tr>
                    <td colSpan={5}>
                      <div style={{ display: 'flex', gap: 24, alignItems: 'center', padding: '4px 0' }}>
                        <div>
                          <div className="label" style={{ marginBottom: 4 }}>online</div>
                          <Sparkline values={hist[n.id]?.online || []} color="var(--accent)" />
                        </div>
                        <div>
                          <div className="label" style={{ marginBottom: 4 }}>latency (ms)</div>
                          <Sparkline values={hist[n.id]?.latency || []} color="var(--warn, #f59e0b)" />
                        </div>
                      </div>
                    </td>
                  </tr>
                )}
              </Fragment>
            ))}
          </tbody>
        </table>
      </div>
      {open && (
        <div className="modal-backdrop" onClick={() => setOpen(false)}>
          <form className="modal" onClick={(e) => e.stopPropagation()} onSubmit={create}>
            <h3>{tr('create')} node</h3>
            {(['name', 'url', 'token', 'region'] as const).map((k) => (
              <div className="field" key={k}>
                <label className="label">{k}</label>
                <input className="input" value={form[k]} onChange={(e) => setForm({ ...form, [k]: e.target.value })} required />
              </div>
            ))}
            <div className="field">
              <label className="label">TLS mode</label>
              <select className="select" value={form.tlsMode} onChange={(e) => setForm({ ...form, tlsMode: e.target.value })}>
                <option value="verify">verify (recommended)</option>
                <option value="skip">skip (insecure)</option>
                <option value="pin">pin</option>
                <option value="mtls">mtls</option>
              </select>
            </div>
            <div className="modal-footer">
              <button className="btn secondary btn-sm" type="button" onClick={() => setOpen(false)}>{tr('cancel')}</button>
              <button className="btn btn-sm" type="submit">{tr('save')}</button>
            </div>
          </form>
        </div>
      )}

      <ConfirmModal
        open={confirmId != null}
        title={tr('confirmDeleteTitle')}
        message={tr('confirmDeleteNode')}
        danger
        onCancel={() => setConfirmId(null)}
        onConfirm={() => {
          const id = confirmId
          setConfirmId(null)
          if (id != null) void remove(id)
        }}
      />
    </div>
  )
}
