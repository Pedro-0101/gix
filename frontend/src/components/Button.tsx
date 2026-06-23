import { forwardRef } from 'react'
import type { ButtonHTMLAttributes } from 'react'

type Variant = 'accent' | 'surface' | 'ghost'

type Props = ButtonHTMLAttributes<HTMLButtonElement> & {
  variant?: Variant
  /** Disables the scale-on-press feedback when motion would distract. */
  static?: boolean
}

const base =
  'inline-flex items-center justify-center gap-1.5 rounded-field text-sm font-medium ' +
  'cursor-pointer select-none outline-none ' +
  'transition-[scale,box-shadow,background-color,color,opacity] duration-150 ease-out ' +
  'focus-visible:shadow-[0_0_0_2px_var(--color-bg),0_0_0_4px_var(--ring-focus)] ' +
  'disabled:opacity-50 disabled:cursor-not-allowed'

const variants: Record<Variant, string> = {
  accent:
    'bg-accent text-white px-3.5 py-1.5 shadow-[var(--shadow-border)] ' +
    'hover:brightness-110 hover:shadow-[var(--shadow-border-hover)]',
  surface:
    'bg-surface text-fg px-3.5 py-1.5 shadow-[var(--shadow-border)] ' +
    'hover:shadow-[var(--shadow-border-hover)]',
  ghost:
    'text-muted px-2.5 py-1.5 hover:bg-surface hover:text-fg',
}

const tapScale = 'active:not-disabled:scale-[0.96]'

export const Button = forwardRef<HTMLButtonElement, Props>(function Button(
  { variant = 'surface', static: isStatic, className = '', children, ...props },
  ref,
) {
  return (
    <button
      ref={ref}
      className={[base, variants[variant], isStatic ? '' : tapScale, className].join(' ')}
      {...props}
    >
      {children}
    </button>
  )
})
