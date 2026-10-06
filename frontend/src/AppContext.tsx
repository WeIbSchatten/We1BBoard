import { createContext, useContext, useEffect, useMemo, useState, type ReactNode } from 'react'
import { t, type DictKey, type Lang } from './i18n'

type Theme = 'light' | 'night' | 'amoled'
type Accent = 'blue' | 'purple'

type AppCtx = {
  lang: Lang
  setLang: (l: Lang) => void
  theme: Theme
  setTheme: (t: Theme) => void
  accent: Accent
  setAccent: (a: Accent) => void
  tr: (k: DictKey) => string
  authed: boolean
  setAuthed: (v: boolean) => void
}

const Ctx = createContext<AppCtx | null>(null)

export function AppProvider({ children }: { children: ReactNode }) {
  const [lang, setLang] = useState<Lang>((localStorage.getItem('lang') as Lang) || 'ru')
  const [theme, setTheme] = useState<Theme>((localStorage.getItem('theme') as Theme) || 'night')
  const [accent, setAccent] = useState<Accent>((localStorage.getItem('accent') as Accent) || 'blue')
  const [authed, setAuthed] = useState(false)

  useEffect(() => {
    document.documentElement.setAttribute('data-theme', theme)
    localStorage.setItem('theme', theme)
  }, [theme])

  useEffect(() => {
    document.documentElement.setAttribute('data-accent', accent)
    localStorage.setItem('accent', accent)
  }, [accent])

  useEffect(() => {
    localStorage.setItem('lang', lang)
  }, [lang])

  const value = useMemo(
    () => ({
      lang,
      setLang,
      theme,
      setTheme,
      accent,
      setAccent,
      tr: (k: DictKey) => t(lang, k),
      authed,
      setAuthed,
    }),
    [lang, theme, accent, authed],
  )

  return <Ctx.Provider value={value}>{children}</Ctx.Provider>
}

export function useApp() {
  const v = useContext(Ctx)
  if (!v) throw new Error('AppProvider missing')
  return v
}
