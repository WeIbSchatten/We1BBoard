import { FormEvent, useEffect, useState } from 'react'
import { api, type Bridge } from '../api'
import { useApp } from '../AppContext'
import { ConfirmModal } from '../components/ConfirmModal'

const PROTOCOLS = ['vless', 'vmess', 'trojan', 'shadowsocks', 'socks', 'http', 'wireguard'] as const
const NETWORKS = ['tcp', 'ws', 'grpc', 'httpupgrade', 'xhttp', 'kcp'] as const
const SECURITIES = ['none', 'tls', 'reality', 'xtls'] as const

type BridgeForm = Omit<Bridge, 'id'> & {
  dialerPassword: string
  dialerEmail: string
  dialerMethod: string
  dialerNetwork: string
  dialerPath: string
  dialerHost: string
  dialerServiceName: string
  dialerSettings: string
  dialerStreamSettings: string
}

const empty: BridgeForm = {
  name: '',
  enable: true,
  dialerProtocol: 'vless',
  dialerAddress: '',
  dialerPort: 443,
  dialerUUID: '',
  dialerPassword: '',
  dialerEmail: '',
  dialerMethod: 'aes-256-gcm',
  dialerFlow: '',
  dialerSecurity: 'reality',
  dialerNetwork: 'tcp',
  dialerSNI: '',
  dialerPublicKey: '',
  dialerShortId: '',
  dialerFingerprint: 'chrome',
  dialerPath: '',
  dialerHost: '',
  dialerServiceName: '',
  dialerSettings: '{}',
  dialerStreamSettings: '{}',
  outboundTag: '',
  routingInbound: '',
  remark: '',
}

