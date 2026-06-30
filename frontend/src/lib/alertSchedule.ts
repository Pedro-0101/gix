// alertSchedule centraliza o agendamento de alertas no SO (toast agendado do
// Windows). As funções de side-effect chamam o AlertSchedulerService do Wails
// via import dinâmico (no-op fora de Windows / se o binding não existir).
//
// Dedup: com o app aberto, quem dispara é o push; o handler de push chama
// cancelOne para desarmar o toast do SO daquela ocorrência. LIMITAÇÃO CONHECIDA:
// se o toast do SO disparar com o app FECHADO e o servidor reentregar a
// ocorrência por push ao reabrir, pode haver um toast duplicado — resolver isso
// exige supressão server-side (não re-empurrar ocorrências que o desktop armou),
// um follow-up fora do escopo deste recurso.

export type ScheduledAlertInput = {
  id: number
  message: string
  fireAt: string
  status: string
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
