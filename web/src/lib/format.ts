const shortIDLength = 12

interface InstanceIdentity {
  id?: string
  name?: string
}

export function instanceLabel(instance: InstanceIdentity): string {
  return instance.name || instance.id?.slice(0, shortIDLength) || 'Unknown instance'
}

export function relative(value?: string): string {
  if (!value) {
    return 'Never'
  }
  const seconds = Math.round((new Date(value).getTime() - Date.now()) / 1000)
  const formatter = new Intl.RelativeTimeFormat(undefined, { numeric: 'auto' })
  const ranges: Array<[number, Intl.RelativeTimeFormatUnit]> = [
    [86_400, 'day'],
    [3_600, 'hour'],
    [60, 'minute'],
    [1, 'second'],
  ]
  for (const [size, unit] of ranges) {
    if (Math.abs(seconds) >= size || unit === 'second') {
      return formatter.format(Math.round(seconds / size), unit)
    }
  }
  return 'Now'
}

export function exact(value?: string): string {
  return value ? new Date(value).toLocaleString() : '—'
}

export function scopeName(scope: { type: string; owner?: string; repository?: string }): string {
  const type = scope.type.charAt(0).toUpperCase() + scope.type.slice(1)

  if (scope.type === 'repository' && scope.owner && scope.repository) {
    return `${type} · ${scope.owner}/${scope.repository}`
  }
  if (scope.type === 'organization' && scope.owner) {
    return `${type} · ${scope.owner}`
  }

  return type
}
