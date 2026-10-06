import { useEffect, useState } from 'react'
import { api, type Inbound, type Outbound } from '../api'
import { useApp } from '../AppContext'
import { RoutingFormModal, type RoutingRule } from '../components/RoutingFormModal'

const DOMAIN_STRATEGIES = ['AsIs', 'IPIfNonMatch', 'IPOnDemand'] as const

export function RoutingPage() {
  const { tr } = useApp()
  const [rows, setRows] = useState<RoutingRule[]>([])
  const [inbounds, setInbounds] = useState<Inbound[]>([])
  const [outbounds, setOutbounds] = useState<Outbound[]>([])
  const [domainStrategy, setDomainStrategy] = useState<string>('AsIs')
  const [savingStrategy, setSavingStrategy] = useState(false)
  const [modal, setModal] = useState<{ open: boolean; mode: 'add' | 'edit'; rule: RoutingRule | null }>({
    open: false, mode: 'add', rule: null,
  })

  async function load() {
    const [r, ib, ob, settings] = await Promise.all([
      api<RoutingRule[]>('/routing'),
      api<Inbound[]>('/inbounds'),
      api<Outbound[]>('/outbounds'),
      api<Record<string, string>>('/settings'),
    ])
    setRows(r)
    setInbounds(ib)
    setOutbounds(ob)
    const ds = settings.routingDomainStrategy || 'AsIs'
    setDomainStrategy(DOMAIN_STRATEGIES.includes(ds as typeof DOMAIN_STRATEGIES[number]) ? ds : 'AsIs')
  }

  useEffect(() => { load().catch(console.error) }, [])

  async function saveDomainStrategy(value: string) {
    setDomainStrategy(value)
    setSavingStrategy(true)
    try {
      await api('/settings', { method: 'POST', body: JSON.stringify({ routingDomainStrategy: value }) })
      await api('/xray/restart', { method: 'POST' })
    } catch (e) {
      console.error(e)
    } finally {
      setSavingStrategy(false)
    }
  }

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

      <div className="card" style={{ marginBottom: 12, display: 'flex', alignItems: 'center', gap: 12, flexWrap: 'wrap' }}>
        <label className="label" style={{ margin: 0 }}>{tr('domainStrategy')}</label>
        <select
          className="select"
          style={{ width: 'auto', minWidth: 160 }}
          value={domainStrategy}
          disabled={savingStrategy}
          onChange={(e) => { void saveDomainStrategy(e.target.value) }}
        >
          {DOMAIN_STRATEGIES.map((s) => <option key={s} value={s}>{s}</option>)}
        </select>
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
