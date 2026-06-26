import ReactMarkdown from 'react-markdown'
import remarkGfm from 'remark-gfm'
import type { Components } from 'react-markdown'

// Renderizador de Markdown para a leitura de notas. Ao contrário do MessageCard
// (bolha de chat), aqui o conteúdo é um documento: títulos, listas, tarefas,
// tabelas e citações ganham tipografia própria, mapeada aos tokens do tema.
// Mantemos a fonte mono do app, mas com hierarquia e espaçamento reais — o
// preflight do Tailwind v4 zera os estilos default desses elementos, então cada
// um é estilizado explicitamente abaixo.
const components: Components = {
  h1: ({ children }) => <h1 className="mt-4 mb-2 text-lg font-bold first:mt-0">{children}</h1>,
  h2: ({ children }) => <h2 className="mt-4 mb-2 text-base font-bold first:mt-0">{children}</h2>,
  h3: ({ children }) => <h3 className="mt-3 mb-1.5 text-sm font-semibold first:mt-0">{children}</h3>,
  p: ({ children }) => <p className="my-2 [text-wrap:pretty] first:mt-0 last:mb-0">{children}</p>,
  ul: ({ children }) => <ul className="my-2 list-disc space-y-1 pl-5 first:mt-0 last:mb-0">{children}</ul>,
  ol: ({ children }) => <ol className="my-2 list-decimal space-y-1 pl-5 first:mt-0 last:mb-0">{children}</ol>,
  li: ({ children }) => <li className="[&>ul]:mt-1 [&>ol]:mt-1 marker:text-muted">{children}</li>,
  input: ({ checked, type }) =>
    type === 'checkbox' ? (
      <input
        type="checkbox"
        checked={checked}
        readOnly
        className="mr-1.5 -ml-5 inline-block translate-y-px accent-accent"
      />
    ) : null,
  a: ({ href, children }) => (
    <a href={href} target="_blank" rel="noreferrer" className="text-accent underline underline-offset-2">
      {children}
    </a>
  ),
  strong: ({ children }) => <strong className="font-bold">{children}</strong>,
  em: ({ children }) => <em className="italic">{children}</em>,
  del: ({ children }) => <del className="text-muted line-through">{children}</del>,
  blockquote: ({ children }) => (
    <blockquote className="my-2 border-l-2 border-accent/50 pl-3 text-muted">{children}</blockquote>
  ),
  hr: () => <hr className="my-3 border-fg/10" />,
  code: ({ className, children }) => {
    const isBlock = (className ?? '').includes('language-')
    if (isBlock) {
      return (
        <code className="block overflow-x-auto whitespace-pre rounded-field bg-surface px-3 py-2 text-[13px] shadow-[var(--shadow-border)]">
          {children}
        </code>
      )
    }
    return <code className="rounded bg-surface px-1 py-0.5 text-[13px] shadow-[var(--shadow-border)]">{children}</code>
  },
  pre: ({ children }) => <pre className="my-2">{children}</pre>,
  table: ({ children }) => (
    <div className="my-2 overflow-x-auto">
      <table className="w-full border-collapse text-sm">{children}</table>
    </div>
  ),
  th: ({ children }) => <th className="border border-fg/15 px-2 py-1 text-left font-semibold">{children}</th>,
  td: ({ children }) => <td className="border border-fg/15 px-2 py-1">{children}</td>,
}

export function Markdown({ children }: { children: string }) {
  return (
    <div className="font-mono text-sm leading-relaxed text-fg">
      <ReactMarkdown remarkPlugins={[remarkGfm]} components={components}>
        {children}
      </ReactMarkdown>
    </div>
  )
}
