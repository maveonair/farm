import type { State } from '@/api/client'

export const states: State[] = ['bootstrapping', 'ready', 'running', 'cleaning', 'finished']

export const stateClasses: Record<State, string> = {
  bootstrapping: 'bg-amber-50 text-amber-700 ring-amber-600/20',
  ready: 'bg-emerald-50 text-emerald-700 ring-emerald-600/20',
  running: 'bg-blue-50 text-blue-700 ring-blue-600/20',
  cleaning: 'bg-slate-100 text-slate-700 ring-slate-600/20',
  finished: 'bg-slate-100 text-slate-500 ring-slate-500/20',
}

export function lifecycleLabel(value?: string): string {
  if (!value) {
    return '—'
  }

  return value.replace(/_/g, ' ')
}
