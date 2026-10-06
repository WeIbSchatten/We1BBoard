import { useEffect, useMemo, useState } from 'react'
import { api, type Client, type GroupSummary, type Inbound, type LimitWarning, type OnlineClients } from '../api'
import { useApp } from '../AppContext'
import { ClientBulkAddModal } from '../components/ClientBulkAddModal'
import { ClientBulkAdjustModal } from '../components/ClientBulkAdjustModal'
import { ClientBulkAttachModal } from '../components/ClientBulkAttachModal'
import { ClientBulkDetachModal } from '../components/ClientBulkDetachModal'
import { ClientFormModal } from '../components/ClientFormModal'
import { ClientInfoModal } from '../components/ClientInfoModal'
import { ConfirmModal } from '../components/ConfirmModal'
import { TrafficBar } from '../components/TrafficBar'
import { inboundSupportsClients } from '../lib/inboundForm'
import type { DictKey } from '../i18n'

type ClientRow = Client & { inboundRemark?: string; inboundProtocol?: string; inboundPort?: number; inboundCount?: number }

type StatusFilter = 'all' | 'enabled' | 'disabled' | 'expired' | 'depleted'

type ConfirmState =
  | { kind: 'delete'; id: number }
  | { kind: 'kick'; id: number }

const STATUS_CHIPS: { id: StatusFilter; labelKey: DictKey }[] = [
  { id: 'all', labelKey: 'filterAll' },
  { id: 'enabled', labelKey: 'filterEnabled' },
  { id: 'disabled', labelKey: 'filterDisabled' },
  { id: 'expired', labelKey: 'filterExpired' },
  { id: 'depleted', labelKey: 'filterDepleted' },
]

function isExpired(c: Client) {
  return (c.expiryTime || 0) > 0 && c.expiryTime < Date.now()
}

function isDepleted(c: Client) {
  return (c.totalGB || 0) > 0 && (c.up || 0) + (c.down || 0) >= c.totalGB * 1e9
}