export function BridgesPage() {
  const { tr } = useApp()
  const [rows, setRows] = useState<Bridge[]>([])
  const [form, setForm] = useState<BridgeForm>(empty)
  const [open, setOpen] = useState(false)
  const [hint, setHint] = useState('')
  const [advanced, setAdvanced] = useState(false)
  const [confirmId, setConfirmId] = useState<number | null>(null)

  async function load() {
    setRows(await api<Bridge[]>('/bridges'))
  }
  useEffect(() => { load().catch(console.error) }, [])

  async function create(e: FormEvent) {
    e.preventDefault()
    await api('/bridges', { method: 'POST', body: JSON.stringify(form) })
    setOpen(false)
    setForm(empty)
    await load()
  }

  async function remove(id: number) {
    await api(`/bridges/${id}`, { method: 'DELETE' })
    await load()
  }

  async function showHint(id: number) {
    const data = await api<Record<string, unknown>>(`/bridges/${id}/hint`)
    setHint(JSON.stringify(data, null, 2))
  }

  const set = <K extends keyof BridgeForm>(k: K, v: BridgeForm[K]) => setForm((f) => ({ ...f, [k]: v }))

  return (
    <div>
      <div className="page-head">
        <div>
          <h1 className="page-title">{tr('bridges')}</h1>
          <p className="page-sub">Мост entry→exit на любом outbound-протоколе Xray (VLESS / VMess / Trojan / SS / …)</p>
        </div>
        <button className="btn" onClick={() => setOpen(true)}>{tr('create')}</button>
      </div>
      <div className="card">
        <table className="table">
          <thead>
            <tr>
              <th>Name</th>
              <th>Dialer</th>
              <th>Transport</th>
              <th>Outbound</th>
              <th>Routing</th>
              <th>{tr('actions')}</th>
            </tr>
          </thead>
          <tbody>
            {rows.length === 0 && <tr><td colSpan={6}>{tr('empty')}</td></tr>}
            {rows.map((b) => (
              <tr key={b.id}>
                <td>{b.name}</td>
                <td>{b.dialerProtocol}://{b.dialerAddress}:{b.dialerPort}</td>
                <td><span className="badge">{(b as BridgeForm).dialerNetwork || 'tcp'} / {b.dialerSecurity}</span></td>
                <td>{b.outboundTag}</td>
                <td>{b.routingInbound || '—'}</td>
                <td className="row-actions">
                  <button className="btn btn-sm secondary" onClick={() => showHint(b.id)}>Hint</button>
                  <button className="btn btn-sm danger" onClick={() => setConfirmId(b.id)}>{tr('delete')}</button>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
      {hint && <div className="card" style={{ marginTop: 12 }}><textarea className="textarea" readOnly value={hint} /></div>}

      {open && (
        <div className="modal-backdrop" onClick={() => setOpen(false)}>
          <form className="modal" onClick={(e) => e.stopPropagation()} onSubmit={create}>
            <h3>{tr('create')} bridge</h3>
            <div className="grid2">
              <div className="field">
                <label className="label">Name</label>
                <input className="input" value={form.name} onChange={(e) => set('name', e.target.value)} required />
              </div>
              <div className="field">
                <label className="label">Protocol (Xray outbound)</label>
                <select className="select" value={form.dialerProtocol} onChange={(e) => set('dialerProtocol', e.target.value)}>
                  {PROTOCOLS.map((p) => <option key={p} value={p}>{p}</option>)}
                </select>
              </div>
              <div className="field">
                <label className="label">Exit address</label>
                <input className="input" value={form.dialerAddress} onChange={(e) => set('dialerAddress', e.target.value)} required />
              </div>
              <div className="field">
                <label className="label">Exit port</label>
                <input className="input" type="number" value={form.dialerPort} onChange={(e) => set('dialerPort', Number(e.target.value))} />
              </div>
              <div className="field">
                <label className="label">Network</label>
                <select className="select" value={form.dialerNetwork} onChange={(e) => set('dialerNetwork', e.target.value)}>
                  {NETWORKS.map((n) => <option key={n} value={n}>{n}</option>)}
                </select>
              </div>
              <div className="field">
                <label className="label">Security</label>
                <select className="select" value={form.dialerSecurity} onChange={(e) => set('dialerSecurity', e.target.value)}>
                  {SECURITIES.map((n) => <option key={n} value={n}>{n}</option>)}
                </select>
              </div>
              {(form.dialerProtocol === 'vless' || form.dialerProtocol === 'vmess') && (
                <>
                  <div className="field">
                    <label className="label">UUID (empty=auto)</label>
                    <input className="input" value={form.dialerUUID} onChange={(e) => set('dialerUUID', e.target.value)} />
                  </div>
                  {form.dialerProtocol === 'vless' && (
                    <div className="field">
                      <label className="label">Flow</label>
                      <input className="input" value={form.dialerFlow} onChange={(e) => set('dialerFlow', e.target.value)} placeholder="xtls-rprx-vision" />
                    </div>
                  )}
                </>
              )}
              {(['trojan', 'shadowsocks', 'socks', 'http', 'wireguard'].includes(form.dialerProtocol)) && (
                <div className="field">
                  <label className="label">Password / secret</label>
                  <input className="input" value={form.dialerPassword} onChange={(e) => set('dialerPassword', e.target.value)} />
                </div>
              )}
              {form.dialerProtocol === 'shadowsocks' && (
                <div className="field">
                  <label className="label">Method</label>
                  <input className="input" value={form.dialerMethod} onChange={(e) => set('dialerMethod', e.target.value)} />
                </div>
              )}
              {(['socks', 'http'].includes(form.dialerProtocol)) && (
                <div className="field">
                  <label className="label">Username</label>
                  <input className="input" value={form.dialerEmail} onChange={(e) => set('dialerEmail', e.target.value)} />
                </div>
              )}
              {(form.dialerSecurity === 'tls' || form.dialerSecurity === 'reality' || form.dialerSecurity === 'xtls') && (
                <div className="field">
                  <label className="label">SNI</label>
                  <input className="input" value={form.dialerSNI} onChange={(e) => set('dialerSNI', e.target.value)} />
                </div>
              )}
              {form.dialerSecurity === 'reality' && (
                <>
                  <div className="field">
                    <label className="label">REALITY public key</label>
                    <input className="input" value={form.dialerPublicKey} onChange={(e) => set('dialerPublicKey', e.target.value)} />
                  </div>
                  <div className="field">
                    <label className="label">Short ID</label>
                    <input className="input" value={form.dialerShortId} onChange={(e) => set('dialerShortId', e.target.value)} />
                  </div>
                </>
              )}
              {form.dialerNetwork === 'ws' && (
                <>
                  <div className="field"><label className="label">WS path</label><input className="input" value={form.dialerPath} onChange={(e) => set('dialerPath', e.target.value)} /></div>
                  <div className="field"><label className="label">WS host</label><input className="input" value={form.dialerHost} onChange={(e) => set('dialerHost', e.target.value)} /></div>
                </>
              )}
              {form.dialerNetwork === 'grpc' && (
                <div className="field"><label className="label">gRPC serviceName</label><input className="input" value={form.dialerServiceName} onChange={(e) => set('dialerServiceName', e.target.value)} /></div>
              )}
              <div className="field">
                <label className="label">Routing inbound tags (csv)</label>
                <input className="input" value={form.routingInbound} onChange={(e) => set('routingInbound', e.target.value)} placeholder="inbound-vless-443" />
              </div>
              <div className="field">
                <label className="label">Outbound tag</label>
                <input className="input" value={form.outboundTag} onChange={(e) => set('outboundTag', e.target.value)} />
              </div>
            </div>

            <button type="button" className="btn ghost" onClick={() => setAdvanced(!advanced)}>
              {advanced ? '▾' : '▸'} Advanced JSON (как в 3x-ui settings/streamSettings)
            </button>
            {advanced && (
              <>
                <div className="field">
                  <label className="label">dialerSettings (полная перезапись Xray outbound settings)</label>
                  <textarea className="textarea" value={form.dialerSettings} onChange={(e) => set('dialerSettings', e.target.value)} />
                </div>
                <div className="field">
                  <label className="label">dialerStreamSettings</label>
                  <textarea className="textarea" value={form.dialerStreamSettings} onChange={(e) => set('dialerStreamSettings', e.target.value)} />
                </div>
              </>
            )}

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
        message={tr('confirmDeleteBridge')}
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
