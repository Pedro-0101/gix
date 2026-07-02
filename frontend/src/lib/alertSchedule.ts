// alertSchedule centraliza o agendamento de alertas no SO (toast agendado do
// Windows). As funções de side-effect chamam o AlertSchedulerService do Wails
// via import estático — o binding é gerado pelo wails generate e está sempre
// presente no build de desktop (o Vite o resolve via vite.config.ts).
//
// Dedup: com o app aberto, quem dispara é o push; o handler de push chama
// cancelOne para desarmar o toast do SO daquela ocorrência. LIMITAÇÃO CONHECIDA:
// se o toast do SO disparar com o app FECHADO e o servidor reentregar a
// ocorrência por push ao reabrir, pode haver um toast duplicado — resolver isso
// exige supressão server-side (não re-empurrar ocorrências que o desktop armou),
// um follow-up fora do escopo deste recurso.

import { AlertSchedulerService } from '../../bindings/gix/internal/app'

const TAG = 'alertSchedule'

function log(msg: string, ...args: unknown[]) {
  console.log(`[${TAG}] ${msg}`, ...args)
}
function err(msg: string, ...args: unknown[]) {
  console.error(`[${TAG}] ${msg}`, ...args)
}

export type ScheduledAlertInput = {
  id: number
  message: string
  fireAt: string
  status: string
}

export async function reconcile(alerts: ScheduledAlertInput[]): Promise<void> {
  log(`reconcile: called with ${alerts.length} alerts`, alerts.map(a => ({ id: a.id, status: a.status, fireAt: a.fireAt })))
  try {
    await AlertSchedulerService.Reconcile(alerts)
    log('reconcile: OK')
  } catch (e: any) {
    err('reconcile: FAILED', e?.message ?? e)
    throw e
  }
}

export async function armOne(a: ScheduledAlertInput): Promise<void> {
  log(`armOne: called id=${a.id} status=${a.status} fireAt=${a.fireAt}`)
  try {
    await AlertSchedulerService.ArmOne(a)
    log(`armOne: id=${a.id} OK`)
  } catch (e: any) {
    err(`armOne: id=${a.id} FAILED`, e?.message ?? e)
    throw e
  }
}

export async function cancelOne(alertId: number): Promise<void> {
  log(`cancelOne: called id=${alertId}`)
  try {
    await AlertSchedulerService.CancelOne(alertId)
    log(`cancelOne: id=${alertId} OK`)
  } catch (e: any) {
    err(`cancelOne: id=${alertId} FAILED`, e?.message ?? e)
    throw e
  }
}

// tap executa fn após a promise resolver com sucesso, retornando a mesma promise.
// Usado em services.ts para disparar side-effects pós-mutação sem alterar o retorno
// nem expandir a contagem de linhas do arquivo.
export function tap<T>(p: Promise<T>, fn: () => Promise<unknown> | unknown): Promise<T> {
  return p.then(async (v) => { await fn(); return v })
}

// syncAlertSchedule busca a lista via listFn e reconcilia o agendamento no SO.
// Recebe listFn por parâmetro para evitar import circular (alertSchedule ← services).
// reconcileFn é injetável para permitir testes; padrão é a função módulo reconcile.
export async function syncAlertSchedule(
  listFn: () => Promise<{ id: number; message: string; fireAt: string; status: string }[]>,
  reconcileFn: (alerts: ScheduledAlertInput[]) => Promise<void> = reconcile,
): Promise<void> {
  log('syncAlertSchedule: fetching alerts...')
  try {
    const alerts = await listFn()
    log(`syncAlertSchedule: got ${alerts.length} alerts from server`, alerts.map(a => ({ id: a.id, status: a.status, fireAt: a.fireAt })))
    await reconcileFn(alerts.map((a) => ({ id: a.id, message: a.message, fireAt: a.fireAt, status: a.status })))
    log('syncAlertSchedule: done')
  } catch (e: any) {
    err('syncAlertSchedule: FAILED', e?.message ?? e)
    throw e
  }
}
