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

export function SettingsPage() {
  const { tr, theme, setTheme, accent, setAccent, lang, setLang } = useApp()
  const [settings, setSettings] = useState<Record<string, string>>({})
  const [subInfo, setSubInfo] = useState<SubInfo | null>(null)
  const [msg, setMsg] = useState('')
  const [pw, setPw] = useState({ oldPassword: '', newPassword: '' })

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

  const subEnabled = (settings.subEnable ?? 'true') === 'true' || settings.subEnable === '1'

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
        {(['subPort', 'subPath', 'subHost', 'subTitle', 'subSupportUrl'] as const).map((k) => (
          <div className="field" key={k}>
            <label className="label">{k}</label>
            <input className="input" value={settings[k] || ''} onChange={(e) => setSettings({ ...settings, [k]: e.target.value })} />
          </div>
        ))}
        <p className="page-sub">В браузере URL подписки открывает HTML-страницу (копирование ссылок + QR). VPN-клиенты получают raw. Полная страница с конфигами: добавьте <code>?html=1</code>.</p>
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
