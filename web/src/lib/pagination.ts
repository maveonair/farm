export function pageFrom(value: unknown): number {
  if (typeof value !== 'string') {
    return 1
  }

  const page = Number(value)
  return Number.isSafeInteger(page) && page > 0 ? page : 1
}

export function pageParams(page: number, perPage: number): URLSearchParams {
  const params = new URLSearchParams({ per_page: String(perPage) })
  if (page > 1) {
    params.set('page', String(page))
  }

  return params
}
