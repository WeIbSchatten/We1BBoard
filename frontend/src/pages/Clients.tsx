import { useEffect, useMemo, useState } from 'react'
import { api, type Client, type GroupSummary, type Inbound } from '../api'
import { useApp } from '../AppContext'
import { ClientBulkAddModal } from '../components/ClientBulkAddModal'
import { ClientBulkAdjustModal } from '../components/ClientBulkAdjustModal'
import { ClientBulkAttachModal } from '../components/ClientBulkAttachModal'
import { ClientFormModal } from '../components/ClientFormModal'
import { ClientInfoModal } from '../components/ClientInfoModal'
import { inboundSupportsClients } from '../lib/inboundForm'

type ClientRow = Client & { inboundRemark?: string; inboundProtocol?: string; inboundPort?: number; inboundCount?: number }

type StatusFilter = 'all' | 'enabled' | 'disabled' | 'expired' | 'depleted'

const STATUS_CHIPS: { id: StatusFilter; label: string }[] = [
  { id: 'all', label: 'All' },
  { id: 'enabled', label: 'Enabled' },
  { id: 'disabled', label: 'Disabled' },
  { id: 'expired', label: 'Expired' },
  { id: 'depleted', label: 'Depleted' },
]

function isExpired(c: Client) {
  return (c.expiryTime || 0) > 0 && c.expiryTime < Date.now()
}

function isDepleted(c: Client) {
  return (c.totalGB || 0) > 0 && (c.up || 0) + (c.down || 0) >= c.totalGB * 1e9
}

