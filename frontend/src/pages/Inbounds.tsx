import { Fragment, useEffect, useState } from 'react'
import { api, type Client, type Inbound, type OnlineClients } from '../api'
import { useApp } from '../AppContext'
import { ClientFormModal } from '../components/ClientFormModal'
import { ClientInfoModal } from '../components/ClientInfoModal'
import { ConfirmModal } from '../components/ConfirmModal'
import { InboundFormModal } from '../components/InboundFormModal'
import { TrafficBar } from '../components/TrafficBar'
import { inboundSupportsClients } from '../lib/inboundForm'

type RateRow = { id: number; upRate: number; downRate: number }

type ConfirmKind =
  | { kind: 'inbound'; id: number }
  | { kind: 'client'; id: number }

function formatRate(bps: number): string {
  if (!bps || bps < 1) return '0 B/s'
  const units = ['B/s', 'KB/s', 'MB/s', 'GB/s']
  let v = bps
  let i = 0
  while (v >= 1024 && i < units.length - 1) {
    v /= 1024
    i++
  }
  return `${v.toFixed(v >= 10 || i === 0 ? 0 : 1)} ${units[i]}`
}

function usedGB(c: Client) {
  return ((c.up || 0) + (c.down || 0)) / (1024 * 1024 * 1024)
}

