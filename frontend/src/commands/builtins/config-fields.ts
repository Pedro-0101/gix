import { tr } from '../../i18n'
import type { Choice } from '../interaction'

// The fixed hotkey choices, mirrored from SettingsView.
export const KEY_OPTIONS = ['Space', 'Escape', 'Tab', 'Enter']

// What enum fields need to build their choices: the language (for translated
// labels) and the model list fetched from the backend.
export type FieldEnv = { lang: string; models: string[] }

// A single configuration field, described declaratively. 'enum' fields are
// picked from a list; 'text'/'number' fields are typed in the bar. Validators
// return an i18n error key, or null when the value is acceptable.
export type FieldDef =
  | { key: string; labelKey: string; kind: 'enum'; choices(env: FieldEnv): Choice[] }
  | { key: string; labelKey: string; kind: 'text' }
  | { key: string; labelKey: string; kind: 'number'; validate(v: string): string | null }

function inRange(min: number, max: number) {
  return (v: string): string | null => {
    const n = Number(v.trim())
    if (v.trim() === '' || !Number.isInteger(n) || n < min || n > max) return 'cfg_invalid_range'
    return null
  }
}

function positiveInt(v: string): string | null {
  const n = Number(v.trim())
  if (v.trim() === '' || !Number.isInteger(n) || n <= 0) return 'cfg_invalid_positive'
  return null
}

// Every field in the Config struct (internal/config/config.go), so the whole
// configuration is reachable by command. A parity test guards this list.
export const CONFIG_FIELDS: FieldDef[] = [
  { key: 'theme', labelKey: 'theme', kind: 'enum',
    choices: (e) => [{ label: tr(e.lang, 'light'), value: 'light' }, { label: tr(e.lang, 'dark'), value: 'dark' }] },
  { key: 'language', labelKey: 'language', kind: 'enum',
    choices: (e) => [{ label: tr(e.lang, 'portuguese'), value: 'pt' }, { label: tr(e.lang, 'english'), value: 'en' }] },
  { key: 'model', labelKey: 'model', kind: 'enum',
    choices: (e) => e.models.map((m) => ({ label: m, value: m })) },
  { key: 'opacity', labelKey: 'opacity', kind: 'number', validate: inRange(0, 100) },
  { key: 'open_key', labelKey: 'open_hotkey', kind: 'enum',
    choices: () => KEY_OPTIONS.map((k) => ({ label: k, value: k })) },
  { key: 'open_interval_ms', labelKey: 'open_interval', kind: 'number', validate: positiveInt },
  { key: 'close_key', labelKey: 'close_hotkey', kind: 'enum',
    choices: () => KEY_OPTIONS.map((k) => ({ label: k, value: k })) },
  { key: 'close_interval_ms', labelKey: 'close_interval', kind: 'number', validate: positiveInt },
  { key: 'api_key', labelKey: 'api_key', kind: 'text' },
  { key: 'system_prompt', labelKey: 'system_prompt', kind: 'text' },
]

// The first card's choices: one entry per field, labelled in the user's language.
export function fieldChoices(lang: string): Choice[] {
  return CONFIG_FIELDS.map((f) => ({ label: tr(lang, f.labelKey), value: f.key }))
}

export function findField(key: string): FieldDef | undefined {
  return CONFIG_FIELDS.find((f) => f.key === key)
}
