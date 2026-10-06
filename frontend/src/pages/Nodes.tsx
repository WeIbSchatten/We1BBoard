import { FormEvent, useEffect, useState } from 'react'
import { api, type Node } from '../api'
import { useApp } from '../AppContext'

export function NodesPage() {
  const { tr } = useApp()
  const [rows, setRows] = useState<Node[]>([])
  const [open, setOpen] = useState(false)
  const [form, setForm] = useState({ name: '', url: '', token: '', tlsMode: 'verify', region: 'eu', enable: true })

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
  }

  async function remove(id: number) {
    await api(`/nodes/${id}`, { method: 'DELETE' })
    await load()
  }

  return (
    <div>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
        <div>
          <h1 className="page-title">{tr('nodes')}</h1>
          <p className="page-sub">Remote We1BBoard instances (multi-node)</p>
        </div>
        <div className="row-actions">
          <button className="btn secondary" onClick={ping}>Ping</button>
          <button className="btn" onClick={() => setOpen(true)}>{tr('create')}</button>
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
              <tr key={n.id}>
                <td>{n.name}</td>
                <td>{n.url}</td>
                <td>{n.region}</td>
                <td><span className={`badge ${n.online ? 'on' : 'off'}`}>{n.online ? 'online' : 'offline'}</span></td>
                <td><button className="btn danger" onClick={() => remove(n.id)}>{tr('delete')}</button></td>
              </tr>
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
