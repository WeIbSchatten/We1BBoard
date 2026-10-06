import { FormEvent, useEffect, useState } from 'react'
import { api, type TgProxy } from '../api'
import { useApp } from '../AppContext'
import { ConfirmModal } from '../components/ConfirmModal'

export function TgProxyPage() {
  const { tr } = useApp()
  const [rows, setRows] = useState<TgProxy[]>([])
  const [open, setOpen] = useState(false)
  const [form, setForm] = useState({
    name: '',
    enable: true,
    hostname: '',
    listen: '127.0.0.1:8080',
    mtproxyAddr: '127.0.0.1:2398',
    secret: '',
    carrierMode: 'websocket',
    publicMode: 'static',
    publicSiteDir: '',
    remark: '',
  })
  const [msg, setMsg] = useState('')
  const [confirmId, setConfirmId] = useState<number | null>(null)

  async function load() { setRows(await api<TgProxy[]>('/tgproxy')) }
  useEffect(() => { load().catch(console.error) }, [])

  async function create(e: FormEvent) {
    e.preventDefault()
    await api('/tgproxy', { method: 'POST', body: JSON.stringify(form) })
    setOpen(false)
    await load()
  }

  async function start(id: number) {
    setMsg('')
    try {
      await api(`/tgproxy/${id}/start`, { method: 'POST' })
      setMsg('started')
    } catch (e) {
      setMsg(e instanceof Error ? e.message : 'error')
    }
  }

  async function stop(id: number) {
    await api(`/tgproxy/${id}/stop`, { method: 'POST' })
    setMsg('stopped')
  }

  async function remove(id: number) {
    await api(`/tgproxy/${id}`, { method: 'DELETE' })
    await load()
  }

  return (
    <div>
      <div className="page-head">
        <div>
          <h1 className="page-title">{tr('tgproxy')}</h1>
          <p className="page-sub">{tr('tgproxyHint')}</p>
        </div>
        <button className="btn" onClick={() => setOpen(true)}>{tr('create')}</button>
      </div>
      {msg && <div className="alert success">{msg}</div>}
      <div className="card">
        <table className="table">
          <thead>
            <tr><th>Name</th><th>Hostname</th><th>Carrier</th><th>MTProxy</th><th>{tr('actions')}</th></tr>
          </thead>
          <tbody>
            {rows.length === 0 && (
              <tr>
                <td colSpan={5}>
                  <div className="empty-state">{tr('empty')}</div>
                </td>
              </tr>
            )}
            {rows.map((p) => (
              <tr key={p.id}>
                <td>{p.name}</td>
                <td><code style={{ fontFamily: 'var(--mono)', fontSize: '0.85rem' }}>{p.hostname}</code></td>
                <td><span className="tag">{p.carrierMode}</span></td>
                <td><code style={{ fontFamily: 'var(--mono)', fontSize: '0.85rem' }}>{p.mtproxyAddr}</code></td>
                <td className="row-actions">
                  <button className="btn btn-sm secondary" onClick={() => start(p.id)}>{tr('start')}</button>
                  <button className="btn btn-sm secondary" onClick={() => stop(p.id)}>{tr('stop')}</button>
                  <button className="btn btn-sm danger" onClick={() => setConfirmId(p.id)}>{tr('delete')}</button>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
      {open && (
        <div className="modal-backdrop" onClick={() => setOpen(false)}>
          <form className="modal" onClick={(e) => e.stopPropagation()} onSubmit={create}>
            <h3>{tr('create')} TG WEB Proxy</h3>
            <div className="grid2">
              {([
                ['name', 'Name'],
                ['hostname', 'Hostname'],
                ['listen', 'Listen'],
                ['mtproxyAddr', 'MTProxy addr'],
                ['secret', 'Secret'],
                ['publicSiteDir', 'Public site dir'],
              ] as const).map(([k, label]) => (
                <div className="field" key={k}>
                  <label className="label">{label}</label>
                  <input className="input" value={form[k]} onChange={(e) => setForm({ ...form, [k]: e.target.value })} required={k !== 'publicSiteDir'} />
                </div>
              ))}
              <div className="field">
                <label className="label">Carrier</label>
                <select className="select" value={form.carrierMode} onChange={(e) => setForm({ ...form, carrierMode: e.target.value })}>
                  <option value="websocket">websocket</option>
                  <option value="https">https</option>
                  <option value="https_lanes">https_lanes</option>
                  <option value="ws_lanes">ws_lanes</option>
                </select>
              </div>
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
        message={tr('confirmDeleteTgProxy')}
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
