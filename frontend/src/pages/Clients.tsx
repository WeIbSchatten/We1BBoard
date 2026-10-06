import { useEffect, useMemo, useState } from 'react'
import { api, type Client, type Inbound } from '../api'
import { useApp } from '../AppContext'
import { ClientBulkAddModal } from '../components/ClientBulkAddModal'
import { ClientBulkAdjustModal } from '../components/ClientBulkAdjustModal'
import { ClientBulkAttachModal } from '../components/ClientBulkAttachModal'
import { ClientFormModal } from '../components/ClientFormModal'
import { inboundSupportsClients } from '../lib/inboundForm'

type ClientRow = Client & { inboundRemark?: string; inboundProtocol?: string; inboundPort?: number; inboundCount?: number }

export function ClientsPage() {
  const { tr } = useApp()
  const [inbounds, setInbounds] = useState<Inbound[]>([])
  const [filter, setFilter] = useState('')
  const [modal, setModal] = useState<{ open: boolean; mode: 'add' | 'edit'; inbound: Inbound | null; client: Client | null }>({
    open: false, mode: 'add', inbound: null, client: null,
  })
  const [bulkOpen, setBulkOpen] = useState(false)
  const [attachOpen, setAttachOpen] = useState(false)
  const [adjustOpen, setAdjustOpen] = useState(false)
  const [link, setLink] = useState('')
  const [subUrls, setSubUrls] = useState<Record<string, string> | null>(null)
  const [qrClientId, setQrClientId] = useState<number | null>(null)

  async function load() {
    setInbounds(await api<Inbound[]>('/inbounds'))
  }

  useEffect(() => { load().catch(console.error) }, [])

  const clientInbounds = useMemo(
    () => inbounds.filter((i) => inboundSupportsClients(i.protocol)),
    [inbounds],
  )

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
    const list = [...byId.values()]
    const q = filter.trim().toLowerCase()
    if (!q) return list
    return list.filter((c) =>
      [c.email, c.uuid, c.subId, c.comment, c.inboundRemark, c.inboundProtocol, c.inboundIds]
        .filter(Boolean)
        .some((v) => String(v).toLowerCase().includes(q)),
    )
  }, [inbounds, filter])

  async function remove(id: number) {
    if (!confirm('Delete client?')) return
    await api(`/clients/${id}`, { method: 'DELETE' })
    await load()
  }

  async function resetTraffic(id: number) {
    await api(`/clients/${id}/reset-traffic`, { method: 'POST' })
    await load()
  }

  async function showLink(c: Client) {
    const data = await api<{ link: string }>(`/clients/${c.id}/link`)
    setLink(data.link)
    setSubUrls(null)
    setQrClientId(c.id)
  }

  async function showSub(c: Client) {
    const data = await api<{ urls: Record<string, string> }>(`/clients/${c.id}/sub`)
    setSubUrls(data.urls)
    setLink('')
    setQrClientId(null)
  }

  const qrSrc = qrClientId
    ? `${window.location.pathname.includes('/we1b') ? window.location.pathname.slice(0, window.location.pathname.indexOf('/we1b') + 5) : '/we1b'}/api/clients/${qrClientId}/qr`
    : ''

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

      <div className="card" style={{ marginBottom: 12 }}>
        <input
          className="input"
          placeholder="Search email / uuid / subId / inbound…"
          value={filter}
          onChange={(e) => setFilter(e.target.value)}
        />
      </div>

      <div className="card">
        <table className="table">
          <thead>
            <tr>
              <th>Email</th>
              <th>Inbound</th>
              <th>UUID</th>
              <th>Traffic</th>
              <th>{tr('status')}</th>
              <th>{tr('actions')}</th>
            </tr>
          </thead>
          <tbody>
            {rows.length === 0 && <tr><td colSpan={6}>{tr('empty')}</td></tr>}
            {rows.map((c) => (
                <tr key={c.id}>
                  <td>{c.email}</td>
                  <td>
                    <span className="badge">{c.inboundProtocol}</span>{' '}
                    {c.inboundRemark}:{c.inboundPort}
                    {(c.inboundCount || 1) > 1 && <span className="badge" style={{ marginLeft: 4 }}>+{c.inboundCount! - 1}</span>}
                  </td>
                  <td><code style={{ fontFamily: 'var(--mono)', fontSize: '0.8rem' }}>{c.uuid?.slice(0, 8)}…</code></td>
                  <td>{traffic(c)}</td>
                  <td><span className={`badge ${c.enable ? 'on' : 'off'}`}>{c.enable ? tr('enable') : tr('disable')}</span></td>
                  <td className="row-actions">
                    <button className="btn secondary" onClick={() => showLink(c)}>{tr('link')}</button>
                    <button className="btn secondary" onClick={() => showSub(c)}>{tr('subscription')}</button>
                    <button className="btn secondary" onClick={() => { void resetTraffic(c.id) }}>{tr('resetTraffic')}</button>
                    <button className="btn secondary" onClick={() => setModal({ open: true, mode: 'edit', inbound: null, client: c })}>{tr('edit')}</button>
                    <button className="btn danger" onClick={() => remove(c.id)}>{tr('delete')}</button>
                  </td>
                </tr>
              ))}
          </tbody>
        </table>
      </div>

      {link && (
        <div className="card" style={{ marginTop: 12 }}>
          <div className="label">{tr('link')}</div>
          <textarea className="textarea" readOnly value={link} onFocus={(e) => e.target.select()} />
          {qrSrc && <img src={qrSrc} alt="qr" style={{ marginTop: 12, width: 180, height: 180, background: '#fff', padding: 8, borderRadius: 8 }} />}
        </div>
      )}

      {subUrls && (
        <div className="card" style={{ marginTop: 12 }}>
          <div className="label">{tr('subscription')}</div>
          {Object.entries(subUrls).map(([k, v]) => (
            <div className="field" key={k}>
              <label className="label">{k}</label>
              <input className="input" readOnly value={v} onFocus={(e) => e.target.select()} />
            </div>
          ))}
        </div>
      )}

      <ClientFormModal
        open={modal.open}
        mode={modal.mode}
        inbound={modal.inbound}
        inbounds={clientInbounds}
        client={modal.client}
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
