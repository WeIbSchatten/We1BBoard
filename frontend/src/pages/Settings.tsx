import { useEffect, useState } from 'react'
import { api } from '../api'
import { useApp } from '../AppContext'
import { ConfirmModal } from '../components/ConfirmModal'

type SubInfo = {
  enable: boolean
  subPort: string
  subPortSetting?: string
  subPath: string
  subHost: string
  subHostSetting?: string
  dedicated?: boolean
  baseUrl: string
  formats: string[]
}

type TwoFASetup = {
  secret: string
  otpauth: string
  qr?: string
}

type SubBalancer = {
  id: number
  name: string
  strategy: string
  selector: string
  enable: boolean
  remark: string
}

const LABELS: Record<string, string> = {
  panelPort: 'Panel port',
  panelPath: 'Panel path',
  webListen: 'Listen address',
  certFile: 'TLS certificate',
  keyFile: 'TLS private key',
  subEnable: 'Enable subscriptions',
  subPort: 'Subscription port',
  subPath: 'Subscription path',
  subHost: 'Subscription host',
  subURI: 'Public subscription URI',
  subForceTLS: 'Force HTTPS in subscription URLs',
  subTitle: 'Subscription title',
  subSupportUrl: 'Support URL',
  subThemeDir: 'Custom theme directory',
  subAnnounce: 'Announcement',
  subJsonEnable: 'JSON subscription format',
  subClashEnable: 'Clash subscription format',
  ufwEnable: 'Auto-open UFW ports',
  tgBotEnable: 'Enable Telegram bot',
  tgBotToken: 'Telegram bot token',
  tgBotChatId: 'Telegram chat ID',
  tgNotifyLogin: 'Notify on login',
  tgNotifyTraffic: 'Notify on traffic',
  emailEnable: 'Enable email',
  smtpHost: 'SMTP host',
  smtpPort: 'SMTP port',
  smtpUser: 'SMTP user',
  smtpPass: 'SMTP password',
  smtpFrom: 'From address',
  emailNotifyLogin: 'Notify on login',
  discordEnable: 'Enable Discord',
  discordWebhook: 'Discord webhook URL',
  discordNotifyLogin: 'Notify on login',
}

function label(key: string) {
  return LABELS[key] || key
}

function truthy(v: string | undefined) {
  return v === 'true' || v === '1' || v === 'yes' || v === 'on'
}

