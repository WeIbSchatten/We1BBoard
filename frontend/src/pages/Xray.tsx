import { FormEvent, useCallback, useEffect, useMemo, useState } from 'react'
import { api, type Outbound } from '../api'
import { useApp } from '../AppContext'

type Tab = 'basics' | 'dns' | 'balancers' | 'advanced' | 'routeTest'

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

  const load = useCallback(async () => {
    const [t, obs] = await Promise.all([
      api<XrayTemplate>('/xray/template'),
      api<Outbound[]>('/outbounds'),
    ])
    setTpl(t)
    setOutbounds(obs || [])
    applyTemplateToForms(t)
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

  const outboundTags = useMemo(
    () => Array.from(new Set([...outbounds.map((o) => o.tag), 'direct', 'blocked'].filter(Boolean))),
    [outbounds],
  )

  const tabs: { id: Tab; label: string }[] = [
    { id: 'basics', label: tr('xrayBasics') },
    { id: 'dns', label: tr('xrayDns') },
    { id: 'balancers', label: tr('xrayBalancers') },
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
        </div>
      )}

      {tab === 'dns' && (
        <div className="card">
          <p className="page-sub" style={{ marginTop: 0 }}>{tr('xrayDnsHint')}</p>
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

      {tab === 'advanced' && (
        <div className="card">
          <p className="page-sub" style={{ marginTop: 0 }}>{tr('xrayAdvancedHint')}</p>
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
