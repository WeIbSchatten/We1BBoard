import { useEffect, useState } from 'react'
import { api, type Inbound, type Outbound } from '../api'
import { useApp } from '../AppContext'
import { RoutingFormModal, type RoutingRule } from '../components/RoutingFormModal'

const DOMAIN_STRATEGIES = ['AsIs', 'IPIfNonMatch', 'IPOnDemand'] as const

type Preset = {
  key: string
  labelKey: 'blockPrivate' | 'blockBittorrent' | 'blockAds' | 'directCN'
  match: (r: RoutingRule) => boolean
  body: Omit<RoutingRule, 'id'>
}

const PRESETS: Preset[] = [
  {
    key: 'block-private',
    labelKey: 'blockPrivate',
    match: (r) => r.ip.includes('geoip:private') && r.outboundTag === 'blocked',
    body: {
      remark: 'Block private IPs',
      enable: true,
      priority: 10,
      inboundTag: '',
      outboundTag: 'blocked',
      balancerTag: '',
      domain: '',
      ip: 'geoip:private',
      port: '',
      network: '',
      protocol: '',
    },
  },
  {
    key: 'block-bittorrent',
    labelKey: 'blockBittorrent',
    match: (r) => r.protocol.toLowerCase().includes('bittorrent') && r.outboundTag === 'blocked',
    body: {
      remark: 'Block BitTorrent',
      enable: true,
      priority: 20,
      inboundTag: '',
      outboundTag: 'blocked',
      balancerTag: '',
      domain: '',
      ip: '',
      port: '',
      network: '',
      protocol: 'bittorrent',
    },
  },
  {
    key: 'block-ads',
    labelKey: 'blockAds',
    match: (r) => r.domain.includes('geosite:category-ads-all') && r.outboundTag === 'blocked',
    body: {
      remark: 'Block ads',
      enable: true,
      priority: 30,
      inboundTag: '',
      outboundTag: 'blocked',
      balancerTag: '',
      domain: 'geosite:category-ads-all',
      ip: '',
      port: '',
      network: '',
      protocol: '',
    },
  },
  {
    key: 'direct-cn',
    labelKey: 'directCN',
    match: (r) =>
      r.outboundTag === 'direct' &&
      (r.ip.includes('geoip:cn') || r.domain.includes('geosite:cn')),
    body: {
      remark: 'Direct CN',
      enable: true,
      priority: 40,
      inboundTag: '',
      outboundTag: 'direct',
      balancerTag: '',
      domain: 'geosite:cn',
      ip: 'geoip:cn',
      port: '',
      network: '',
      protocol: '',
    },
  },
]

export function RoutingPage() {
  const { tr } = useApp()
  const [rows, setRows] = useState<RoutingRule[]>([])
  const [inbounds, setInbounds] = useState<Inbound[]>([])
  const [outbounds, setOutbounds] = useState<Outbound[]>([])
  const [domainStrategy, setDomainStrategy] = useState<string>('AsIs')
  const [savingStrategy, setSavingStrategy] = useState(false)
  const [presetBusy, setPresetBusy] = useState<string | null>(null)
  const [presetMsg, setPresetMsg] = useState('')
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

  async function applyPreset(p: Preset) {
    setPresetMsg('')
    if (rows.some(p.match)) {
      setPresetMsg(`${tr(p.labelKey)}: already exists`)
      return
    }
    setPresetBusy(p.key)
    try {
      await api('/routing', { method: 'POST', body: JSON.stringify(p.body) })
      await load()
      setPresetMsg(`${tr(p.labelKey)}: OK`)
    } catch (e) {
      setPresetMsg(e instanceof Error ? e.message : 'error')
    } finally {
      setPresetBusy(null)
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

      <div className="card" style={{ marginBottom: 12 }}>
        <div className="label" style={{ marginBottom: 8 }}>{tr('routingPresets')}</div>
        <div className="row-actions" style={{ flexWrap: 'wrap' }}>
          {PRESETS.map((p) => {
            const exists = rows.some(p.match)
            return (
              <button
                key={p.key}
                type="button"
                className="btn secondary"
                disabled={!!presetBusy || exists}
                onClick={() => { void applyPreset(p) }}
                title={exists ? 'already exists' : undefined}
              >
                {presetBusy === p.key ? '…' : tr(p.labelKey)}
              </button>
            )
          })}
        </div>
        {presetMsg && <p className="page-sub" style={{ margin: '8px 0 0' }}>{presetMsg}</p>}
      </div>

      <div className="card">
        <table className="table">
          <thead>
            <tr>
              <th>Priority</th>
              <th>{tr('remark')}</th>
              <th>Inbound</th>
              <th>Outbound / Balancer</th>
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
                <td>
                  <code style={{ fontFamily: 'var(--mono)' }}>
                    {r.balancerTag ? `balancer:${r.balancerTag}` : r.outboundTag}
                  </code>
                </td>
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