function usedGB(c: Client) {
  return ((c.up || 0) + (c.down || 0)) / (1024 * 1024 * 1024)
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
  const [detachOpen, setDetachOpen] = useState(false)
  const [adjustOpen, setAdjustOpen] = useState(false)
  const [onlineMap, setOnlineMap] = useState<Record<string, number>>({})
  const [limitWarnings, setLimitWarnings] = useState<LimitWarning[]>([])
  const [confirm, setConfirm] = useState<ConfirmState | null>(null)

  async function load() {
    const [ib, g] = await Promise.all([
      api<Inbound[]>('/inbounds'),
      api<GroupSummary[]>('/clients/groups').catch(() => [] as GroupSummary[]),
    ])
    setInbounds(ib)
    setGroups(g || [])
  }

  async function loadOnlines() {
    try {
      const data = await api<OnlineClients>('/clients/onlines')
      setOnlineMap(data?.map || {})
    } catch { /* ignore */ }
  }

  async function loadLimitWarnings() {
    try {
      const data = await api<LimitWarning[]>('/clients/limit-warnings')
      setLimitWarnings(Array.isArray(data) ? data : [])
    } catch { /* ignore */ }
  }

  useEffect(() => { load().catch(console.error) }, [])
  useEffect(() => {
    void loadOnlines()
    void loadLimitWarnings()
    const t = setInterval(() => {
      void loadOnlines()
      void loadLimitWarnings()
    }, 15000)
    return () => clearInterval(t)
  }, [])

  const emailToId = useMemo(() => {
    const m = new Map<string, number>()
    for (const ib of inbounds) {
      for (const c of ib.clients || []) {
        if (c.email && !m.has(c.email)) m.set(c.email, c.id)
      }
    }
    return m
  }, [inbounds])

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
    await api(`/clients/${id}`, { method: 'DELETE' })
    await load()
  }

  async function resetTraffic(id: number) {
    await api(`/clients/${id}/reset-traffic`, { method: 'POST' })
    await load()
  }

  async function kickClient(id: number) {
    await api(`/clients/${id}/kick`, { method: 'POST' })
    await loadLimitWarnings()
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

  return (
    <div>
      <div className="page-head">
        <div>
          <h1 className="page-title">{tr('clients')}</h1>
          <p className="page-sub">{tr('clientsHint')}</p>
        </div>
        <div className="toolbar">
          <button className="btn secondary btn-sm" onClick={() => { void bulkAddToGroup() }} disabled={rows.length === 0}>{tr('addToGroup')}</button>
          <button className="btn secondary btn-sm" onClick={() => setAttachOpen(true)} disabled={rows.length === 0}>{tr('bulkAttach')}</button>
          <button className="btn secondary btn-sm" onClick={() => setDetachOpen(true)} disabled={rows.length === 0}>{tr('bulkDetach')}</button>
          <button className="btn secondary btn-sm" onClick={() => setAdjustOpen(true)} disabled={rows.length === 0}>{tr('bulkAdjust')}</button>
          <button className="btn secondary btn-sm" onClick={() => setBulkOpen(true)} disabled={clientInbounds.length === 0}>{tr('bulkAdd')}</button>
          <button
            className="btn btn-sm"
            onClick={() => setModal({ open: true, mode: 'add', inbound: null, client: null })}
            disabled={clientInbounds.length === 0}
          >
            {tr('create')}
          </button>
        </div>
      </div>

      {limitWarnings.length > 0 && (
        <div className="alert danger">
          <strong>{tr('limitIpWarnings')}</strong>
          <ul style={{ margin: '8px 0 0', paddingLeft: 18 }}>
            {limitWarnings.map((w) => {
              const id = emailToId.get(w.email)
              return (
                <li key={w.email} style={{ marginBottom: 6 }}>
                  <code>{w.email}</code>: {w.ips.length}/{w.limit} IPs
                  {' '}({w.ips.join(', ')})
                  {id ? (
                    <button
                      type="button"
                      className="btn btn-sm secondary"
                      style={{ marginLeft: 8 }}
                      onClick={() => setConfirm({ kind: 'kick', id })}
                    >
                      {tr('clearIpsWarn')}
                    </button>
                  ) : null}
                </li>
              )
            })}
          </ul>
        </div>
      )}

      <div className="card" style={{ marginBottom: 12 }}>
        <div className="toolbar">
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
        </div>
        <div className="chip-row">
          {STATUS_CHIPS.map((chip) => (
            <button
              key={chip.id}
              type="button"
              className={`chip${statusFilter === chip.id ? ' active' : ''}`}
              onClick={() => setStatusFilter(chip.id)}
            >
              {tr(chip.labelKey)}
            </button>
          ))}
        </div>
      </div>

      {rows.length === 0 ? (
        <div className="card">
          <div className="empty-state">
            <div>{tr('empty')}</div>
            <button
              type="button"
              className="btn"
              disabled={clientInbounds.length === 0}
              onClick={() => setModal({ open: true, mode: 'add', inbound: null, client: null })}
            >
              {tr('create')}
            </button>
          </div>
        </div>
      ) : (
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
              {rows.map((c) => (
                <tr key={c.id}>
                  <td>
                    <input
                      type="checkbox"
                      checked={selected.includes(c.id)}
                      onChange={(e) => setSelected((prev) => e.target.checked ? [...prev, c.id] : prev.filter((x) => x !== c.id))}
                    />
                  </td>
                  <td>
                    {c.email}{' '}
                    {onlineMap[c.email] ? <span className="badge online">{tr('online')}</span> : null}
                  </td>
                  <td>{c.group ? <span className="tag">{c.group}</span> : '—'}</td>
                  <td>
                    <span className="badge">{c.inboundProtocol}</span>{' '}
                    {c.inboundRemark}:{c.inboundPort}
                    {(c.inboundCount || 1) > 1 && <span className="badge" style={{ marginLeft: 4 }}>+{c.inboundCount! - 1}</span>}
                  </td>
                  <td><code style={{ fontFamily: 'var(--mono)', fontSize: '0.8rem' }}>{c.uuid?.slice(0, 8)}…</code></td>
                  <td><TrafficBar used={usedGB(c)} totalGB={c.totalGB || 0} /></td>
                  <td><span className={`badge ${c.enable ? 'on' : 'off'}`}>{c.enable ? tr('enable') : tr('disable')}</span></td>
                  <td className="row-actions">
                    <button className="btn btn-sm secondary" onClick={() => { setInfoTab('links'); setInfoClient(c) }}>{tr('link')}</button>
                    <button className="btn btn-sm secondary" onClick={() => { setInfoTab('sub'); setInfoClient(c) }}>{tr('subscription')}</button>
                    <button className="btn btn-sm secondary" onClick={() => { void resetTraffic(c.id) }}>{tr('resetTraffic')}</button>
                    {(c.limitIp || 0) > 0 && (
                      <button className="btn btn-sm secondary" onClick={() => setConfirm({ kind: 'kick', id: c.id })}>{tr('clearIpsWarn')}</button>
                    )}
                    <button className="btn btn-sm secondary" onClick={() => setModal({ open: true, mode: 'edit', inbound: null, client: c })}>{tr('edit')}</button>
                    <button className="btn btn-sm danger" onClick={() => setConfirm({ kind: 'delete', id: c.id })}>{tr('delete')}</button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      <ConfirmModal
        open={!!confirm}
        title={confirm?.kind === 'kick' ? tr('clearIpsWarn') : tr('confirmDeleteTitle')}
        message={confirm?.kind === 'kick' ? `${tr('clearIpsWarn')}?` : tr('confirmDeleteClient')}
        confirmLabel={confirm?.kind === 'kick' ? tr('clearIpsWarn') : tr('delete')}
        danger={confirm?.kind === 'delete'}
        onCancel={() => setConfirm(null)}
        onConfirm={() => {
          const c = confirm
          setConfirm(null)
          if (!c) return
          if (c.kind === 'kick') void kickClient(c.id)
          else void remove(c.id)
        }}
      />

      <ClientInfoModal
        open={!!infoClient}
        client={infoClient}
        inbounds={inbounds}
        online={!!(infoClient && onlineMap[infoClient.email])}
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
      <ClientBulkDetachModal
        open={detachOpen}
        clients={rows}
        inbounds={clientInbounds}
        onClose={() => setDetachOpen(false)}
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
