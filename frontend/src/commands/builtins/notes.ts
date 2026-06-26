import type { Command } from '../types'

// /notes (alias /notas): abre a view de leitura das notas salvas. A captura de
// novas notas continua em /note <texto>; sem argumento, /note também cai aqui.
export const notesCommand: Command = {
  name: 'notes',
  aliases: ['notas'],
  descriptionKey: 'cmd_notes_desc',
  run: (ctx) => {
    ctx.setView('notes')
  },
}
