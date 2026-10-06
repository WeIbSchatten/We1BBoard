import { FormEvent, useCallback, useEffect, useMemo, useState } from 'react'
import { api, type GeodataStatus, type Outbound } from '../api'
import { useApp } from '../AppContext'

type Tab = 'basics' | 'dns' | 'balancers' | 'advanced' | 'routeTest' | 'geodata'

type Balancer = {
  tag: string
  selector?: string[]
  strategy?: { type?: string }
  fallbackTag?: string
}

type Observatory = {
  subjectSelector?: string[]
  probeURL?: string
  probeInterval?: string
}

type XrayTemplate = {
  log?: { loglevel?: string; access?: string; error?: string }
  routing?: {
    domainStrategy?: string
    balancers?: Balancer[]
    rules?: unknown[]
  }
  dns?: unknown
  observatory?: Observatory
  burstObservatory?: unknown
  [key: string]: unknown
}

type RouteTestResult = {
  outboundTag?: string
  balancerTag?: string
  matchedRule: Record<string, unknown> | null
}

const DOMAIN_STRATEGIES = ['AsIs', 'IPIfNonMatch', 'IPOnDemand'] as const
const LOG_LEVELS = ['debug', 'info', 'warning', 'error', 'none'] as const
const BALANCER_STRATEGIES = ['random', 'roundRobin', 'leastPing', 'leastLoad'] as const

const DNS_SAMPLE = `{
  "servers": [
    "1.1.1.1",
    "8.8.8.8",
    {
      "address": "localhost",
      "domains": ["geosite:private"]
    }
  ],
  "queryStrategy": "UseIPv4"
}`

const DNS_PRESETS: { id: string; label: string; servers: unknown[] }[] = [
  { id: 'cloudflare', label: 'Cloudflare 1.1.1.1', servers: ['1.1.1.1', '1.0.0.1'] },
  { id: 'google', label: 'Google 8.8.8.8', servers: ['8.8.8.8', '8.8.4.4'] },
  { id: 'adguard', label: 'AdGuard', servers: ['dns.adguard-dns.com'] },
]

function csvToList(s: string): string[] {
  return s.split(',').map((x) => x.trim()).filter(Boolean)
}

function listToCsv(arr?: string[]): string {
  return (arr || []).join(', ')
}