export function InboundsPage() {
  const { tr } = useApp()
  const [rows, setRows] = useState<Inbound[]>([])
  const [rates, setRates] = useState<Record<number, RateRow>>({})
  const [expanded, setExpanded] = useState<number | null>(null)
  const [inboundModal, setInboundModal] = useState<{ open: boolean; mode: 'add' | 'edit'; inbound: Inbound | null }>({
    open: false, mode: 'add', inbound: null,
  })
  const [clientModal, setClientModal] = useState<{ open: boolean; mode: 'add' | 'edit'; inbound: Inbound | null; client: Client | null }>({
    open: false, mode: 'add', inbound: null, client: null,
  })
  const [infoClient, setInfoClient] = useState<Client | null>(null)
  const [infoTab, setInfoTab] = useState<'info' | 'links' | 'sub' | 'qr'>('info')
  const [pendingClientInbound, setPendingClientInbound] = useState<Inbound | null>(null)
  const [onlineMap, setOnlineMap] = useState<Record<string, number>>({})
  const [confirm, setConfirm] = useState<ConfirmKind | null>(null)

  async function load() {
    setRows(await api<Inbound[]>('/inbounds'))
  }

  async function loadOnlines() {
    try {
      const data = await api<OnlineClients>('/clients/onlines')
      setOnlineMap(data?.map || {})
    } catch { /* ignore */ }
  }

  async function loadRates() {
    try {
      const list = await api<RateRow[]>('/inbounds/rates')
      const map: Record<number, RateRow> = {}
      for (const r of list || []) map[r.id] = r
      setRates(map)
    } catch { /* ignore */ }
  }

  useEffect(() => {
    load().catch(console.error)
    loadRates().catch(() => {})
    void loadOnlines()
    const t = setInterval(() => { void loadRates() }, 5000)
    const t2 = setInterval(() => { void loadOnlines() }, 15000)
    return () => { clearInterval(t); clearInterval(t2) }
  }, [])

  useEffect(() => {
    if (pendingClientInbound) {
      setClientModal({ open: true, mode: 'add', inbound: pendingClientInbound, client: null })
      setPendingClientInbound(null)
    }
  }, [pendingClientInbound, rows])

  async function removeInbound(id: number) {
    await api(`/inbounds/${id}`, { method: 'DELETE' })
    if (expanded === id) setExpanded(null)
    await load()
  }

  async function removeClient(id: number) {
    await api(`/clients/${id}`, { method: 'DELETE' })
    await load()
  }

  async function resetTraffic(id: number) {
    await api(`/clients/${id}/reset-traffic`, { method: 'POST' })
    await load()
  }

  async function disableInvalid() {
    const res = await api<{ count: number }>('/inbounds/disable-invalid', { method: 'POST' })
    alert(`${tr('disabledCount')}: ${res.count}`)
    await load()
  }

  async function cloneInbound(id: number) {
    const created = await api<Inbound>(`/inbounds/${id}/clone`, { method: 'POST' })
    await load()
    setExpanded(created.id)
  }

  function expiryLabel(c: Client) {
    if (!c.expiryTime) return '∞'
    return new Date(c.expiryTime).toLocaleDateString()
  }

  return (
    <div>
      <div className="page-head">
        <div>
          <h1 className="page-title">{tr('inbounds')}</h1>
          <p className="page-sub">{tr('inboundsHint')}</p>
        </div>
        <div className="toolbar">
          <button className="btn secondary btn-sm" type="button" onClick={() => { void disableInvalid() }}>
            {tr('disableInvalidInbounds')}
          </button>
          <button className="btn btn-sm" onClick={() => setInboundModal({ open: true, mode: 'add', inbound: null })}>
            {tr('create')}
          </button>
        </div>
      </div>

      <div className="card">
        <table className="table">
          <thead>
            <tr>
              <th style={{ width: 36 }} />
              <th>ID</th>
              <th>{tr('remark')}</th>
              <th>{tr('protocol')}</th>
              <th>{tr('port')}</th>
              <th>{tr('network')}</th>
              <th>{tr('security')}</th>
              <th>{tr('speed')}</th>
              <th>{tr('clients')}</th>
              <th>{tr('status')}</th>
              <th>{tr('actions')}</th>
            </tr>
          </thead>
          <tbody>
            {rows.length === 0 && (
              <tr>
                <td colSpan={11}>
                  <div className="empty-state">{tr('empty')}</div>
                </td>
              </tr>
            )}
            {rows.map((r) => {
              let net = '—'
              let sec = '—'
              try {
                const s = JSON.parse(r.streamSettings || '{}') as { network?: string; security?: string }
                net = s.network || '—'
                sec = s.security || '—'
              } catch { /* */ }
              const open = expanded === r.id
              const clients = r.clients || []
              const rate = rates[r.id]
              return (
                <Fragment key={r.id}>
                  <tr>
                    <td>
                      <button
                        type="button"
                        className="icon-btn"
                        onClick={() => setExpanded(open ? null : r.id)}
                        aria-label={open ? 'Collapse' : 'Expand'}
                      >
                        {open ? '▾' : '▸'}
                      </button>
                    </td>
                    <td>{r.id}</td>
                    <td>{r.remark || r.tag}</td>
                    <td><span className="badge">{r.protocol}</span></td>
                    <td><code style={{ fontFamily: 'var(--mono)' }}>{r.port}</code></td>
                    <td><span className="tag">{net}</span></td>
                    <td><span className="tag">{sec}</span></td>
                    <td style={{ fontFamily: 'var(--mono)', fontSize: '0.8rem', whiteSpace: 'nowrap' }}>
                      <span style={{ color: 'var(--accent)' }}>↑{formatRate(rate?.upRate || 0)}</span>
                      {' '}
                      <span style={{ color: 'var(--warn)' }}>↓{formatRate(rate?.downRate || 0)}</span>
                    </td>
                    <td>{clients.length}</td>
                    <td><span className={`badge ${r.enable ? 'on' : 'off'}`}>{r.enable ? tr('enable') : tr('disable')}</span></td>
                    <td className="row-actions">
                      <button className="btn btn-sm secondary" onClick={() => setInboundModal({ open: true, mode: 'edit', inbound: r })}>{tr('edit')}</button>
                      <button className="btn btn-sm secondary" onClick={() => { void cloneInbound(r.id) }}>{tr('clone')}</button>
                      {inboundSupportsClients(r.protocol) && (
                        <button className="btn btn-sm secondary" onClick={() => setClientModal({ open: true, mode: 'add', inbound: r, client: null })}>+ {tr('clients')}</button>
                      )}
                      <button className="btn btn-sm danger" onClick={() => setConfirm({ kind: 'inbound', id: r.id })}>{tr('delete')}</button>
                    </td>
                  </tr>
                  {open && (
                    <tr>
                      <td colSpan={11} style={{ padding: '0.5rem 0.75rem 1rem' }}>
                        {clients.length === 0 ? (
                          <div className="empty-state" style={{ padding: '1.5rem 1rem' }}>
                            <div>{tr('empty')}</div>
                            <button type="button" className="btn btn-sm secondary" onClick={() => setClientModal({ open: true, mode: 'add', inbound: r, client: null })}>
                              {tr('create')} {tr('clients')}
                            </button>
                          </div>
                        ) : (
                          <table className="table" style={{ margin: 0 }}>
                            <thead>
                              <tr>
                                <th>Email</th>
                                <th>UUID</th>
                                <th>Traffic</th>
                                <th>Expiry</th>
                                <th>{tr('status')}</th>
                                <th>{tr('actions')}</th>
                              </tr>
                            </thead>
                            <tbody>
                              {clients.map((c) => (
                                <tr key={c.id}>
                                  <td>
                                    {c.email}{' '}
                                    {onlineMap[c.email] ? <span className="badge online">{tr('online')}</span> : null}
                                  </td>
                                  <td><code style={{ fontFamily: 'var(--mono)', fontSize: '0.8rem' }}>{c.uuid?.slice(0, 8)}…</code></td>
                                  <td><TrafficBar used={usedGB(c)} totalGB={c.totalGB || 0} /></td>
                                  <td>{expiryLabel(c)}</td>
                                  <td><span className={`badge ${c.enable ? 'on' : 'off'}`}>{c.enable ? tr('enable') : tr('disable')}</span></td>
                                  <td className="row-actions">
                                    <button className="btn btn-sm secondary" onClick={() => { setInfoTab('links'); setInfoClient(c) }}>{tr('link')}</button>
                                    <button className="btn btn-sm secondary" onClick={() => { setInfoTab('sub'); setInfoClient(c) }}>{tr('subscription')}</button>
                                    <button className="btn btn-sm secondary" onClick={() => { void resetTraffic(c.id) }}>{tr('resetTraffic')}</button>
                                    <button className="btn btn-sm secondary" onClick={() => setClientModal({ open: true, mode: 'edit', inbound: r, client: c })}>{tr('edit')}</button>
                                    <button className="btn btn-sm danger" onClick={() => setConfirm({ kind: 'client', id: c.id })}>{tr('delete')}</button>
                                  </td>
                                </tr>
                              ))}
                            </tbody>
                          </table>
                        )}
                      </td>
                    </tr>
                  )}
                </Fragment>
              )
            })}
          </tbody>
        </table>
      </div>

      <ConfirmModal
        open={!!confirm}
        title={tr('confirmDeleteTitle')}
        message={confirm?.kind === 'inbound' ? tr('confirmDeleteInbound') : tr('confirmDeleteClient')}
        danger
        onCancel={() => setConfirm(null)}
        onConfirm={() => {
          const c = confirm
          setConfirm(null)
          if (!c) return
          if (c.kind === 'inbound') void removeInbound(c.id)
          else void removeClient(c.id)
        }}
      />

      <ClientInfoModal
        open={!!infoClient}
        client={infoClient}
        inbounds={rows}
        online={!!(infoClient && onlineMap[infoClient.email])}
        initialTab={infoTab}
        onClose={() => setInfoClient(null)}
        onResetTraffic={() => { void load() }}
      />

      <InboundFormModal
        open={inboundModal.open}
        mode={inboundModal.mode}
        inbound={inboundModal.inbound}
        onClose={() => setInboundModal({ open: false, mode: 'add', inbound: null })}
        onSaved={(created) => {
          void load().then(() => {
            if (inboundModal.mode === 'add' && created && inboundSupportsClients(created.protocol)) {
              setExpanded(created.id)
              setPendingClientInbound(created)
            }
          })
        }}
      />

      <ClientFormModal
        open={clientModal.open}
        mode={clientModal.mode}
        inbound={clientModal.inbound}
        inbounds={rows.filter((i) => inboundSupportsClients(i.protocol))}
        client={clientModal.client}
        onClose={() => setClientModal({ open: false, mode: 'add', inbound: null, client: null })}
        onSaved={() => { void load() }}
      />
    </div>
  )
}
