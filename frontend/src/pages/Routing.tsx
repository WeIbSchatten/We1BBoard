import { FormEvent, useEffect, useState } from 'react'
import { api } from '../api'
import { useApp } from '../AppContext'

type Rule = {
  id: number
  remark: string
  enable: boolean
  priority: number
  inboundTag: string
  outboundTag: string
  domain: string
  ip: string
  port: string
  network: string
  protocol: string
}

export function RoutingPage() {
  const { tr } = useApp()
  const [rows, setRows] = useState<Rule[]>([])
  const [open, setOpen] = useState(false)
  const [form, setForm] = useState({
    remark: '', enable: true, priority: 100,
    inboundTag: '', outboundTag: 'direct', domain: '', ip: '', port: '', network: '', protocol: '',
  })

  async function load() { setRows(await api<Rule[]>('/routing')) }
  useEffect(() => { load().catch(console.error) }, [])

  async function create(e: FormEvent) {
    e.preventDefault()
    await api('/routing', { method: 'POST', body: JSON.stringify(form) })
    setOpen(false)
    await load()
  }

  async function remove(id: number) {
    await api(`/routing/${id}`, { method: 'DELETE' })
    await load()
  }

  return (
    <div>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
        <div>
          <h1 className="page-title">{tr('routing')}</h1>
          <p className="page-sub">Правила маршрутизации Xray</p>
        </div>
        <button className="btn" onClick={() => setOpen(true)}>{tr('create')}</button>
      </div>
      <div className="card">
        <table className="table">
          <thead>
            <tr><th>Priority</th><th>Inbound</th><th>Outbound</th><th>Match</th><th>{tr('actions')}</th></tr>
          </thead>
          <tbody>
            {rows.length === 0 && <tr><td colSpan={5}>{tr('empty')}</td></tr>}
            {rows.map((r) => (
              <tr key={r.id}>
                <td>{r.priority}</td>
                <td>{r.inboundTag || '*'}</td>
                <td>{r.outboundTag}</td>
                <td>{[r.domain, r.ip, r.port, r.network, r.protocol].filter(Boolean).join(' / ') || '—'}</td>
                <td><button className="btn danger" onClick={() => remove(r.id)}>{tr('delete')}</button></td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
      {open && (
        <div className="modal-backdrop" onClick={() => setOpen(false)}>
          <form className="modal" onClick={(e) => e.stopPropagation()} onSubmit={create}>
            <h3>{tr('create')} rule</h3>
            <div className="grid2">
              <div className="field"><label className="label">Remark</label><input className="input" value={form.remark} onChange={(e) => setForm({ ...form, remark: e.target.value })} /></div>
              <div className="field"><label className="label">Priority</label><input className="input" type="number" value={form.priority} onChange={(e) => setForm({ ...form, priority: Number(e.target.value) })} /></div>
              <div className="field"><label className="label">Inbound tag</label><input className="input" value={form.inboundTag} onChange={(e) => setForm({ ...form, inboundTag: e.target.value })} /></div>
              <div className="field"><label className="label">Outbound tag</label><input className="input" value={form.outboundTag} onChange={(e) => setForm({ ...form, outboundTag: e.target.value })} required /></div>
              <div className="field"><label className="label">Domain csv</label><input className="input" value={form.domain} onChange={(e) => setForm({ ...form, domain: e.target.value })} /></div>
              <div className="field"><label className="label">IP csv</label><input className="input" value={form.ip} onChange={(e) => setForm({ ...form, ip: e.target.value })} /></div>
            </div>
            <div className="row-actions">
              <button className="btn" type="submit">{tr('save')}</button>
              <button className="btn secondary" type="button" onClick={() => setOpen(false)}>Cancel</button>
            </div>
          </form>
        </div>
      )}
    </div>
  )
}
