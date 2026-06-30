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

// tap executa fn após a promise resolver com sucesso, retornando a mesma promise.
// Usado em services.ts para disparar side-effects pós-mutação sem alterar o retorno
// nem expandir a contagem de linhas do arquivo.
export function tap<T>(p: Promise<T>, fn: () => void): Promise<T> {
  return p.then((v) => { fn(); return v })
}

// syncAlertSchedule busca a lista via listFn e reconcilia o agendamento no SO.
// Recebe listFn por parâmetro para evitar import circular (alertSchedule ← services).
// reconcileFn é injetável para permitir testes; padrão é a função módulo reconcile.
export async function syncAlertSchedule(
  listFn: () => Promise<{ id: number; message: string; fireAt: string; status: string }[]>,
  reconcileFn: (alerts: ScheduledAlertInput[]) => Promise<void> = reconcile,
): Promise<void> {
  try {
    const alerts = await listFn()
    await reconcileFn(alerts.map((a) => ({ id: a.id, message: a.message, fireAt: a.fireAt, status: a.status })))
  } catch {
    /* best-effort */
  }
}
