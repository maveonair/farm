import type { State } from '@/api/client'

export const states: State[] = ['bootstrapping', 'ready', 'running', 'cleaning', 'finished']

export const stateClasses: Record<State, string> = {
  bootstrapping:
    'bg-amber-50 text-amber-700 ring-amber-600/20 dark:bg-amber-400/10 dark:text-amber-300 dark:ring-amber-400/30',
  ready:
    'bg-emerald-50 text-emerald-700 ring-emerald-600/20 dark:bg-emerald-400/10 dark:text-emerald-300 dark:ring-emerald-400/30',
  running:
    'bg-blue-50 text-blue-700 ring-blue-600/20 dark:bg-blue-400/10 dark:text-blue-300 dark:ring-blue-400/30',
  cleaning:
    'bg-zinc-100 text-zinc-700 ring-zinc-600/20 dark:bg-zinc-800 dark:text-zinc-300 dark:ring-zinc-400/30',
  finished:
    'bg-zinc-100 text-zinc-500 ring-zinc-500/20 dark:bg-zinc-800 dark:text-zinc-400 dark:ring-zinc-400/30',
}

export function lifecycleLabel(value?: string): string {
  if (!value) {
    return '—'
  }

  return value.replace(/_/g, ' ')
}