export function XrayPage() {
  const { tr } = useApp()
  const [tab, setTab] = useState<Tab>('basics')
  const [tpl, setTpl] = useState<XrayTemplate | null>(null)
  const [advancedText, setAdvancedText] = useState('')
  const [dnsText, setDnsText] = useState('')
  const [outbounds, setOutbounds] = useState<Outbound[]>([])
  const [msg, setMsg] = useState('')
  const [busy, setBusy] = useState(false)

  const [domainStrategy, setDomainStrategy] = useState('AsIs')
  const [loglevel, setLoglevel] = useState('warning')

  const [balModal, setBalModal] = useState<{ open: boolean; index: number; form: BalancerForm }>({
    open: false,
    index: -1,
    form: emptyBalancer(),
  })

  const [obsEnable, setObsEnable] = useState(false)
  const [obsProbeURL, setObsProbeURL] = useState('https://www.google.com/generate_204')
  const [obsInterval, setObsInterval] = useState('10s')
  const [obsSelector, setObsSelector] = useState('')

  const [testForm, setTestForm] = useState({
    inboundTag: '', domain: '', ip: '', port: '', network: '', protocol: '', user: '',
  })
  const [testResult, setTestResult] = useState<RouteTestResult | null>(null)
  const [geoStatus, setGeoStatus] = useState<GeodataStatus | null>(null)
  const [geoURLs, setGeoURLs] = useState({ geosite: '', geoip: '' })
  const [xrayVer, setXrayVer] = useState<{ current: string; bin: string } | null>(null)

  const load = useCallback(async () => {
    const [t, obs] = await Promise.all([
      api<XrayTemplate>('/xray/template'),
      api<Outbound[]>('/outbounds'),
    ])
    setTpl(t)
    setOutbounds(obs || [])
    applyTemplateToForms(t)
  }, [])

  const loadXrayVersion = useCallback(async () => {
    try {
      setXrayVer(await api<{ current: string; bin: string }>('/server/xray-version'))
    } catch {
      setXrayVer(null)
    }
  }, [])

  const loadGeodata = useCallback(async () => {
    const st = await api<GeodataStatus>('/server/geodata-status')
    setGeoStatus(st)
    setGeoURLs({ geosite: st.geositeURL || '', geoip: st.geoipURL || '' })
  }, [])

  function applyTemplateToForms(t: XrayTemplate) {
    setDomainStrategy(t.routing?.domainStrategy || 'AsIs')
    setLoglevel(t.log?.loglevel || 'warning')
    setDnsText(t.dns ? JSON.stringify(t.dns, null, 2) : '')
    setAdvancedText(JSON.stringify(t, null, 2))
    const obs = t.observatory
    if (obs && typeof obs === 'object') {
      setObsEnable(true)
      setObsProbeURL(obs.probeURL || 'https://www.google.com/generate_204')
      setObsInterval(obs.probeInterval || '10s')
      setObsSelector(listToCsv(obs.subjectSelector))
    } else {
      setObsEnable(false)
      setObsProbeURL('https://www.google.com/generate_204')
      setObsInterval('10s')
      setObsSelector('')
    }
  }

  useEffect(() => { load().catch(console.error) }, [load])
  useEffect(() => {
    if (tab === 'basics') loadXrayVersion().catch(() => {})
  }, [tab, loadXrayVersion])
  useEffect(() => {
    if (tab === 'geodata') loadGeodata().catch(console.error)
  }, [tab, loadGeodata])

  const balancers = useMemo(() => tpl?.routing?.balancers || [], [tpl])

  async function saveTemplate(next: XrayTemplate) {
    setBusy(true)
    setMsg('')
    try {
      const saved = await api<XrayTemplate>('/xray/template', {
        method: 'POST',
        body: JSON.stringify(next),
      })
      setTpl(saved)
      applyTemplateToForms(saved)
      setMsg('OK')
    } catch (e) {
      setMsg(e instanceof Error ? e.message : 'error')
    } finally {
      setBusy(false)
    }
  }

  async function saveBasics() {
    if (!tpl) return
    const next: XrayTemplate = {
      ...tpl,
      log: { ...(tpl.log || {}), loglevel },
      routing: {
        ...(tpl.routing || {}),
        domainStrategy,
        balancers: tpl.routing?.balancers,
        rules: tpl.routing?.rules,
      },
    }
    // Also patch setting so panel Routing page stays in sync
    await api('/settings', { method: 'POST', body: JSON.stringify({ routingDomainStrategy: domainStrategy }) })
    await saveTemplate(next)
  }

  async function saveDns() {
    if (!tpl) return
    let dns: unknown = undefined
    const raw = dnsText.trim()
    if (raw) {
      try {
        dns = JSON.parse(raw)
      } catch {
        setMsg('DNS: invalid JSON')
        return
      }
    }
    const next: XrayTemplate = { ...tpl }
    if (dns === undefined) delete next.dns
    else next.dns = dns
    await saveTemplate(next)
  }

  async function applyDnsPreset(servers: unknown[]) {
    if (!tpl) return
    const prev = (typeof tpl.dns === 'object' && tpl.dns && !Array.isArray(tpl.dns))
      ? { ...(tpl.dns as Record<string, unknown>) }
      : {}
    const dns = { ...prev, servers }
    const text = JSON.stringify(dns, null, 2)
    setDnsText(text)
    const next: XrayTemplate = { ...tpl, dns }
    await saveTemplate(next)
  }

  async function installXray() {
    if (!confirm(tr('installXrayConfirm'))) return
    setBusy(true)
    setMsg('')
    try {
      const r = await api<{ current?: string; version?: string; message?: string }>('/server/install-xray', {
        method: 'POST',
        body: JSON.stringify({}),
      })
      setMsg(r.message || `OK ${r.current || r.version || ''}`)
      await loadXrayVersion()
    } catch (e) {
      setMsg(e instanceof Error ? e.message : 'error')
    } finally {
      setBusy(false)
    }
  }

  async function saveBalancersAndObs(nextBalancers: Balancer[]) {
    if (!tpl) return
    const next: XrayTemplate = {
      ...tpl,
      routing: {
        ...(tpl.routing || {}),
        domainStrategy: tpl.routing?.domainStrategy || domainStrategy,
        rules: tpl.routing?.rules,
        balancers: nextBalancers.length ? nextBalancers : undefined,
      },
    }
    if (!nextBalancers.length && next.routing) {
      delete next.routing.balancers
    }
    if (obsEnable) {
      next.observatory = {
        probeURL: obsProbeURL,
        probeInterval: obsInterval,
        subjectSelector: csvToList(obsSelector),
      }
    } else {
      delete next.observatory
    }
    await saveTemplate(next)
  }

  async function saveAdvanced() {
    try {
      const parsed = JSON.parse(advancedText) as XrayTemplate
      await saveTemplate(parsed)
    } catch (e) {
      setMsg(e instanceof Error ? e.message : 'invalid JSON')
    }
  }

  async function resetDefault() {
    if (!confirm('Reset template to default?')) return
    const def = await api<XrayTemplate>('/xray/template/default')
    await saveTemplate(def)
  }

  async function runRouteTest(e: FormEvent) {
    e.preventDefault()
    setBusy(true)
    setMsg('')
    try {
      const res = await api<RouteTestResult>('/xray/route-test', {
        method: 'POST',
        body: JSON.stringify(testForm),
      })
      setTestResult(res)
    } catch (err) {
      setMsg(err instanceof Error ? err.message : 'error')
    } finally {
      setBusy(false)
    }
  }

  async function saveGeoURLs() {
    setBusy(true)
    setMsg('')
    try {
      await api('/settings', {
        method: 'POST',
        body: JSON.stringify({
          geodataGeositeURL: geoURLs.geosite,
          geodataGeoipURL: geoURLs.geoip,
        }),
      })
      await loadGeodata()
      setMsg('OK')
    } catch (e) {
      setMsg(e instanceof Error ? e.message : 'error')
    } finally {
      setBusy(false)
    }
  }

  async function updateGeodata() {
    setBusy(true)
    setMsg('')
    try {
      await api('/server/update-geodata', { method: 'POST' })
      await loadGeodata()
      setMsg(tr('geodataUpdated'))
    } catch (e) {
      setMsg(e instanceof Error ? e.message : 'error')
    } finally {
      setBusy(false)
    }
  }

  async function addWarpPlaceholder() {
    setBusy(true)
    setMsg('')
    try {
      const data = await api<{
        outbound: { tag: string; remark: string; settings: string }
        note: string
      }>('/xray/warp/generate', { method: 'POST' })
      let tag = data.outbound.tag || 'warp'
      const existing = new Set(outbounds.map((o) => o.tag))
      if (existing.has(tag)) {
        let i = 2
        while (existing.has(`${tag}-${i}`)) i++
        tag = `${tag}-${i}`
      }
      await api('/outbounds', {
        method: 'POST',
        body: JSON.stringify({
          tag,
          protocol: 'wireguard',
          enable: true,
          remark: data.outbound.remark || 'Cloudflare WARP (placeholder)',
          settings: data.outbound.settings,
          streamSettings: '',
        }),
      })
      setOutbounds(await api<Outbound[]>('/outbounds'))
      setMsg(data.note || 'WARP placeholder added')
    } catch (e) {
      setMsg(e instanceof Error ? e.message : 'error')
    } finally {
      setBusy(false)
    }
  }

  function formatGeoFile(f?: GeodataStatus['geosite']) {
    if (!f?.exists) return 'missing'
    const kb = f.size != null ? `${(f.size / 1024).toFixed(0)} KB` : '?'
    const mt = f.mtime ? new Date(f.mtime * 1000).toLocaleString() : '—'
    return `${kb} · ${mt}`
  }

  const outboundTags = useMemo(
    () => Array.from(new Set([...outbounds.map((o) => o.tag), 'direct', 'blocked'].filter(Boolean))),
    [outbounds],
  )

  const tabs: { id: Tab; label: string }[] = [
    { id: 'basics', label: tr('xrayBasics') },
    { id: 'dns', label: tr('xrayDns') },
    { id: 'balancers', label: tr('xrayBalancers') },
    { id: 'geodata', label: tr('geodata') },
    { id: 'advanced', label: tr('xrayAdvanced') },
    { id: 'routeTest', label: tr('xrayRouteTest') },
  ]

  if (!tpl) {
    return <div><h1 className="page-title">{tr('xray')}</h1><p className="page-sub">…</p></div>
  }

  return (
    <div>
      <div className="page-head">
        <div>
          <h1 className="page-title">{tr('xray')}</h1>
          <p className="page-sub">{tr('xrayHint')}</p>
        </div>
      </div>

      <div className="tabs" style={{ display: 'flex', flexWrap: 'wrap', gap: 6, marginBottom: 12 }}>
        {tabs.map((t) => (
          <button
            key={t.id}
            type="button"
            className={`tab ${tab === t.id ? 'active' : ''}`}
            onClick={() => setTab(t.id)}
          >
            {t.label}
          </button>
        ))}
      </div>

      {tab === 'basics' && (
        <div className="card">
          <div className="grid2">
            <div className="field">
              <label className="label">{tr('domainStrategy')}</label>
              <select className="select" value={domainStrategy} onChange={(e) => setDomainStrategy(e.target.value)}>
                {DOMAIN_STRATEGIES.map((s) => <option key={s} value={s}>{s}</option>)}
              </select>
            </div>
            <div className="field">
              <label className="label">{tr('xrayLoglevel')}</label>
              <select className="select" value={loglevel} onChange={(e) => setLoglevel(e.target.value)}>
                {LOG_LEVELS.map((s) => <option key={s} value={s}>{s}</option>)}
              </select>
            </div>
          </div>
          <button className="btn" type="button" disabled={busy} onClick={() => void saveBasics()}>
            {busy ? '…' : tr('save')}
          </button>

          <div style={{ marginTop: 20, paddingTop: 16, borderTop: '1px solid var(--border)' }}>
            <div className="label">{tr('xrayVersion')}</div>
            <p className="page-sub" style={{ marginTop: 4 }}>
              {xrayVer?.current || '—'}
              {xrayVer?.bin ? (
                <>
                  {' · '}
                  <code style={{ fontFamily: 'var(--mono)', fontSize: '0.8rem' }}>{xrayVer.bin}</code>
                </>
              ) : null}
            </p>
            <button className="btn secondary" type="button" disabled={busy} onClick={() => void installXray()}>
              {tr('installXray')}
            </button>
          </div>
        </div>
      )}

      {tab === 'dns' && (
        <div className="card">
          <p className="page-sub" style={{ marginTop: 0 }}>{tr('xrayDnsHint')}</p>
          <div className="row-actions" style={{ marginBottom: 10, flexWrap: 'wrap' }}>
            {DNS_PRESETS.map((p) => (
              <button
                key={p.id}
                className="btn secondary"
                type="button"
                disabled={busy}
                onClick={() => void applyDnsPreset(p.servers)}
              >
                {p.label}
              </button>
            ))}
          </div>
          <textarea
            className="input"
            style={{ fontFamily: 'var(--mono)', minHeight: 280, width: '100%' }}
            value={dnsText}
            onChange={(e) => setDnsText(e.target.value)}
            placeholder={DNS_SAMPLE}
          />
          <div className="row-actions" style={{ marginTop: 10 }}>
            <button className="btn secondary" type="button" onClick={() => setDnsText(DNS_SAMPLE)}>
              {tr('xrayDnsSample')}
            </button>
            <button className="btn" type="button" disabled={busy} onClick={() => void saveDns()}>
              {busy ? '…' : tr('save')}
            </button>
          </div>
        </div>
      )}

      {tab === 'balancers' && (
        <div>
          <div className="card" style={{ marginBottom: 12 }}>
            <div className="page-head" style={{ marginBottom: 8 }}>
              <h3 style={{ margin: 0 }}>{tr('xrayBalancers')}</h3>
              <button
                className="btn"
                type="button"
                onClick={() => setBalModal({ open: true, index: -1, form: emptyBalancer() })}
              >
                {tr('create')}
              </button>
            </div>
            <table className="table">
              <thead>
                <tr>
                  <th>tag</th>
                  <th>selector</th>
                  <th>strategy</th>
                  <th>fallback</th>
                  <th>{tr('actions')}</th>
                </tr>
              </thead>
              <tbody>
                {balancers.length === 0 && (
                  <tr><td colSpan={5}>{tr('empty')}</td></tr>
                )}
                {balancers.map((b, i) => (
                  <tr key={`${b.tag}-${i}`}>
                    <td><code style={{ fontFamily: 'var(--mono)' }}>{b.tag}</code></td>
                    <td>{listToCsv(b.selector) || '—'}</td>
                    <td>{b.strategy?.type || 'random'}</td>
                    <td>{b.fallbackTag || '—'}</td>
                    <td className="row-actions">
                      <button
                        className="btn secondary"
                        type="button"
                        onClick={() => setBalModal({
                          open: true,
                          index: i,
                          form: {
                            tag: b.tag || '',
                            selector: listToCsv(b.selector),
                            strategy: b.strategy?.type || 'random',
                            fallbackTag: b.fallbackTag || '',
                          },
                        })}
                      >
                        {tr('edit')}
                      </button>
                      <button
                        className="btn danger"
                        type="button"
                        onClick={() => {
                          const next = balancers.filter((_, j) => j !== i)
                          void saveBalancersAndObs(next)
                        }}
                      >
                        {tr('delete')}
                      </button>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>

          <div className="card">
            <h3 style={{ marginTop: 0 }}>{tr('xrayObservatory')}</h3>
            <div className="field">
              <label className="label" style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
                <input type="checkbox" checked={obsEnable} onChange={(e) => setObsEnable(e.target.checked)} />
                {tr('enable')}
              </label>
            </div>
            <div className="grid2">
              <div className="field">
                <label className="label">probeURL</label>
                <input className="input" value={obsProbeURL} disabled={!obsEnable} onChange={(e) => setObsProbeURL(e.target.value)} />
              </div>
              <div className="field">
                <label className="label">probeInterval</label>
                <input className="input" value={obsInterval} disabled={!obsEnable} onChange={(e) => setObsInterval(e.target.value)} placeholder="10s" />
              </div>
              <div className="field" style={{ gridColumn: '1 / -1' }}>
                <label className="label">subjectSelector (csv outbound tags)</label>
                <input className="input" value={obsSelector} disabled={!obsEnable} onChange={(e) => setObsSelector(e.target.value)} placeholder={outboundTags.join(', ')} />
              </div>
            </div>
            <button className="btn" type="button" disabled={busy} onClick={() => void saveBalancersAndObs(balancers)}>
              {busy ? '…' : tr('save')}
            </button>
          </div>
        </div>
      )}

      {tab === 'geodata' && (
        <div className="card">
          <h3 style={{ marginTop: 0 }}>{tr('geodata')}</h3>
          <p className="page-sub" style={{ marginTop: 0 }}>{tr('geodataHint')}</p>
          <div className="grid2">
            <div className="field">
              <label className="label">geosite.dat</label>
              <div className="page-sub">{formatGeoFile(geoStatus?.geosite)}</div>
              <code style={{ fontFamily: 'var(--mono)', fontSize: 12 }}>{geoStatus?.geosite?.path || geoStatus?.dir || '—'}</code>
            </div>
            <div className="field">
              <label className="label">geoip.dat</label>
              <div className="page-sub">{formatGeoFile(geoStatus?.geoip)}</div>
              <code style={{ fontFamily: 'var(--mono)', fontSize: 12 }}>{geoStatus?.geoip?.path || '—'}</code>
            </div>
            <div className="field" style={{ gridColumn: '1 / -1' }}>
              <label className="label">geodataGeositeURL</label>
              <input className="input" value={geoURLs.geosite} onChange={(e) => setGeoURLs({ ...geoURLs, geosite: e.target.value })} />
            </div>
            <div className="field" style={{ gridColumn: '1 / -1' }}>
              <label className="label">geodataGeoipURL</label>
              <input className="input" value={geoURLs.geoip} onChange={(e) => setGeoURLs({ ...geoURLs, geoip: e.target.value })} />
            </div>
          </div>
          <div className="row-actions" style={{ marginTop: 10 }}>
            <button className="btn secondary" type="button" disabled={busy} onClick={() => void saveGeoURLs()}>
              {tr('save')}
            </button>
            <button className="btn" type="button" disabled={busy} onClick={() => void updateGeodata()}>
              {busy ? '…' : tr('updateGeodata')}
            </button>
          </div>
        </div>
      )}

      {tab === 'advanced' && (
        <div className="card">
          <p className="page-sub" style={{ marginTop: 0 }}>{tr('xrayAdvancedHint')}</p>
          <div className="row-actions" style={{ marginBottom: 10 }}>
            <button className="btn secondary" type="button" disabled={busy} onClick={() => void addWarpPlaceholder()}>
              {tr('addWarp')}
            </button>
          </div>
          <textarea
            className="input"
            style={{ fontFamily: 'var(--mono)', minHeight: 420, width: '100%' }}
            value={advancedText}
            onChange={(e) => setAdvancedText(e.target.value)}
          />
          <div className="row-actions" style={{ marginTop: 10 }}>
            <button className="btn danger" type="button" disabled={busy} onClick={() => void resetDefault()}>
              {tr('xrayResetDefault')}
            </button>
            <button className="btn" type="button" disabled={busy} onClick={() => void saveAdvanced()}>
              {busy ? '…' : tr('save')}
            </button>
          </div>
        </div>
      )}

      {tab === 'routeTest' && (
        <div className="card">
          <form onSubmit={runRouteTest}>
            <div className="grid2">
              {([
                ['inboundTag', 'Inbound tag'],
                ['domain', 'Domain'],
                ['ip', 'IP'],
                ['port', 'Port'],
                ['network', 'Network'],
                ['protocol', 'Protocol'],
                ['user', 'User (email)'],
              ] as const).map(([key, label]) => (
                <div className="field" key={key}>
                  <label className="label">{label}</label>
                  <input
                    className="input"
                    value={testForm[key]}
                    onChange={(e) => setTestForm({ ...testForm, [key]: e.target.value })}
                  />
                </div>
              ))}
            </div>
            <button className="btn" type="submit" disabled={busy}>{busy ? '…' : tr('xrayRunTest')}</button>
          </form>
          {testResult && (
            <pre className="log-view" style={{ marginTop: 12, minHeight: 120 }}>
              {JSON.stringify(testResult, null, 2)}
            </pre>
          )}
        </div>
      )}

      {msg && <p className="page-sub" style={{ marginTop: 10 }}>{msg}</p>}

      {balModal.open && (
        <BalancerModal
          form={balModal.form}
          outboundTags={outboundTags}
          onClose={() => setBalModal({ open: false, index: -1, form: emptyBalancer() })}
          onSave={(form) => {
            const item: Balancer = {
              tag: form.tag.trim(),
              selector: csvToList(form.selector),
              strategy: { type: form.strategy },
              fallbackTag: form.fallbackTag.trim() || undefined,
            }
            const next = [...balancers]
            if (balModal.index >= 0) next[balModal.index] = item
            else next.push(item)
            setBalModal({ open: false, index: -1, form: emptyBalancer() })
            void saveBalancersAndObs(next)
          }}
        />
      )}

      <style>{`
        .tab {
          border: 1px solid var(--border);
          background: transparent;
          color: var(--text-muted);
          border-radius: 999px;
          padding: 0.4rem 0.85rem;
          cursor: pointer;
          font-weight: 600;
          font-size: 0.85rem;
        }
        .tab.active {
          background: var(--accent-soft);
          color: var(--accent);
          border-color: transparent;
        }
        .log-view {
          margin: 0;
          padding: 1rem 1.1rem;
          overflow: auto;
          border: 1px solid var(--border);
          border-radius: 14px;
          background: color-mix(in srgb, var(--bg-sidebar) 85%, transparent);
          font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
          font-size: 0.78rem;
          line-height: 1.45;
          white-space: pre-wrap;
          word-break: break-word;
        }
      `}</style>
    </div>
  )
}

type BalancerForm = {
  tag: string
  selector: string
  strategy: string
  fallbackTag: string
}

function emptyBalancer(): BalancerForm {
  return { tag: '', selector: '', strategy: 'random', fallbackTag: '' }
}

function BalancerModal({
  form: initial,
  outboundTags,
  onClose,
  onSave,
}: {
  form: BalancerForm
  outboundTags: string[]
  onClose: () => void
  onSave: (f: BalancerForm) => void
}) {
  const { tr } = useApp()
  const [form, setForm] = useState(initial)

  function toggleTag(tag: string) {
    const set = new Set(csvToList(form.selector))
    if (set.has(tag)) set.delete(tag)
    else set.add(tag)
    setForm({ ...form, selector: Array.from(set).join(', ') })
  }

  return (
    <div className="modal-backdrop" onClick={onClose}>
      <form
        className="modal"
        style={{ width: 'min(560px, 100%)' }}
        onClick={(e) => e.stopPropagation()}
        onSubmit={(e) => {
          e.preventDefault()
          if (!form.tag.trim()) return
          onSave(form)
        }}
      >
        <h3>{tr('xrayBalancers')}</h3>
        <div className="field">
          <label className="label">tag *</label>
          <input className="input" value={form.tag} onChange={(e) => setForm({ ...form, tag: e.target.value })} required />
        </div>
        <div className="field">
          <label className="label">selector (csv outbound tags)</label>
          <input className="input" value={form.selector} onChange={(e) => setForm({ ...form, selector: e.target.value })} />
          <div className="chip-row" style={{ marginTop: 6 }}>
            {outboundTags.map((t) => (
              <button
                key={t}
                type="button"
                className={`chip${csvToList(form.selector).includes(t) ? ' active' : ''}`}
                onClick={() => toggleTag(t)}
              >
                {t}
              </button>
            ))}
          </div>
        </div>
        <div className="field">
          <label className="label">strategy</label>
          <select className="select" value={form.strategy} onChange={(e) => setForm({ ...form, strategy: e.target.value })}>
            {BALANCER_STRATEGIES.map((s) => <option key={s} value={s}>{s}</option>)}
          </select>
        </div>
        <div className="field">
          <label className="label">fallbackTag</label>
          <select className="select" value={form.fallbackTag} onChange={(e) => setForm({ ...form, fallbackTag: e.target.value })}>
            <option value="">—</option>
            {outboundTags.map((t) => <option key={t} value={t}>{t}</option>)}
          </select>
        </div>
        <div className="row-actions">
          <button className="btn" type="submit">{tr('save')}</button>
          <button className="btn secondary" type="button" onClick={onClose}>{tr('cancel')}</button>
        </div>
      </form>
    </div>
  )
}
