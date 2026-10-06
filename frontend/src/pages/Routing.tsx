import { useEffect, useState } from 'react'
import { api, type Inbound, type Outbound } from '../api'
import { useApp } from '../AppContext'
import { RoutingFormModal, type RoutingRule } from '../components/RoutingFormModal'

export function RoutingPage() {
  const { tr } = useApp()
  const [rows, setRows] = useState<RoutingRule[]>([])
  const [inbounds, setInbounds] = useState<Inbound[]>([])
  const [outbounds, setOutbounds] = useState<Outbound[]>([])
  const [modal, setModal] = useState<{ open: boolean; mode: 'add' | 'edit'; rule: RoutingRule | null }>({
    open: false, mode: 'add', rule: null,
  })

  async function load() {
    const [r, ib, ob] = await Promise.all([
      api<RoutingRule[]>('/routing'),
      api<Inbound[]>('/inbounds'),
      api<Outbound[]>('/outbounds'),
    ])
    setRows(r)
    setInbounds(ib)
    setOutbounds(ob)
  }

  useEffect(() => { load().catch(console.error) }, [])

  async function remove(id: number) {
    if (!confirm('Delete rule?')) return
    await api(`/routing/${id}`, { method: 'DELETE' })
    await load()
  }

  return (
    <div>
      <div className="page-head">
        <div>
          <h1 className="page-title">{tr('routing')}</h1>
          <p className="page-sub">{tr('routingHint')}</p>
        </div>
        <button className="btn" onClick={() => setModal({ open: true, mode: 'add', rule: null })}>{tr('create')}</button>
      </div>
      <div className="card">
        <table className="table">
          <thead>
            <tr>
              <th>Priority</th>
              <th>{tr('remark')}</th>
              <th>Inbound</th>
              <th>Outbound</th>
              <th>Match</th>
              <th>{tr('status')}</th>
              <th>{tr('actions')}</th>
            </tr>
          </thead>
          <tbody>
            {rows.length === 0 && <tr><td colSpan={7}>{tr('empty')}</td></tr>}
            {rows.map((r) => (
              <tr key={r.id}>
                <td>{r.priority}</td>
                <td>{r.remark || '—'}</td>
                <td>{r.inboundTag || '*'}</td>
                <td><code style={{ fontFamily: 'var(--mono)' }}>{r.outboundTag}</code></td>
                <td>{[r.domain, r.ip, r.port, r.network, r.protocol].filter(Boolean).join(' / ') || '—'}</td>
                <td><span className={`badge ${r.enable ? 'on' : 'off'}`}>{r.enable ? tr('enable') : tr('disable')}</span></td>
                <td className="row-actions">
                  <button className="btn secondary" onClick={() => setModal({ open: true, mode: 'edit', rule: r })}>{tr('edit')}</button>
                  <button className="btn danger" onClick={() => remove(r.id)}>{tr('delete')}</button>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>

      <RoutingFormModal
        open={modal.open}
        mode={modal.mode}
        rule={modal.rule}
        inbounds={inbounds}
        outbounds={outbounds}
        onClose={() => setModal({ open: false, mode: 'add', rule: null })}
        onSaved={() => { void load() }}
      />
    </div>
  )
}
