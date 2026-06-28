import { useEffect, useState } from 'react'
import type { ReactNode } from 'react'
import { motion } from 'motion/react'
import { ConfigService } from '../../bindings/gix/internal/app'
import { Button } from '../components/Button'
import { Slider } from '../components/Slider'
import { tr } from '../i18n'

const KEY_OPTIONS = ['Space', 'Escape', 'Tab', 'Enter']

const field =
  'w-full rounded-field bg-surface px-2.5 py-1.5 text-sm text-fg shadow-[var(--shadow-border)] outline-none ' +
  'transition-[box-shadow] duration-150 ease-out ' +
  'focus-visible:shadow-[0_0_0_1px_var(--ring-focus),0_0_0_3px_color-mix(in_srgb,var(--ring-focus)_25%,transparent)]'

export function SettingsView({ lang, onClose }: { lang: string; onClose: () => void }) {
  const [cfg, setCfg] = useState<any>(null)
  const [models, setModels] = useState<string[]>([])

  useEffect(() => {
    ConfigService.Get().then(setCfg)
    ConfigService.Models().then((m) => setModels(m ?? []))
  }, [])

  if (!cfg) return null

  const set = (k: string, v: any) => setCfg({ ...cfg, [k]: v })

  const save = async () => {
    // os campos de intervalo já são coeridos a número nos onChange
    await ConfigService.Save(cfg)
    onClose()
  }

  // A row in the form: label on the left, control on the right.
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
          <select className={field} value={cfg.theme} onChange={(e) => set('theme', e.target.value)}>
            <option value="light">{tr(lang, 'light')}</option>
            <option value="dark">{tr(lang, 'dark')}</option>
          </select>
        </Row>

        <Row label={tr(lang, 'language')} i={2}>
          <select className={field} value={cfg.language} onChange={(e) => set('language', e.target.value)}>
            <option value="pt">{tr(lang, 'portuguese')}</option>
            <option value="en">{tr(lang, 'english')}</option>
          </select>
        </Row>

        <Row label={tr(lang, 'model')} i={3}>
          <select className={field} value={cfg.model} onChange={(e) => set('model', e.target.value)}>
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
            onChange={(v) => set('opacity', v)}
          />
        </Row>

        <Row label={tr(lang, 'open_hotkey')} i={5}>
          <select className={field} value={cfg.open_key} onChange={(e) => set('open_key', e.target.value)}>
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
            onChange={(v) => set('open_interval_ms', v)}
          />
        </Row>

        <Row label={tr(lang, 'close_hotkey')} i={7}>
          <select className={field} value={cfg.close_key} onChange={(e) => set('close_key', e.target.value)}>
            {KEY_OPTIONS.map((k) => (<option key={k} value={k}>{k}</option>))}
          </select>
        </Row>

        <Row label={tr(lang, 'close_interval')} i={8}>
          <Slider
            ariaLabel={tr(lang, 'close_interval')}
            value={cfg.close_interval_ms}
            min={100}
            max={2000}
            step={50}
            onChange={(v) => set('close_interval_ms', v)}
          />
        </Row>

        <Row label={tr(lang, 'note_char_limit')} i={9}>
          <Slider
            ariaLabel={tr(lang, 'note_char_limit')}
            value={cfg.note_char_limit ?? 8000}
            min={1000}
            max={50000}
            step={1000}
            onChange={(v) => set('note_char_limit', v)}
          />
        </Row>

        <motion.label
          initial={{ opacity: 0, y: 8, filter: 'blur(3px)' }}
          animate={{ opacity: 1, y: 0, filter: 'blur(0px)' }}
          transition={{ duration: 0.3, ease: 'easeOut', delay: 0.3 }}
          className="flex flex-col gap-1.5"
        >
          <span className="text-sm text-muted">{tr(lang, 'api_key')}</span>
          <input
            type="password"
            className={`${field} font-mono`}
            value={cfg.api_key}
            onChange={(e) => set('api_key', e.target.value)}
          />
        </motion.label>

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
            value={cfg.system_prompt}
            onChange={(e) => set('system_prompt', e.target.value)}
          />
        </motion.label>
      </div>

      <div className="sticky bottom-0 flex gap-2 border-t border-[color:var(--shell-border)] p-3">
        <Button variant="accent" onClick={save}>{tr(lang, 'save')}</Button>
        <Button variant="surface" onClick={onClose}>{tr(lang, 'cancel')}</Button>
      </div>
    </div>
  )
}
