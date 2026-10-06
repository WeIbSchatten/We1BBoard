import { FormEvent, useEffect, useState } from 'react'
import { api, type Client, type Inbound } from '../api'
import { useApp } from '../AppContext'

const PROTOCOLS = [
  'vless', 'vmess', 'trojan', 'shadowsocks', 'wireguard', 'amneziawg',
  'tuic', 'hysteria2', 'mtproto', 'http', 'socks', 'tunnel', 'tun',
]

const defaultStream = JSON.stringify({ network: 'tcp', security: 'none' }, null, 2)
const realityStream = JSON.stringify({
  network: 'tcp',
  security: 'reality',
  realitySettings: {
    show: false,
    dest: 'www.cloudflare.com:443',
    xver: 0,
    serverNames: ['www.cloudflare.com'],
    privateKey: '',
    shortIds: [''],
    fingerprint: 'chrome',
  },
}, null, 2)

export function InboundsPage() {
  const { tr } = useApp()
  const [rows, setRows] = useState<Inbound[]>([])
  const [open, setOpen] = useState(false)
  const [form, setForm] = useState({
    remark: '',
    port: 443,
    protocol: 'vless',
    listen: '0.0.0.0',
    enable: true,
    settings: '{}',
    streamSettings: defaultStream,
  })
  const [error, setError] = useState('')
  const [link, setLink] = useState('')
  const [subUrls, setSubUrls] = useState<Record<string, string> | null>(null)
  const [qrClientId, setQrClientId] = useState<number | null>(null)

  async function load() {
    setRows(await api<Inbound[]>('/inbounds'))
  }

  useEffect(() => {
    load().catch(console.error)
  }, [])

  async function create(e: FormEvent) {
    e.preventDefault()
    setError('')
    try {
      const created = await api<Inbound>('/inbounds', {
        method: 'POST',
        body: JSON.stringify({
          ...form,
          settings: form.settings || '{}',
          streamSettings: form.streamSettings || defaultStream,
        }),
      })
      // auto client for proxy protocols
      if (!['tun', 'tunnel'].includes(form.protocol)) {
        await api('/clients', {
          method: 'POST',
          body: JSON.stringify({
            inboundId: created.id,
            email: `${form.protocol}-${form.port}@we1b`,
            enable: true,
            flow: form.protocol === 'vless' && form.streamSettings.includes('reality') ? 'xtls-rprx-vision' : '',
          }),
        })
      }
      setOpen(false)
      await load()
    } catch (err) {
      setError(err instanceof Error ? err.message : 'error')
    }
  }

  async function remove(id: number) {
    if (!confirm('Delete?')) return
    await api(`/inbounds/${id}`, { method: 'DELETE' })
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

  return (
    <div>
      <div style={{ display: 'flex', justifyContent: 'space-between', gap: 12, alignItems: 'center' }}>
        <div>
          <h1 className="page-title">{tr('inbounds')}</h1>
          <p className="page-sub">VLESS / VMess / Trojan / SS / WG / TUIC / Hy2 / MTProto / …</p>
        </div>
        <button className="btn" onClick={() => setOpen(true)}>{tr('create')}</button>
      </div>

      <div className="card">
        <table className="table">
          <thead>
            <tr>
              <th>ID</th>
              <th>{tr('remark')}</th>
              <th>{tr('protocol')}</th>
              <th>{tr('port')}</th>
              <th>{tr('status')}</th>
              <th>{tr('actions')}</th>
            </tr>
          </thead>
          <tbody>
            {rows.length === 0 && (
              <tr><td colSpan={6}>{tr('empty')}</td></tr>
            )}
            {rows.map((r) => (
              <tr key={r.id}>
                <td>{r.id}</td>
                <td>{r.remark || r.tag}</td>
                <td><span className="badge">{r.protocol}</span></td>
                <td>{r.port}</td>
                <td><span className={`badge ${r.enable ? 'on' : 'off'}`}>{r.enable ? tr('enable') : tr('disable')}</span></td>
                <td className="row-actions">
                  <button className="btn secondary" onClick={() => showLink(r.clients?.[0])}>{tr('link')}</button>
                  <button className="btn secondary" onClick={() => showSub(r.clients?.[0])}>{tr('subscription')}</button>
                  <button className="btn danger" onClick={() => remove(r.id)}>{tr('delete')}</button>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>

      {link && (
        <div className="card" style={{ marginTop: 12 }}>
          <div className="label">{tr('link')}</div>
          <textarea className="textarea" readOnly value={link} />
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

      {open && (
        <div className="modal-backdrop" onClick={() => setOpen(false)}>
          <form className="modal" onClick={(e) => e.stopPropagation()} onSubmit={create}>
            <h3>{tr('create')} inbound</h3>
            <div className="grid2">
              <div className="field">
                <label className="label">{tr('remark')}</label>
                <input className="input" value={form.remark} onChange={(e) => setForm({ ...form, remark: e.target.value })} />
              </div>
              <div className="field">
                <label className="label">{tr('port')}</label>
                <input className="input" type="number" value={form.port} onChange={(e) => setForm({ ...form, port: Number(e.target.value) })} />
              </div>
              <div className="field">
                <label className="label">{tr('protocol')}</label>
                <select className="select" value={form.protocol} onChange={(e) => setForm({ ...form, protocol: e.target.value })}>
                  {PROTOCOLS.map((p) => <option key={p} value={p}>{p}</option>)}
                </select>
              </div>
              <div className="field">
                <label className="label">Listen</label>
                <input className="input" value={form.listen} onChange={(e) => setForm({ ...form, listen: e.target.value })} />
              </div>
            </div>
            <div className="field">
              <label className="label">Stream settings</label>
              <div className="row-actions" style={{ marginBottom: 8 }}>
                <button type="button" className="btn secondary" onClick={() => setForm({ ...form, streamSettings: defaultStream })}>TCP none</button>
                <button type="button" className="btn secondary" onClick={() => setForm({ ...form, streamSettings: realityStream })}>REALITY</button>
                <button type="button" className="btn secondary" onClick={() => setForm({ ...form, streamSettings: JSON.stringify({ network: 'ws', security: 'tls', wsSettings: { path: '/ws' }, tlsSettings: { serverName: '' } }, null, 2) })}>WS+TLS</button>
                <button type="button" className="btn secondary" onClick={() => setForm({ ...form, streamSettings: JSON.stringify({ network: 'grpc', security: 'tls', grpcSettings: { serviceName: 'grpc' } }, null, 2) })}>gRPC</button>
                <button type="button" className="btn secondary" onClick={() => setForm({ ...form, streamSettings: JSON.stringify({ network: 'xhttp', security: 'tls', xhttpSettings: { path: '/x', mode: 'auto' } }, null, 2) })}>XHTTP</button>
                <button type="button" className="btn secondary" onClick={() => setForm({ ...form, streamSettings: JSON.stringify({ network: 'httpupgrade', security: 'tls', httpupgradeSettings: { path: '/hu' } }, null, 2) })}>HTTPUpgrade</button>
                <button type="button" className="btn secondary" onClick={() => setForm({ ...form, streamSettings: JSON.stringify({ network: 'kcp', security: 'none', kcpSettings: { mtu: 1350, seed: '' } }, null, 2) })}>mKCP</button>
              </div>
              <textarea className="textarea" value={form.streamSettings} onChange={(e) => setForm({ ...form, streamSettings: e.target.value })} />
            </div>
            <div className="field">
              <label className="label">Settings JSON</label>
              <textarea className="textarea" value={form.settings} onChange={(e) => setForm({ ...form, settings: e.target.value })} />
            </div>
            {error && <p className="error">{error}</p>}
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
