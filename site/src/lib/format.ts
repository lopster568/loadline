export function formatTokens(n: number | null | undefined): string {
  if (n === null || n === undefined) return 'n/a'
  return n.toLocaleString('en-US')
}

export function formatPercent(n: number | null | undefined, digits = 1): string {
  if (n === null || n === undefined) return 'n/a'
  return `${n.toFixed(digits)}%`
}

export function formatUsd(n: number | null | undefined): string {
  if (n === null || n === undefined) return 'n/a'
  if (n < 0.01) return `$${n.toFixed(4)}`
  if (n < 1) return `$${n.toFixed(3)}`
  return `$${n.toFixed(2)}`
}
