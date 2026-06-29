import { useEffect, useState } from 'react'
import type { ReactNode } from 'react'
import { motion } from 'motion/react'
import { ConfigService } from '../../bindings/gix/internal/app'
import { logout, Models, Prefs } from '../api/services'
import { emitAuthError } from '../lib/events'
import { Button } from '../components/Button'
import { Slider } from '../components/Slider'
import { tr } from '../i18n'

const KEY_OPTIONS = ['Space', 'Escape', 'Tab', 'Enter']

const field =
  'w-full rounded-field bg-surface px-2.5 py-1.5 text-sm text-fg shadow-[var(--shadow-border)] outline-none ' +
  'transition-[box-shadow] duration-150 ease-out ' +
  'focus-visible:shadow-[0_0_0_1px_var(--ring-focus),0_0_0_3px_color-mix(in_srgb,var(--ring-focus)_25%,transparent)]'

// Settings reúne duas fontes: prefs locais de desktop (ConfigService — tema,
// opacidade, hotkeys, idioma, server_url) e prefs de usuário no gix-server
// (Prefs — modelo, system_prompt, note_char_limit). A api_key não aparece mais:
// vive só no servidor. O botão Salvar grava ambas as fontes.
export function SettingsView({ lang, onClose }: { lang: string; onClose: () => void }) {
  const [cfg, setCfg] = useState<any>(null)
  const [prefs, setPrefs] = useState<any>(null)
  const [models, setModels] = useState<string[]>([])

  useEffect(() => {
    ConfigService.Get().then(setCfg)
    Prefs.get().then(setPrefs).catch(() => setPrefs({}))
    Models.list().then((m) => setModels(m.map((x) => x.id))).catch(() => {})
  }, [])

  if (!cfg) return null

  const setCfgField = (k: string, v: any) => setCfg({ ...cfg, [k]: v })
  const setPrefsField = (k: string, v: any) => setPrefs({ ...prefs, [k]: v })

  const save = async () => {
    // prefs locais de desktop
    await ConfigService.Save(cfg)
    // prefs de usuário no servidor (só se carregou)
    if (prefs) {
      await Prefs.update({
        model: prefs.model,
        systemPrompt: prefs.systemPrompt,
        charLimit: prefs.charLimit,
      }).catch(() => {})
    }
    onClose()
  }

  const Row = ({ label, children, i }: { label: string; children: ReactNode; i: number }) => (
    <motion.label
      initial={{ opacity: 0, y: 8, filter: 'blur(3px)' }}
      animate={{ opacity: 1, y: 0, filter: 'blur(0px)' }}
      transition={{ duration: 0.3, ease: 'easeOut', delay: Math.min(i * 0.04, 0.3) }}
      className="flex items-center gap-3"
    >
      <span className="w-48 shrink-0 text-sm text-muted">{label}</span>
      <div className="flex-1">{children}</div>
    </motion.label>
  )

  return (
    <div className="flex flex-col text-fg">
      <div className="space-y-3 p-4">
        <motion.h1
          initial={{ opacity: 0, y: 8 }}
          animate={{ opacity: 1, y: 0 }}
          transition={{ duration: 0.3, ease: 'easeOut' }}
          className="mb-1 text-base font-semibold [text-wrap:balance]"
        >
          {tr(lang, 'settings')}
        </motion.h1>

        <Row label={tr(lang, 'theme')} i={1}>
          <select className={field} value={cfg.theme} onChange={(e) => setCfgField('theme', e.target.value)}>
            <option value="light">{tr(lang, 'light')}</option>
            <option value="dark">{tr(lang, 'dark')}</option>
          </select>
        </Row>

        <Row label={tr(lang, 'language')} i={2}>
          <select className={field} value={cfg.language} onChange={(e) => setCfgField('language', e.target.value)}>
            <option value="pt">{tr(lang, 'portuguese')}</option>
            <option value="en">{tr(lang, 'english')}</option>
          </select>
        </Row>

        <Row label={tr(lang, 'model')} i={3}>
          <select className={field} value={prefs?.model ?? ''} onChange={(e) => setPrefsField('model', e.target.value)}>
            {models.map((m) => (<option key={m} value={m}>{m}</option>))}
          </select>
        </Row>

        <Row label={tr(lang, 'opacity')} i={4}>
          <Slider
            ariaLabel={tr(lang, 'opacity')}
            value={cfg.opacity ?? 85}
            min={0}
            max={100}
            step={5}
            onChange={(v) => setCfgField('opacity', v)}
          />
        </Row>

        <Row label={tr(lang, 'open_hotkey')} i={5}>
          <select className={field} value={cfg.open_key} onChange={(e) => setCfgField('open_key', e.target.value)}>
            {KEY_OPTIONS.map((k) => (<option key={k} value={k}>{k}</option>))}
          </select>
        </Row>

        <Row label={tr(lang, 'open_interval')} i={6}>
          <Slider
            ariaLabel={tr(lang, 'open_interval')}
            value={cfg.open_interval_ms}
            min={100}
            max={2000}
            step={50}
            onChange={(v) => setCfgField('open_interval_ms', v)}
          />
        </Row>

        <Row label={tr(lang, 'open_press_count')} i={7}>
          <select className={field} value={cfg.open_press_count} onChange={(e) => setCfgField('open_press_count', Number(e.target.value))}>
            <option value={2}>2</option>
            <option value={3}>3</option>
          </select>
        </Row>

        <Row label={tr(lang, 'close_hotkey')} i={8}>
          <select className={field} value={cfg.close_key} onChange={(e) => setCfgField('close_key', e.target.value)}>
            {KEY_OPTIONS.map((k) => (<option key={k} value={k}>{k}</option>))}
          </select>
        </Row>

        <Row label={tr(lang, 'close_interval')} i={9}>
          <Slider
            ariaLabel={tr(lang, 'close_interval')}
            value={cfg.close_interval_ms}
            min={100}
            max={2000}
            step={50}
            onChange={(v) => setCfgField('close_interval_ms', v)}
          />
        </Row>

        <Row label={tr(lang, 'close_press_count')} i={10}>
          <select className={field} value={cfg.close_press_count} onChange={(e) => setCfgField('close_press_count', Number(e.target.value))}>
            <option value={2}>2</option>
            <option value={3}>3</option>
          </select>
        </Row>

        <Row label={tr(lang, 'note_char_limit')} i={11}>
          <Slider
            ariaLabel={tr(lang, 'note_char_limit')}
            value={prefs?.charLimit ?? 8000}
            min={1000}
            max={50000}
            step={1000}
            onChange={(v) => setPrefsField('charLimit', v)}
          />
        </Row>

        <Row label="Server URL" i={12}>
          <input
            type="text"
            className={`${field} font-mono`}
            value={cfg.server_url ?? ''}
            onChange={(e) => setCfgField('server_url', e.target.value)}
          />
        </Row>

        <motion.label
          initial={{ opacity: 0, y: 8, filter: 'blur(3px)' }}
          animate={{ opacity: 1, y: 0, filter: 'blur(0px)' }}
          transition={{ duration: 0.3, ease: 'easeOut', delay: 0.3 }}
          className="flex flex-col gap-1.5"
        >
          <span className="text-sm text-muted">{tr(lang, 'system_prompt')}</span>
          <textarea
            className={`${field} resize-none font-mono leading-relaxed`}
            rows={4}
            value={prefs?.systemPrompt ?? ''}
            onChange={(e) => setPrefsField('systemPrompt', e.target.value)}
          />
        </motion.label>
      </div>

      <div className="sticky bottom-0 flex items-center gap-2 border-t border-[color:var(--shell-border)] p-3">
        <Button variant="accent" onClick={save}>{tr(lang, 'save')}</Button>
        <Button variant="surface" onClick={onClose}>{tr(lang, 'cancel')}</Button>
        <Button
          variant="ghost"
          className="ml-auto"
          onClick={() => { logout(); emitAuthError() }}
        >
          {tr(lang, 'logout')}
        </Button>
      </div>
    </div>
  )
}
