// alertSchedule centraliza o agendamento de alertas no SO (toast agendado do
// Windows). As funções de side-effect chamam o AlertSchedulerService do Wails
// via import dinâmico (no-op fora de Windows / se o binding não existir). O
// registro "surfaced" desduplica o push contra o toast do SO.

export type ScheduledAlertInput = {
  id: number
  message: string
  fireAt: string
  status: string
}

// keyOf identifica uma ocorrência por id + instante absoluto (segundos Unix),
// imune a fuso/representação. Casa com winnotify.Key (tag:group) no Go.
export function keyOf(alertId: number, fireAt: string): string {
  const unix = Math.floor(new Date(fireAt).getTime() / 1000)
  return `${alertId}:${unix}`
}

const surfaced = new Set<string>()

export function markSurfaced(key: string): void {
  surfaced.add(key)
}
export function wasSurfaced(key: string): boolean {
  return surfaced.has(key)
}
// _resetSurfaced é só para testes.
export function _resetSurfaced(): void {
  surfaced.clear()
}

async function svc(): Promise<any> {
  try {
    const mod: any = await import('../../bindings/gix/internal/app')
    return mod?.AlertSchedulerService ?? null
  } catch {
    return null
  }
}

export async function reconcile(alerts: ScheduledAlertInput[]): Promise<void> {
  try {
    await (await svc())?.Reconcile?.(alerts)
  } catch {
    /* best-effort: o push do servidor segue cobrindo */
  }
}

export async function armOne(a: ScheduledAlertInput): Promise<void> {
  try {
    await (await svc())?.ArmOne?.(a)
  } catch {
    /* best-effort */
  }
}

export async function cancelOne(alertId: number): Promise<void> {
  try {
    await (await svc())?.CancelOne?.(alertId)
  } catch {
    /* best-effort */
  }
}
