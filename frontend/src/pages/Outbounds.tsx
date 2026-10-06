import { FormEvent, useEffect, useState } from 'react'
import { api, type Outbound } from '../api'
import { useApp } from '../AppContext'

export function OutboundsPage() {
  const { tr } = useApp()
  const [rows, setRows] = useState<Outbound[]>([])
  const [open, setOpen] = useState(false)
  const [form, setForm] = useState({ tag: '', protocol: 'freedom', settings: '{}', streamSettings: '{}', enable: true, remark: '' })

  async function load() { setRows(await api<Outbound[]>('/outbounds')) }
  useEffect(() => { load().catch(console.error) }, [])

  async function create(e: FormEvent) {
    e.preventDefault()
    await api('/outbounds', { method: 'POST', body: JSON.stringify(form) })
    setOpen(false)
    await load()
  }

  async function remove(id: number) {
    await api(`/outbounds/${id}`, { method: 'DELETE' })
    await load()
  }

  return (
    <div>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
        <div>
          <h1 className="page-title">{tr('outbounds')}</h1>
          <p className="page-sub">Freedom / blackhole / proxy outbounds</p>
        </div>
        <button className="btn" onClick={() => setOpen(true)}>{tr('create')}</button>
      </div>
      <div className="card">
        <table className="table">
          <thead>
            <tr><th>Tag</th><th>Protocol</th><th>Remark</th><th>{tr('actions')}</th></tr>
          </thead>
          <tbody>
            {rows.map((o) => (
              <tr key={o.id}>
                <td>{o.tag}</td>
                <td>{o.protocol}</td>
                <td>{o.remark}</td>
                <td>
                  {!['direct', 'blocked'].includes(o.tag) && (
                    <button className="btn danger" onClick={() => remove(o.id)}>{tr('delete')}</button>
                  )}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
      {open && (
        <div className="modal-backdrop" onClick={() => setOpen(false)}>
          <form className="modal" onClick={(e) => e.stopPropagation()} onSubmit={create}>
            <h3>{tr('create')} outbound</h3>
            <div className="field"><label className="label">Tag</label><input className="input" value={form.tag} onChange={(e) => setForm({ ...form, tag: e.target.value })} required /></div>
            <div className="field">
              <label className="label">Protocol</label>
              <select className="select" value={form.protocol} onChange={(e) => setForm({ ...form, protocol: e.target.value })}>
                <option value="freedom">freedom</option>
                <option value="blackhole">blackhole</option>
                <option value="vless">vless</option>
                <option value="vmess">vmess</option>
                <option value="trojan">trojan</option>
                <option value="shadowsocks">shadowsocks</option>
                <option value="socks">socks</option>
                <option value="http">http</option>
              </select>
            </div>
            <div className="field"><label className="label">Settings</label><textarea className="textarea" value={form.settings} onChange={(e) => setForm({ ...form, settings: e.target.value })} /></div>
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
