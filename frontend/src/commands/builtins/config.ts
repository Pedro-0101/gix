import { tr } from '../../i18n'
import type { Command } from '../types'
import { CONFIG_FIELDS, fieldChoices, findField } from './config-fields'

// /config drives a two-step interactive flow: pick a setting, then pick or type
// its value. The field map (config-fields.ts) is the pure, tested core; this
// command is just the glue that sequences the cards through the context.
export const configCommand: Command = {
  name: 'config',
  aliases: ['configuracoes', 'settings'],
  descriptionKey: 'cmd_config_desc',
  run: async (ctx) => {
    const key = await ctx.choose({ title: tr(ctx.lang, 'cfg_which'), choices: fieldChoices(ctx.lang) })
    if (!key) return
    const field = findField(key)
    if (!field) return

    let value: string | null
    if (field.kind === 'enum') {
      const models = field.key === 'model' ? await ctx.config.models() : []
      value = await ctx.choose({ title: tr(ctx.lang, field.labelKey), choices: field.choices({ lang: ctx.lang, models }) })
    } else {
      value = await ctx.prompt({
        title: tr(ctx.lang, field.labelKey),
        validate: field.kind === 'number'
          ? (v) => { const k = field.validate(v); return k ? tr(ctx.lang, k) : null }
          : undefined,
      })
    }
    if (value == null) return

    await ctx.config.apply(field.key, field.kind === 'number' ? Number(value) : value)
    ctx.emitSystemMessage(`${tr(ctx.lang, 'cfg_saved')} **${tr(ctx.lang, field.labelKey)}**`)
  },
}

// Re-exported so callers that build the first card (or tests) need only this module.
export { CONFIG_FIELDS }
