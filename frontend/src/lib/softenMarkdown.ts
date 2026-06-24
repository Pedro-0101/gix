// Amacia o prefixo visível durante o streaming: esconde marcadores inline
// ainda não fechados para o ReactMarkdown não exibir símbolos parciais.
// Deixa * / _ de um caractere e fences de bloco literais de propósito
// (ver docs/superpowers/specs/2026-06-24-reveal-texto-suave-design.md).

// Remove a última ocorrência ímpar (sem par) do token, preservando o texto.
function hideUnpaired(s: string, tok: string): string {
  const idxs: number[] = []
  let i = s.indexOf(tok)
  while (i !== -1) {
    idxs.push(i)
    i = s.indexOf(tok, i + tok.length)
  }
  if (idxs.length % 2 === 1) {
    const last = idxs[idxs.length - 1]
    return s.slice(0, last) + s.slice(last + tok.length)
  }
  return s
}

export function softenMarkdown(s: string): string {
  let out = s

  // Link parcial: [texto](url  sem ')' de fechamento → mostra só [texto].
  const linkOpen = out.lastIndexOf('](')
  if (linkOpen !== -1 && out.indexOf(')', linkOpen) === -1) {
    const bracket = out.lastIndexOf('[', linkOpen)
    if (bracket !== -1) {
      out = out.slice(0, bracket) + out.slice(bracket + 1, linkOpen)
    }
  }

  // Marcadores de dois caracteres antes dos de um (para ** não casar com *).
  for (const tok of ['**', '__', '~~']) out = hideUnpaired(out, tok)
  // Code inline (um caractere, mas seguro — não conflita com listas).
  out = hideUnpaired(out, '`')

  return out
}
