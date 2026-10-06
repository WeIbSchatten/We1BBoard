import { useEffect, useState } from 'react'
import { api, type Outbound } from '../api'
import { useApp } from '../AppContext'
import { OutboundFormModal } from '../components/OutboundFormModal'

export function OutboundsPage() {
  const { tr } = useApp()
  const [rows, setRows] = useState<Outbound[]>([])
  const [modal, setModal] = useState<{ open: boolean; mode: 'add' | 'edit'; outbound: Outbound | null }>({
    open: false, mode: 'add', outbound: null,
  })

  async function load() { setRows(await api<Outbound[]>('/outbounds')) }
  useEffect(() => { load().catch(console.error) }, [])

  async function remove(id: number) {
    if (!confirm('Delete outbound?')) return
    await api(`/outbounds/${id}`, { method: 'DELETE' })
    await load()
  }

  return (
    <div>
      <div className="page-head">
        <div>
          <h1 className="page-title">{tr('outbounds')}</h1>
          <p className="page-sub">{tr('outboundsHint')}</p>
        </div>
        <button className="btn" onClick={() => setModal({ open: true, mode: 'add', outbound: null })}>{tr('create')}</button>
      </div>
      <div className="card">
        <table className="table">
          <thead>
            <tr>
              <th>Tag</th>
              <th>{tr('protocol')}</th>
              <th>{tr('remark')}</th>
              <th>{tr('status')}</th>
              <th>{tr('actions')}</th>
            </tr>
          </thead>
          <tbody>
            {rows.length === 0 && <tr><td colSpan={5}>{tr('empty')}</td></tr>}
            {rows.map((o) => (
              <tr key={o.id}>
                <td><code style={{ fontFamily: 'var(--mono)' }}>{o.tag}</code></td>
                <td><span className="badge">{o.protocol}</span></td>
                <td>{o.remark || '—'}</td>
                <td><span className={`badge ${o.enable ? 'on' : 'off'}`}>{o.enable ? tr('enable') : tr('disable')}</span></td>
                <td className="row-actions">
                  <button className="btn secondary" onClick={() => setModal({ open: true, mode: 'edit', outbound: o })}>{tr('edit')}</button>
                  {!['direct', 'blocked'].includes(o.tag) && (
                    <button className="btn danger" onClick={() => remove(o.id)}>{tr('delete')}</button>
                  )}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>

      <OutboundFormModal
        open={modal.open}
        mode={modal.mode}
        outbound={modal.outbound}
        onClose={() => setModal({ open: false, mode: 'add', outbound: null })}
        onSaved={() => { void load() }}
      />
    </div>
  )
}