export function ClientsPage() {
  const { tr } = useApp()
  const [inbounds, setInbounds] = useState<Inbound[]>([])
  const [groups, setGroups] = useState<GroupSummary[]>([])
  const [filter, setFilter] = useState('')
  const [groupFilter, setGroupFilter] = useState('')
  const [statusFilter, setStatusFilter] = useState<StatusFilter>('all')
  const [selected, setSelected] = useState<number[]>([])
  const [modal, setModal] = useState<{ open: boolean; mode: 'add' | 'edit'; inbound: Inbound | null; client: Client | null }>({
    open: false, mode: 'add', inbound: null, client: null,
  })
  const [infoClient, setInfoClient] = useState<Client | null>(null)
  const [infoTab, setInfoTab] = useState<'info' | 'links' | 'sub' | 'qr'>('info')
  const [bulkOpen, setBulkOpen] = useState(false)
  const [attachOpen, setAttachOpen] = useState(false)
  const [adjustOpen, setAdjustOpen] = useState(false)

  async function load() {
    const [ib, g] = await Promise.all([
      api<Inbound[]>('/inbounds'),
      api<GroupSummary[]>('/clients/groups').catch(() => [] as GroupSummary[]),
    ])
    setInbounds(ib)
    setGroups(g || [])
  }

  useEffect(() => { load().catch(console.error) }, [])

  const clientInbounds = useMemo(
    () => inbounds.filter((i) => inboundSupportsClients(i.protocol)),
    [inbounds],
  )

  const groupNames = useMemo(() => {
    const set = new Set<string>()
    for (const g of groups) if (g.name) set.add(g.name)
    for (const ib of inbounds) {
      for (const c of ib.clients || []) {
        if (c.group) set.add(c.group)
      }
    }
    return [...set].sort()
  }, [groups, inbounds])

  const rows: ClientRow[] = useMemo(() => {
    const byId = new Map<number, ClientRow>()
    for (const ib of inbounds) {
      for (const c of ib.clients || []) {
        const existing = byId.get(c.id)
        if (existing) {
          existing.inboundCount = (existing.inboundCount || 1) + 1
          continue
        }
        const primary = inbounds.find((i) => i.id === c.inboundId) || ib
        byId.set(c.id, {
          ...c,
          inboundRemark: primary.remark || primary.tag,
          inboundProtocol: primary.protocol,
          inboundPort: primary.port,
          inboundCount: (c.inboundIds || String(c.inboundId || '')).split(',').filter(Boolean).length || 1,
        })
      }
    }
    let list = [...byId.values()]
    if (groupFilter === '__none__') {
      list = list.filter((c) => !c.group)
    } else if (groupFilter) {
      list = list.filter((c) => (c.group || '') === groupFilter)
    }
    switch (statusFilter) {
      case 'enabled':
        list = list.filter((c) => c.enable)
        break
      case 'disabled':
        list = list.filter((c) => !c.enable)
        break
      case 'expired':
        list = list.filter((c) => isExpired(c))
        break
      case 'depleted':
        list = list.filter((c) => isDepleted(c))
        break
      default:
        break
    }
    const q = filter.trim().toLowerCase()
    if (!q) return list
    return list.filter((c) =>
      [c.email, c.uuid, c.subId, c.comment, c.group, c.inboundRemark, c.inboundProtocol, c.inboundIds]
        .filter(Boolean)
        .some((v) => String(v).toLowerCase().includes(q)),
    )
  }, [inbounds, filter, groupFilter, statusFilter])

  async function remove(id: number) {
    if (!confirm('Delete client?')) return
    await api(`/clients/${id}`, { method: 'DELETE' })
    await load()
  }

  async function resetTraffic(id: number) {
    await api(`/clients/${id}/reset-traffic`, { method: 'POST' })
    await load()
  }

  async function bulkAddToGroup() {
    const ids = selected.length ? selected : rows.map((r) => r.id)
    if (!ids.length) return
    const name = prompt(tr('groupName'))
    if (!name?.trim()) return
    await api('/clients/groups/assign', { method: 'POST', body: JSON.stringify({ name: name.trim(), ids }) })
    setSelected([])
    await load()
  }

  function traffic(c: Client) {
    const used = ((c.up || 0) + (c.down || 0)) / (1024 * 1024 * 1024)
    const total = c.totalGB || 0
    if (total <= 0) return `${used.toFixed(2)} GB / ∞`
    return `${used.toFixed(2)} / ${total} GB`
  }

  return (
    <div>
      <div className="page-head">
        <div>
          <h1 className="page-title">{tr('clients')}</h1>
          <p className="page-sub">{tr('clientsHint')}</p>
        </div>
        <div className="row-actions">
          <button className="btn secondary" onClick={() => { void bulkAddToGroup() }} disabled={rows.length === 0}>{tr('addToGroup')}</button>
          <button className="btn secondary" onClick={() => setAttachOpen(true)} disabled={rows.length === 0}>{tr('bulkAttach')}</button>
          <button className="btn secondary" onClick={() => setAdjustOpen(true)} disabled={rows.length === 0}>{tr('bulkAdjust')}</button>
          <button className="btn secondary" onClick={() => setBulkOpen(true)} disabled={clientInbounds.length === 0}>{tr('bulkAdd')}</button>
          <button
            className="btn"
            onClick={() => setModal({ open: true, mode: 'add', inbound: null, client: null })}
            disabled={clientInbounds.length === 0}
          >
            {tr('create')}
          </button>
        </div>
      </div>

      <div className="card" style={{ marginBottom: 12, display: 'flex', gap: 8, flexWrap: 'wrap', alignItems: 'center' }}>
        <input
          className="input"
          style={{ flex: 1, minWidth: 200 }}
          placeholder="Search email / uuid / subId / inbound…"
          value={filter}
          onChange={(e) => setFilter(e.target.value)}
        />
        <select className="select" style={{ width: 200 }} value={groupFilter} onChange={(e) => setGroupFilter(e.target.value)}>
          <option value="">{tr('filterGroup')}</option>
          <option value="__none__">(no group)</option>
          {groupNames.map((g) => (
            <option key={g} value={g}>{g}</option>
          ))}
        </select>
        <div className="chip-row" style={{ marginTop: 0, width: '100%' }}>
          {STATUS_CHIPS.map((chip) => (
            <button
              key={chip.id}
              type="button"
              className={`chip${statusFilter === chip.id ? ' active' : ''}`}
              onClick={() => setStatusFilter(chip.id)}
            >
              {chip.label}
            </button>
          ))}
        </div>
      </div>

      <div className="card">
        <table className="table">
          <thead>
            <tr>
              <th style={{ width: 36 }}>
                <input
                  type="checkbox"
                  checked={selected.length === rows.length && rows.length > 0}
                  onChange={(e) => setSelected(e.target.checked ? rows.map((r) => r.id) : [])}
                />
              </th>
              <th>Email</th>
              <th>{tr('group')}</th>
              <th>Inbound</th>
              <th>UUID</th>
              <th>Traffic</th>
              <th>{tr('status')}</th>
              <th>{tr('actions')}</th>
            </tr>
          </thead>
          <tbody>
            {rows.length === 0 && <tr><td colSpan={8}>{tr('empty')}</td></tr>}
            {rows.map((c) => (
                <tr key={c.id}>
                  <td>
                    <input
                      type="checkbox"
                      checked={selected.includes(c.id)}
                      onChange={(e) => setSelected((prev) => e.target.checked ? [...prev, c.id] : prev.filter((x) => x !== c.id))}
                    />
                  </td>
                  <td>{c.email}</td>
                  <td>{c.group || '—'}</td>
                  <td>
                    <span className="badge">{c.inboundProtocol}</span>{' '}
                    {c.inboundRemark}:{c.inboundPort}
                    {(c.inboundCount || 1) > 1 && <span className="badge" style={{ marginLeft: 4 }}>+{c.inboundCount! - 1}</span>}
                  </td>
                  <td><code style={{ fontFamily: 'var(--mono)', fontSize: '0.8rem' }}>{c.uuid?.slice(0, 8)}…</code></td>
                  <td>{traffic(c)}</td>
                  <td><span className={`badge ${c.enable ? 'on' : 'off'}`}>{c.enable ? tr('enable') : tr('disable')}</span></td>
                  <td className="row-actions">
                    <button className="btn secondary" onClick={() => { setInfoTab('links'); setInfoClient(c) }}>{tr('link')}</button>
                    <button className="btn secondary" onClick={() => { setInfoTab('sub'); setInfoClient(c) }}>{tr('subscription')}</button>
                    <button className="btn secondary" onClick={() => { void resetTraffic(c.id) }}>{tr('resetTraffic')}</button>
                    <button className="btn secondary" onClick={() => setModal({ open: true, mode: 'edit', inbound: null, client: c })}>{tr('edit')}</button>
                    <button className="btn danger" onClick={() => remove(c.id)}>{tr('delete')}</button>
                  </td>
                </tr>
              ))}
          </tbody>
        </table>
      </div>

      <ClientInfoModal
        open={!!infoClient}
        client={infoClient}
        inbounds={inbounds}
        initialTab={infoTab}
        onClose={() => setInfoClient(null)}
        onResetTraffic={() => { void load() }}
      />

      <ClientFormModal
        open={modal.open}
        mode={modal.mode}
        inbound={modal.inbound}
        inbounds={clientInbounds}
        client={modal.client}
        groupNames={groupNames}
        onClose={() => setModal({ open: false, mode: 'add', inbound: null, client: null })}
        onSaved={() => { void load() }}
      />
      <ClientBulkAddModal
        open={bulkOpen}
        inbounds={clientInbounds}
        onClose={() => setBulkOpen(false)}
        onSaved={() => { void load() }}
      />
      <ClientBulkAttachModal
        open={attachOpen}
        clients={rows}
        inbounds={clientInbounds}
        onClose={() => setAttachOpen(false)}
        onSaved={() => { void load() }}
      />
      <ClientBulkAdjustModal
        open={adjustOpen}
        clients={rows}
        onClose={() => setAdjustOpen(false)}
        onSaved={() => { void load() }}
      />
    </div>
  )
}
