import { Fragment, useEffect, useState } from 'react'
import { api, type Client, type Inbound } from '../api'
import { useApp } from '../AppContext'
import { ClientFormModal } from '../components/ClientFormModal'
import { InboundFormModal } from '../components/InboundFormModal'

export function InboundsPage() {
  const { tr } = useApp()
  const [rows, setRows] = useState<Inbound[]>([])
  const [expanded, setExpanded] = useState<number | null>(null)
  const [inboundModal, setInboundModal] = useState<{ open: boolean; mode: 'add' | 'edit'; inbound: Inbound | null }>({
    open: false, mode: 'add', inbound: null,
  })
  const [clientModal, setClientModal] = useState<{ open: boolean; mode: 'add' | 'edit'; inbound: Inbound | null; client: Client | null }>({
    open: false, mode: 'add', inbound: null, client: null,
  })
  const [link, setLink] = useState('')
  const [subUrls, setSubUrls] = useState<Record<string, string> | null>(null)
  const [qrClientId, setQrClientId] = useState<number | null>(null)
  const [pendingClientInbound, setPendingClientInbound] = useState<Inbound | null>(null)

  async function load() {
    setRows(await api<Inbound[]>('/inbounds'))
  }

  useEffect(() => {
    load().catch(console.error)
  }, [])

  useEffect(() => {
    if (pendingClientInbound) {
      setClientModal({ open: true, mode: 'add', inbound: pendingClientInbound, client: null })
      setPendingClientInbound(null)
    }
  }, [pendingClientInbound, rows])

  async function removeInbound(id: number) {
    if (!confirm('Delete inbound?')) return
    await api(`/inbounds/${id}`, { method: 'DELETE' })
    if (expanded === id) setExpanded(null)
    await load()
  }

  async function removeClient(id: number) {
    if (!confirm('Delete client?')) return
    await api(`/clients/${id}`, { method: 'DELETE' })
    await load()
  }

  async function showLink(client?: Client) {
    if (!client) return
    const data = await api<{ link: string }>(`/clients/${client.id}/link`)
    setLink(data.link)
    setSubUrls(null)
    setQrClientId(client.id)
  }

  async function showSub(client?: Client) {
    if (!client) return
    const data = await api<{ urls: Record<string, string>; subId: string; enable: boolean }>(`/clients/${client.id}/sub`)
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
        <button className="btn" onClick={() => setInboundModal({ open: true, mode: 'add', inbound: null })}>
          {tr('create')}
        </button>
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
              <th>{tr('clients')}</th>
              <th>{tr('status')}</th>
              <th>{tr('actions')}</th>
            </tr>
          </thead>
          <tbody>
            {rows.length === 0 && (
              <tr><td colSpan={10}>{tr('empty')}</td></tr>
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
              return (
                <Fragment key={r.id}>
                  <tr>
                    <td>
                      <button
                        type="button"
                        className="btn secondary"
                        style={{ padding: '0.2rem 0.5rem' }}
                        onClick={() => setExpanded(open ? null : r.id)}
                      >
                        {open ? '▾' : '▸'}
                      </button>
                    </td>
                    <td>{r.id}</td>
                    <td>{r.remark || r.tag}</td>
                    <td><span className="badge">{r.protocol}</span></td>
                    <td><code style={{ fontFamily: 'var(--mono)' }}>{r.port}</code></td>
                    <td><span className="badge">{net}</span></td>
                    <td><span className="badge">{sec}</span></td>
                    <td>{clients.length}</td>
                    <td><span className={`badge ${r.enable ? 'on' : 'off'}`}>{r.enable ? tr('enable') : tr('disable')}</span></td>
                    <td className="row-actions">
                      <button className="btn secondary" onClick={() => setInboundModal({ open: true, mode: 'edit', inbound: r })}>{tr('edit')}</button>
                      <button className="btn secondary" onClick={() => setClientModal({ open: true, mode: 'add', inbound: r, client: null })}>+ {tr('clients')}</button>
                      <button className="btn danger" onClick={() => removeInbound(r.id)}>{tr('delete')}</button>
                    </td>
                  </tr>
                  {open && (
                    <tr>
                      <td colSpan={10} style={{ padding: '0.5rem 0.75rem 1rem' }}>
                        {clients.length === 0 ? (
                          <p className="page-sub" style={{ margin: '0.5rem 0' }}>
                            {tr('empty')} —{' '}
                            <button type="button" className="btn secondary" onClick={() => setClientModal({ open: true, mode: 'add', inbound: r, client: null })}>
                              {tr('create')} {tr('clients')}
                            </button>
                          </p>
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
                                  <td>{c.email}</td>
                                  <td><code style={{ fontFamily: 'var(--mono)', fontSize: '0.8rem' }}>{c.uuid?.slice(0, 8)}…</code></td>
                                  <td>{traffic(c)}</td>
                                  <td>{expiryLabel(c)}</td>
                                  <td><span className={`badge ${c.enable ? 'on' : 'off'}`}>{c.enable ? tr('enable') : tr('disable')}</span></td>
                                  <td className="row-actions">
                                    <button className="btn secondary" onClick={() => showLink(c)}>{tr('link')}</button>
                                    <button className="btn secondary" onClick={() => showSub(c)}>{tr('subscription')}</button>
                                    <button className="btn secondary" onClick={() => setClientModal({ open: true, mode: 'edit', inbound: r, client: c })}>{tr('edit')}</button>
                                    <button className="btn danger" onClick={() => removeClient(c.id)}>{tr('delete')}</button>
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

      <InboundFormModal
        open={inboundModal.open}
        mode={inboundModal.mode}
        inbound={inboundModal.inbound}
        onClose={() => setInboundModal({ open: false, mode: 'add', inbound: null })}
        onSaved={(created) => {
          void load().then(() => {
            if (inboundModal.mode === 'add' && created) {
              setExpanded(created.id)
              setPendingClientInbound(created)
            }
          })
        }}
      />

      {clientModal.inbound && (
        <ClientFormModal
          open={clientModal.open}
          mode={clientModal.mode}
          inbound={clientModal.inbound}
          client={clientModal.client}
          onClose={() => setClientModal({ open: false, mode: 'add', inbound: null, client: null })}
          onSaved={() => { void load() }}
        />
      )}
    </div>
  )
}
