import { useEffect, useState } from 'react'
import { ConfigService } from '../../bindings/gix/internal/app'

const KEY_OPTIONS = ['Space', 'Escape', 'Tab', 'Enter']

export function SettingsView({ onClose }: { onClose: () => void }) {
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

  return (
    <div className="flex h-full flex-col bg-bg p-4 text-fg font-mono gap-3 overflow-y-auto">

      <label className="flex items-center gap-2">
        <span className="w-52 shrink-0">Tema</span>
        <select
          className="bg-surface rounded-card px-2 py-1 flex-1"
          value={cfg.theme}
          onChange={(e) => set('theme', e.target.value)}
        >
          <option value="light">Claro</option>
          <option value="dark">Escuro</option>
        </select>
      </label>

      <label className="flex items-center gap-2">
        <span className="w-52 shrink-0">Idioma</span>
        <select
          className="bg-surface rounded-card px-2 py-1 flex-1"
          value={cfg.language}
          onChange={(e) => set('language', e.target.value)}
        >
          <option value="pt">Português</option>
          <option value="en">English</option>
        </select>
      </label>

      <label className="flex items-center gap-2">
        <span className="w-52 shrink-0">Modelo</span>
        <select
          className="bg-surface rounded-card px-2 py-1 flex-1"
          value={cfg.model}
          onChange={(e) => set('model', e.target.value)}
        >
          {models.map((m) => (
            <option key={m} value={m}>{m}</option>
          ))}
        </select>
      </label>

      <label className="flex items-center gap-2">
        <span className="w-52 shrink-0">Tecla para abrir</span>
        <select
          className="bg-surface rounded-card px-2 py-1 flex-1"
          value={cfg.open_key}
          onChange={(e) => set('open_key', e.target.value)}
        >
          {KEY_OPTIONS.map((k) => (
            <option key={k} value={k}>{k}</option>
          ))}
        </select>
      </label>

      <label className="flex items-center gap-2">
        <span className="w-52 shrink-0">Intervalo para abrir (ms)</span>
        <input
          type="number"
          className="bg-surface rounded-card px-2 py-1 flex-1"
          value={cfg.open_interval_ms}
          onChange={(e) => {
            const n = parseInt(e.target.value, 10)
            set('open_interval_ms', isNaN(n) ? cfg.open_interval_ms : n)
          }}
        />
      </label>

      <label className="flex items-center gap-2">
        <span className="w-52 shrink-0">Tecla para fechar</span>
        <select
          className="bg-surface rounded-card px-2 py-1 flex-1"
          value={cfg.close_key}
          onChange={(e) => set('close_key', e.target.value)}
        >
          {KEY_OPTIONS.map((k) => (
            <option key={k} value={k}>{k}</option>
          ))}
        </select>
      </label>

      <label className="flex items-center gap-2">
        <span className="w-52 shrink-0">Intervalo para fechar (ms)</span>
        <input
          type="number"
          className="bg-surface rounded-card px-2 py-1 flex-1"
          value={cfg.close_interval_ms}
          onChange={(e) => {
            const n = parseInt(e.target.value, 10)
            set('close_interval_ms', isNaN(n) ? cfg.close_interval_ms : n)
          }}
        />
      </label>

      <label className="flex flex-col gap-1">
        <span>Chave da API</span>
        <input
          type="password"
          className="bg-surface rounded-card px-2 py-1"
          value={cfg.api_key}
          onChange={(e) => set('api_key', e.target.value)}
        />
      </label>

      <label className="flex flex-col gap-1">
        <span>Prompt de sistema</span>
        <textarea
          className="bg-surface rounded-card px-2 py-1 resize-none"
          rows={4}
          value={cfg.system_prompt}
          onChange={(e) => set('system_prompt', e.target.value)}
        />
      </label>

      <div className="flex gap-2 pt-1">
        <button
          className="bg-accent text-bg rounded-card px-3 py-1"
          onClick={save}
        >
          Salvar
        </button>
        <button
          className="bg-surface rounded-card px-3 py-1"
          onClick={onClose}
        >
          Cancelar
        </button>
      </div>

    </div>
  )
}