export function SettingsPage() {
  const { tr, theme, setTheme, accent, setAccent, lang, setLang } = useApp()
  const [settings, setSettings] = useState<Record<string, string>>({})
  const [subInfo, setSubInfo] = useState<SubInfo | null>(null)
  const [msg, setMsg] = useState('')
  const [pw, setPw] = useState({ oldPassword: '', newPassword: '' })
  const [twoFA, setTwoFA] = useState<TwoFASetup | null>(null)
  const [twoFACode, setTwoFACode] = useState('')
  const [disablePw, setDisablePw] = useState('')
  const [tgBusy, setTgBusy] = useState(false)
  const [emailBusy, setEmailBusy] = useState(false)
  const [discordBusy, setDiscordBusy] = useState(false)
  const [balancers, setBalancers] = useState<SubBalancer[]>([])
  const [balForm, setBalForm] = useState({ name: '', strategy: 'url-test', selector: '', enable: true, remark: '' })
  const [confirmBalId, setConfirmBalId] = useState<number | null>(null)

  useEffect(() => {
    api<Record<string, string>>('/settings').then((s) => {
      setSettings(s)
      if (s.theme) setTheme(s.theme as 'light' | 'night' | 'amoled')
      if (s.accent) setAccent(s.accent as 'blue' | 'purple')
      if (s.lang === 'ru' || s.lang === 'en') setLang(s.lang)
    }).catch(console.error)
    api<SubInfo>('/subscription').then(setSubInfo).catch(console.error)
    api<SubBalancer[]>('/sub-balancers').then((r) => setBalancers(r || [])).catch(console.error)
  }, [])

  async function save() {
    setMsg('')
    const next = { ...settings, theme, accent, lang }
    await api('/settings', { method: 'POST', body: JSON.stringify(next) })
    setSettings(next)
    setSubInfo(await api<SubInfo>('/subscription'))
    setMsg('OK — restart panel if subPort / subEnable / TLS changed')
  }

  async function changePassword() {
    setMsg('')
    try {
      await api('/password', { method: 'POST', body: JSON.stringify(pw) })
      setMsg('password updated')
      setPw({ oldPassword: '', newPassword: '' })
    } catch (e) {
      setMsg(e instanceof Error ? e.message : 'error')
    }
  }

  async function setup2FA() {
    setMsg('')
    try {
      const data = await api<TwoFASetup>('/settings/2fa/setup')
      setTwoFA(data)
      setTwoFACode('')
    } catch (e) {
      setMsg(e instanceof Error ? e.message : 'error')
    }
  }

  async function enable2FA() {
    setMsg('')
    try {
      await api('/settings/2fa/enable', { method: 'POST', body: JSON.stringify({ code: twoFACode.trim() }) })
      setSettings((s) => ({ ...s, twoFactorEnable: 'true' }))
      setTwoFA(null)
      setTwoFACode('')
      setMsg('2FA enabled')
    } catch (e) {
      setMsg(e instanceof Error ? e.message : 'error')
    }
  }

  async function disable2FA() {
    setMsg('')
    try {
      const body: Record<string, string> = { code: twoFACode.trim() }
      if (disablePw) body.password = disablePw
      await api('/settings/2fa/disable', { method: 'POST', body: JSON.stringify(body) })
      setSettings((s) => ({ ...s, twoFactorEnable: 'false' }))
      setTwoFA(null)
      setTwoFACode('')
      setDisablePw('')
      setMsg('2FA disabled')
    } catch (e) {
      setMsg(e instanceof Error ? e.message : 'error')
    }
  }

  async function testTelegram() {
    setMsg('')
    setTgBusy(true)
    try {
      const token = settings.tgBotToken || ''
      await api('/settings/tg-test', {
        method: 'POST',
        body: JSON.stringify({
          tgBotToken: token.includes('***') || token.includes('…') ? '' : token,
          tgBotChatId: settings.tgBotChatId || '',
        }),
      })
      setMsg('Telegram test sent')
    } catch (e) {
      setMsg(e instanceof Error ? e.message : 'error')
    } finally {
      setTgBusy(false)
    }
  }

  async function testEmail() {
    setMsg('')
    setEmailBusy(true)
    try {
      await api('/settings/email-test', { method: 'POST', body: '{}' })
      setMsg('Email test sent')
    } catch (e) {
      setMsg(e instanceof Error ? e.message : 'error')
    } finally {
      setEmailBusy(false)
    }
  }

  async function testDiscord() {
    setMsg('')
    setDiscordBusy(true)
    try {
      const wh = settings.discordWebhook || ''
      await api('/settings/discord-test', {
        method: 'POST',
        body: JSON.stringify({
          discordWebhook: wh.includes('***') || wh.includes('…') ? '' : wh,
        }),
      })
      setMsg('Discord test sent')
    } catch (e) {
      setMsg(e instanceof Error ? e.message : 'error')
    } finally {
      setDiscordBusy(false)
    }
  }

  async function loadBalancers() {
    setBalancers(await api<SubBalancer[]>('/sub-balancers') || [])
  }

  async function createBalancer() {
    if (!balForm.name.trim()) return
    setMsg('')
    try {
      await api('/sub-balancers', { method: 'POST', body: JSON.stringify(balForm) })
      setBalForm({ name: '', strategy: 'url-test', selector: '', enable: true, remark: '' })
      await loadBalancers()
    } catch (e) {
      setMsg(e instanceof Error ? e.message : 'error')
    }
  }

  async function toggleBalancer(b: SubBalancer) {
    await api(`/sub-balancers/${b.id}`, {
      method: 'PUT',
      body: JSON.stringify({ ...b, enable: !b.enable }),
    })
    await loadBalancers()
  }

  async function deleteBalancer(id: number) {
    await api(`/sub-balancers/${id}`, { method: 'DELETE' })
    await loadBalancers()
  }

  const subEnabled = truthy(settings.subEnable ?? 'true')
  const twoFAOn = truthy(settings.twoFactorEnable)

  return (
    <div>
      <div className="page-head">
        <div>
          <h1 className="page-title">{tr('settings')}</h1>
          <p className="page-sub">Theme, panel, subscriptions, and notifications</p>
        </div>
      </div>
      {msg && <div className="alert success">{msg}</div>}

      <div className="grid2">
        <div className="card">
          <div className="section-title" style={{ marginTop: 0 }}>Appearance</div>
          <div className="field">
            <label className="label">{tr('theme')}</label>
            <select className="select" value={theme} onChange={(e) => setTheme(e.target.value as typeof theme)}>
              <option value="light">{tr('light')}</option>
              <option value="night">{tr('night')}</option>
              <option value="amoled">{tr('amoled')}</option>
            </select>
          </div>
          <div className="field">
            <label className="label">{tr('accent')}</label>
            <select className="select" value={accent} onChange={(e) => setAccent(e.target.value as typeof accent)}>
              <option value="blue">{tr('blue')}</option>
              <option value="purple">{tr('purple')}</option>
            </select>
          </div>
          <div className="field">
            <label className="label">{tr('lang')}</label>
            <select className="select" value={lang} onChange={(e) => setLang(e.target.value as typeof lang)}>
              <option value="ru">Русский</option>
              <option value="en">English</option>
            </select>
          </div>
        </div>

        <div className="card">
          <div className="section-title" style={{ marginTop: 0 }}>Panel</div>
          {(['panelPort', 'panelPath', 'webListen', 'certFile', 'keyFile'] as const).map((k) => (
            <div className="field" key={k}>
              <label className="label">{label(k)}</label>
              <input className="input" value={settings[k] || ''} onChange={(e) => setSettings({ ...settings, [k]: e.target.value })} />
            </div>
          ))}
          <p className="page-sub">TLS paths for the panel. For Let’s Encrypt use install.sh (SSL) — acme.sh renews and restarts.</p>
          <button className="btn" onClick={save}>{tr('save')}</button>
        </div>
      </div>

      <div className="card" style={{ marginTop: 12 }}>
        <div className="section-title" style={{ marginTop: 0 }}>{tr('subscription')}</div>
        <p className="page-sub">Separate listener on subscription port (3x-ui style). UA auto: base64 / Clash / sing-box.</p>
        <div className="field">
          <label className="label" style={{ display: 'flex', alignItems: 'center', gap: 8, textTransform: 'none', letterSpacing: 0 }}>
            <input
              type="checkbox"
              checked={subEnabled}
              onChange={(e) => setSettings({ ...settings, subEnable: e.target.checked ? 'true' : 'false' })}
            />
            {label('subEnable')}
          </label>
        </div>
        <div className="section-title">Endpoints</div>
        <div className="field">
          <label className="label">{label('subPort')}</label>
          <input className="input" value={settings.subPort || ''} onChange={(e) => setSettings({ ...settings, subPort: e.target.value })} />
          <p className="page-sub">{tr('subPortHint')}</p>
        </div>
        <div className="field">
          <label className="label">{label('subPath')}</label>
          <input className="input" value={settings.subPath || ''} onChange={(e) => setSettings({ ...settings, subPath: e.target.value })} />
        </div>
        <div className="field">
          <label className="label">{label('subHost')}</label>
          <input className="input" value={settings.subHost || ''} onChange={(e) => setSettings({ ...settings, subHost: e.target.value })} />
          <p className="page-sub">{tr('subHostHint')}</p>
        </div>
        <div className="field">
          <label className="label">{label('subURI')}</label>
          <input
            className="input"
            value={settings.subURI || ''}
            onChange={(e) => setSettings({ ...settings, subURI: e.target.value })}
            placeholder="https://vpn.example.com:2096"
          />
          <p className="page-sub">{tr('subURIHint')}</p>
        </div>
        <div className="field">
          <label className="label" style={{ display: 'flex', alignItems: 'center', gap: 8, textTransform: 'none', letterSpacing: 0 }}>
            <input
              type="checkbox"
              checked={(settings.subForceTLS || 'false') === 'true'}
              onChange={(e) => setSettings({ ...settings, subForceTLS: e.target.checked ? 'true' : 'false' })}
            />
            {label('subForceTLS')}
          </label>
          <p className="page-sub">{tr('subForceTLSHint')}</p>
        </div>
        {(['subTitle', 'subSupportUrl', 'subThemeDir', 'subAnnounce'] as const).map((k) => (
          <div className="field" key={k}>
            <label className="label">{label(k)}</label>
            <input className="input" value={settings[k] || ''} onChange={(e) => setSettings({ ...settings, [k]: e.target.value })} placeholder={k === 'subThemeDir' ? '/etc/we1bboard/sub_templates/my-theme' : undefined} />
          </div>
        ))}
        <div className="section-title">Formats & firewall</div>
        {([
          ['subJsonEnable', true],
          ['subClashEnable', true],
          ['ufwEnable', true],
        ] as const).map(([k, def]) => (
          <div className="field" key={k}>
            <label className="label" style={{ display: 'flex', alignItems: 'center', gap: 8, textTransform: 'none', letterSpacing: 0 }}>
              <input
                type="checkbox"
                checked={truthy(settings[k] ?? (def ? 'true' : 'false'))}
                onChange={(e) => setSettings({ ...settings, [k]: e.target.checked ? 'true' : 'false' })}
              />
              {label(k)}
            </label>
          </div>
        ))}
        <p className="page-sub">Browsers get an HTML page; VPN clients get raw. Full page: <code>?html=1</code>. Custom theme: folder with <code>sub.html</code> or <code>index.html</code>.</p>
        {subInfo && (
          <div className="field" style={{ marginTop: 8 }}>
            <label className="label">{tr('subPreviewUrl')}</label>
            <input
              className="input"
              readOnly
              value={(subInfo.baseUrl || '') + '{subId}'}
              style={{ fontWeight: 600, letterSpacing: 0.2 }}
            />
            <p className="page-sub" style={{ marginTop: 6 }}>
              {tr('subEffectivePort')}: <code>{subInfo.subPort || '—'}</code>
              {subInfo.subPortSetting && subInfo.subPortSetting !== subInfo.subPort
                ? ` (setting: ${subInfo.subPortSetting})`
                : ''}
              {' · '}
              {subInfo.dedicated ? tr('subModeDedicated') : tr('subModePanel')}
              {' · '}
              host: <code>{subInfo.subHost || '—'}</code>
              {subInfo.formats?.length ? ` · ${subInfo.formats.join(', ')}` : ''}
            </p>
          </div>
        )}
        <button className="btn" onClick={save}>{tr('save')}</button>
      </div>

      <div className="card" style={{ marginTop: 12 }}>
        <div className="section-title" style={{ marginTop: 0 }}>Two-factor (TOTP)</div>
        <p className="page-sub">Status: {twoFAOn ? 'enabled' : 'disabled'}</p>
        {!twoFAOn && (
          <>
            <div className="toolbar" style={{ marginBottom: 8 }}>
              <button className="btn secondary btn-sm" type="button" onClick={() => { void setup2FA() }}>Setup / QR</button>
            </div>
            {twoFA && (
              <>
                {twoFA.qr && <img src={twoFA.qr} alt="2FA QR" style={{ width: 180, height: 180, display: 'block', marginBottom: 8 }} />}
                <div className="field">
                  <label className="label">Secret (base32)</label>
                  <input className="input" readOnly value={twoFA.secret} />
                </div>
                <div className="field">
                  <label className="label">otpauth URL</label>
                  <input className="input" readOnly value={twoFA.otpauth} />
                </div>
                <div className="field">
                  <label className="label">Code from authenticator</label>
                  <input className="input" value={twoFACode} onChange={(e) => setTwoFACode(e.target.value)} placeholder="123456" />
                </div>
                <button className="btn" type="button" onClick={() => { void enable2FA() }}>Enable 2FA</button>
              </>
            )}
          </>
        )}
        {twoFAOn && (
          <>
            <div className="field">
              <label className="label">Code</label>
              <input className="input" value={twoFACode} onChange={(e) => setTwoFACode(e.target.value)} placeholder="123456" />
            </div>
            <div className="field">
              <label className="label">Password (optional)</label>
              <input className="input" type="password" value={disablePw} onChange={(e) => setDisablePw(e.target.value)} />
            </div>
            <button className="btn danger btn-sm" type="button" onClick={() => { void disable2FA() }}>Disable 2FA</button>
          </>
        )}
      </div>

      <div className="card" style={{ marginTop: 12 }}>
        <div className="section-title" style={{ marginTop: 0 }}>Telegram notify</div>
        <p className="page-sub">Panel bot (not TgProxy). Login alerts via api.telegram.org.</p>
        <div className="field">
          <label className="label" style={{ display: 'flex', alignItems: 'center', gap: 8, textTransform: 'none', letterSpacing: 0 }}>
            <input
              type="checkbox"
              checked={truthy(settings.tgBotEnable)}
              onChange={(e) => setSettings({ ...settings, tgBotEnable: e.target.checked ? 'true' : 'false' })}
            />
            {label('tgBotEnable')}
          </label>
        </div>
        <div className="field">
          <label className="label">{label('tgBotToken')}</label>
          <input
            className="input"
            value={settings.tgBotToken || ''}
            onChange={(e) => setSettings({ ...settings, tgBotToken: e.target.value })}
            placeholder="123456:ABC…"
          />
        </div>
        <div className="field">
          <label className="label">{label('tgBotChatId')}</label>
          <input
            className="input"
            value={settings.tgBotChatId || ''}
            onChange={(e) => setSettings({ ...settings, tgBotChatId: e.target.value })}
            placeholder="123456789"
          />
        </div>
        <div className="field">
          <label className="label" style={{ display: 'flex', alignItems: 'center', gap: 8, textTransform: 'none', letterSpacing: 0 }}>
            <input
              type="checkbox"
              checked={truthy(settings.tgNotifyLogin)}
              onChange={(e) => setSettings({ ...settings, tgNotifyLogin: e.target.checked ? 'true' : 'false' })}
            />
            {label('tgNotifyLogin')}
          </label>
        </div>
        <div className="field">
          <label className="label" style={{ display: 'flex', alignItems: 'center', gap: 8, textTransform: 'none', letterSpacing: 0 }}>
            <input
              type="checkbox"
              checked={truthy(settings.tgNotifyTraffic)}
              onChange={(e) => setSettings({ ...settings, tgNotifyTraffic: e.target.checked ? 'true' : 'false' })}
            />
            {label('tgNotifyTraffic')}
          </label>
        </div>
        <div className="toolbar">
          <button className="btn" type="button" onClick={save}>{tr('save')}</button>
          <button className="btn secondary btn-sm" type="button" disabled={tgBusy} onClick={() => { void testTelegram() }}>
            {tgBusy ? '…' : 'Test Telegram'}
          </button>
        </div>
      </div>

      <div className="card" style={{ marginTop: 12 }}>
        <div className="section-title" style={{ marginTop: 0 }}>Email notify</div>
        <p className="page-sub">SMTP login alerts via net/smtp.</p>
        <div className="field">
          <label className="label" style={{ display: 'flex', alignItems: 'center', gap: 8, textTransform: 'none', letterSpacing: 0 }}>
            <input
              type="checkbox"
              checked={truthy(settings.emailEnable)}
              onChange={(e) => setSettings({ ...settings, emailEnable: e.target.checked ? 'true' : 'false' })}
            />
            {label('emailEnable')}
          </label>
        </div>
        <div className="section-title">SMTP</div>
        <div className="grid2">
          <div className="field">
            <label className="label">{label('smtpHost')}</label>
            <input className="input" value={settings.smtpHost || ''} onChange={(e) => setSettings({ ...settings, smtpHost: e.target.value })} placeholder="smtp.example.com" />
          </div>
          <div className="field">
            <label className="label">{label('smtpPort')}</label>
            <input className="input" value={settings.smtpPort || '587'} onChange={(e) => setSettings({ ...settings, smtpPort: e.target.value })} />
          </div>
          <div className="field">
            <label className="label">{label('smtpUser')}</label>
            <input className="input" value={settings.smtpUser || ''} onChange={(e) => setSettings({ ...settings, smtpUser: e.target.value })} />
          </div>
          <div className="field">
            <label className="label">{label('smtpPass')}</label>
            <input className="input" type="password" value={settings.smtpPass || ''} onChange={(e) => setSettings({ ...settings, smtpPass: e.target.value })} placeholder="***" />
          </div>
          <div className="field" style={{ gridColumn: '1 / -1' }}>
            <label className="label">{label('smtpFrom')}</label>
            <input className="input" value={settings.smtpFrom || ''} onChange={(e) => setSettings({ ...settings, smtpFrom: e.target.value })} placeholder="panel@example.com" />
          </div>
        </div>
        <div className="field">
          <label className="label" style={{ display: 'flex', alignItems: 'center', gap: 8, textTransform: 'none', letterSpacing: 0 }}>
            <input
              type="checkbox"
              checked={truthy(settings.emailNotifyLogin)}
              onChange={(e) => setSettings({ ...settings, emailNotifyLogin: e.target.checked ? 'true' : 'false' })}
            />
            {label('emailNotifyLogin')}
          </label>
        </div>
        <div className="toolbar">
          <button className="btn" type="button" onClick={save}>{tr('save')}</button>
          <button className="btn secondary btn-sm" type="button" disabled={emailBusy} onClick={() => { void testEmail() }}>
            {emailBusy ? '…' : 'Test Email'}
          </button>
        </div>
      </div>

      <div className="card" style={{ marginTop: 12 }}>
        <div className="section-title" style={{ marginTop: 0 }}>Discord notify</div>
        <p className="page-sub">Webhook must be https://discord.com/api/webhooks/…</p>
        <div className="field">
          <label className="label" style={{ display: 'flex', alignItems: 'center', gap: 8, textTransform: 'none', letterSpacing: 0 }}>
            <input
              type="checkbox"
              checked={truthy(settings.discordEnable)}
              onChange={(e) => setSettings({ ...settings, discordEnable: e.target.checked ? 'true' : 'false' })}
            />
            {label('discordEnable')}
          </label>
        </div>
        <div className="field">
          <label className="label">{label('discordWebhook')}</label>
          <input
            className="input"
            value={settings.discordWebhook || ''}
            onChange={(e) => setSettings({ ...settings, discordWebhook: e.target.value })}
            placeholder="https://discord.com/api/webhooks/…"
          />
        </div>
        <div className="field">
          <label className="label" style={{ display: 'flex', alignItems: 'center', gap: 8, textTransform: 'none', letterSpacing: 0 }}>
            <input
              type="checkbox"
              checked={truthy(settings.discordNotifyLogin)}
              onChange={(e) => setSettings({ ...settings, discordNotifyLogin: e.target.checked ? 'true' : 'false' })}
            />
            {label('discordNotifyLogin')}
          </label>
        </div>
        <div className="toolbar">
          <button className="btn" type="button" onClick={save}>{tr('save')}</button>
          <button className="btn secondary btn-sm" type="button" disabled={discordBusy} onClick={() => { void testDiscord() }}>
            {discordBusy ? '…' : 'Test Discord'}
          </button>
        </div>
      </div>

      <div className="card" style={{ marginTop: 12 }}>
        <div className="section-title" style={{ marginTop: 0 }}>Sub balancers</div>
        <p className="page-sub">Clash/JSON proxy-groups (url-test over all proxies in matching sub).</p>
        <div className="grid2">
          <div className="field">
            <label className="label">Name</label>
            <input className="input" value={balForm.name} onChange={(e) => setBalForm({ ...balForm, name: e.target.value })} />
          </div>
          <div className="field">
            <label className="label">Strategy</label>
            <select className="select" value={balForm.strategy} onChange={(e) => setBalForm({ ...balForm, strategy: e.target.value })}>
              <option value="url-test">url-test</option>
              <option value="fallback">fallback</option>
              <option value="round-robin">round-robin</option>
            </select>
          </div>
          <div className="field" style={{ gridColumn: '1 / -1' }}>
            <label className="label">Selector (csv emails or inbound tags; empty = all)</label>
            <input className="input" value={balForm.selector} onChange={(e) => setBalForm({ ...balForm, selector: e.target.value })} />
          </div>
          <div className="field">
            <label className="label">{tr('remark')}</label>
            <input className="input" value={balForm.remark} onChange={(e) => setBalForm({ ...balForm, remark: e.target.value })} />
          </div>
        </div>
        <button className="btn btn-sm" type="button" onClick={() => { void createBalancer() }}>{tr('create')}</button>
        <table className="table" style={{ marginTop: 12 }}>
          <thead>
            <tr><th>Name</th><th>Strategy</th><th>Selector</th><th>{tr('status')}</th><th>{tr('actions')}</th></tr>
          </thead>
          <tbody>
            {balancers.length === 0 && <tr><td colSpan={5}>{tr('empty')}</td></tr>}
            {balancers.map((b) => (
              <tr key={b.id}>
                <td>{b.name}</td>
                <td>{b.strategy}</td>
                <td style={{ maxWidth: 180, overflow: 'hidden', textOverflow: 'ellipsis' }}>{b.selector || '—'}</td>
                <td>
                  <button className="btn btn-sm secondary" type="button" onClick={() => { void toggleBalancer(b) }}>
                    {b.enable ? tr('enable') : tr('disable')}
                  </button>
                </td>
                <td>
                  <button className="btn btn-sm danger" type="button" onClick={() => setConfirmBalId(b.id)}>{tr('delete')}</button>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>

      <div className="card" style={{ marginTop: 12 }}>
        <div className="section-title" style={{ marginTop: 0 }}>Password</div>
        <div className="grid2">
          <div className="field">
            <label className="label">Current password</label>
            <input className="input" type="password" value={pw.oldPassword} onChange={(e) => setPw({ ...pw, oldPassword: e.target.value })} />
          </div>
          <div className="field">
            <label className="label">New password</label>
            <input className="input" type="password" value={pw.newPassword} onChange={(e) => setPw({ ...pw, newPassword: e.target.value })} />
          </div>
        </div>
        <button className="btn secondary" onClick={changePassword}>{tr('save')}</button>
      </div>

      <ConfirmModal
        open={confirmBalId != null}
        title={tr('confirmDeleteTitle')}
        message={tr('confirmDeleteBalancer')}
        danger
        onCancel={() => setConfirmBalId(null)}
        onConfirm={() => {
          const id = confirmBalId
          setConfirmBalId(null)
          if (id != null) void deleteBalancer(id)
        }}
      />
    </div>
  )
}
