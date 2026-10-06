import { useEffect, useState } from 'react'
import { api } from '../api'
import { useApp } from '../AppContext'

type SubInfo = {
  enable: boolean
  subPort: string
  subPath: string
  subHost: string
  baseUrl: string
  formats: string[]
}

type TwoFASetup = {
  secret: string
  otpauth: string
  qr?: string
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

  useEffect(() => {
    api<Record<string, string>>('/settings').then((s) => {
      setSettings(s)
      if (s.theme) setTheme(s.theme as 'light' | 'night' | 'amoled')
      if (s.accent) setAccent(s.accent as 'blue' | 'purple')
      if (s.lang === 'ru' || s.lang === 'en') setLang(s.lang)
    }).catch(console.error)
    api<SubInfo>('/subscription').then(setSubInfo).catch(console.error)
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

  const subEnabled = truthy(settings.subEnable ?? 'true')
  const twoFAOn = truthy(settings.twoFactorEnable)

  return (
    <div>
      <h1 className="page-title">{tr('settings')}</h1>
      <p className="page-sub">Тема, акцент, панель, подписки</p>

      <div className="grid2">
        <div className="card">
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
          {(['panelPort', 'panelPath', 'webListen', 'certFile', 'keyFile'] as const).map((k) => (
            <div className="field" key={k}>
              <label className="label">{k}</label>
              <input className="input" value={settings[k] || ''} onChange={(e) => setSettings({ ...settings, [k]: e.target.value })} />
            </div>
          ))}
          <p className="page-sub">certFile / keyFile — пути к TLS сертификату панели. Для Let’s Encrypt используйте install.sh (пункт SSL): acme.sh продлевает и делает restart.</p>
          <button className="btn" onClick={save}>{tr('save')}</button>
          {msg && <p className="page-sub">{msg}</p>}
        </div>
      </div>

      <div className="card" style={{ marginTop: 12 }}>
        <h3 style={{ marginTop: 0 }}>{tr('subscription')}</h3>
        <p className="page-sub">Отдельный listener на subPort (как 3x-ui). UA auto: base64 / Clash / sing-box. Subscription-Userinfo + фильтр expiry/traffic. В проде задайте certFile/keyFile — иначе подписка отдаётся по HTTP (учётки в открытом виде).</p>
        <div className="field">
          <label className="label" style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
            <input
              type="checkbox"
              checked={subEnabled}
              onChange={(e) => setSettings({ ...settings, subEnable: e.target.checked ? 'true' : 'false' })}
            />
            subEnable
          </label>
        </div>
        {(['subPort', 'subPath', 'subHost', 'subTitle', 'subSupportUrl', 'subThemeDir', 'subAnnounce'] as const).map((k) => (
          <div className="field" key={k}>
            <label className="label">{k}</label>
            <input className="input" value={settings[k] || ''} onChange={(e) => setSettings({ ...settings, [k]: e.target.value })} placeholder={k === 'subThemeDir' ? '/etc/we1bboard/sub_templates/my-theme' : undefined} />
          </div>
        ))}
        <div className="field">
          <label className="label" style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
            <input
              type="checkbox"
              checked={truthy(settings.subJsonEnable ?? 'true')}
              onChange={(e) => setSettings({ ...settings, subJsonEnable: e.target.checked ? 'true' : 'false' })}
            />
            subJsonEnable — JSON subscription format
          </label>
        </div>
        <div className="field">
          <label className="label" style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
            <input
              type="checkbox"
              checked={truthy(settings.subClashEnable ?? 'true')}
              onChange={(e) => setSettings({ ...settings, subClashEnable: e.target.checked ? 'true' : 'false' })}
            />
            subClashEnable — Clash subscription format
          </label>
        </div>
        <div className="field">
          <label className="label" style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
            <input
              type="checkbox"
              checked={truthy(settings.ufwEnable ?? 'true')}
              onChange={(e) => setSettings({ ...settings, ufwEnable: e.target.checked ? 'true' : 'false' })}
            />
            ufwEnable — auto-open ports in UFW (panel / sub / inbounds)
          </label>
        </div>
        <p className="page-sub">В браузере URL подписки открывает HTML-страницу (копирование ссылок + QR). VPN-клиенты получают raw. Полная страница с конфигами: <code>?html=1</code>. Кастомный шаблон: абсолютный путь к папке с <code>sub.html</code> или <code>index.html</code> (как 3x-ui).</p>
        {subInfo && (
          <div className="field">
            <label className="label">Base URL</label>
            <input className="input" readOnly value={subInfo.baseUrl + '{subId}'} />
            <p className="page-sub">Форматы: {subInfo.formats.join(', ')} · порт {subInfo.subPort || '2096'}</p>
          </div>
        )}
        <button className="btn" onClick={save}>{tr('save')}</button>
      </div>

      <div className="card" style={{ marginTop: 12 }}>
        <h3 style={{ marginTop: 0 }}>Two-factor (TOTP)</h3>
        <p className="page-sub">Status: {twoFAOn ? 'enabled' : 'disabled'}</p>
        {!twoFAOn && (
          <>
            <div className="row-actions" style={{ marginBottom: 8 }}>
              <button className="btn secondary" type="button" onClick={() => { void setup2FA() }}>Setup / QR</button>
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
            <button className="btn danger" type="button" onClick={() => { void disable2FA() }}>Disable 2FA</button>
          </>
        )}
      </div>

      <div className="card" style={{ marginTop: 12 }}>
        <h3 style={{ marginTop: 0 }}>Telegram notify</h3>
        <p className="page-sub">Panel bot (not TgProxy). Login alerts via api.telegram.org.</p>
        <div className="field">
          <label className="label" style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
            <input
              type="checkbox"
              checked={truthy(settings.tgBotEnable)}
              onChange={(e) => setSettings({ ...settings, tgBotEnable: e.target.checked ? 'true' : 'false' })}
            />
            tgBotEnable
          </label>
        </div>
        <div className="field">
          <label className="label">tgBotToken</label>
          <input
            className="input"
            value={settings.tgBotToken || ''}
            onChange={(e) => setSettings({ ...settings, tgBotToken: e.target.value })}
            placeholder="123456:ABC…"
          />
        </div>
        <div className="field">
          <label className="label">tgBotChatId</label>
          <input
            className="input"
            value={settings.tgBotChatId || ''}
            onChange={(e) => setSettings({ ...settings, tgBotChatId: e.target.value })}
            placeholder="123456789"
          />
        </div>
        <div className="field">
          <label className="label" style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
            <input
              type="checkbox"
              checked={truthy(settings.tgNotifyLogin)}
              onChange={(e) => setSettings({ ...settings, tgNotifyLogin: e.target.checked ? 'true' : 'false' })}
            />
            tgNotifyLogin
          </label>
        </div>
        <div className="field">
          <label className="label" style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
            <input
              type="checkbox"
              checked={truthy(settings.tgNotifyTraffic)}
              onChange={(e) => setSettings({ ...settings, tgNotifyTraffic: e.target.checked ? 'true' : 'false' })}
            />
            tgNotifyTraffic
          </label>
        </div>
        <div className="row-actions">
          <button className="btn" type="button" onClick={save}>{tr('save')}</button>
          <button className="btn secondary" type="button" disabled={tgBusy} onClick={() => { void testTelegram() }}>
            {tgBusy ? '…' : 'Test Telegram'}
          </button>
        </div>
      </div>

      <div className="card" style={{ marginTop: 12 }}>
        <h3 style={{ marginTop: 0 }}>Password</h3>
        <div className="grid2">
          <div className="field">
            <label className="label">Old</label>
            <input className="input" type="password" value={pw.oldPassword} onChange={(e) => setPw({ ...pw, oldPassword: e.target.value })} />
          </div>
          <div className="field">
            <label className="label">New</label>
            <input className="input" type="password" value={pw.newPassword} onChange={(e) => setPw({ ...pw, newPassword: e.target.value })} />
          </div>
        </div>
        <button className="btn secondary" onClick={changePassword}>{tr('save')}</button>
      </div>
    </div>
  )
}
