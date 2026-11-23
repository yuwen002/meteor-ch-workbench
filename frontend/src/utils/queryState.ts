export function shouldCancelPrevious(currentQueryId: string | null | undefined): string | null {
  const id = (currentQueryId ?? '').trim()
  return id === '' ? null : id
}

export function reconcileQueryIds(
  pending: readonly string[] | null | undefined,
  currentQueryId: string | null | undefined,
): { cancel: string[]; keep: string[] } {
  const current = (currentQueryId ?? '').trim()
  const cancel: string[] = []
  const keep: string[] = []
  for (const raw of pending ?? []) {
    const id = (raw ?? '').trim()
    if (id === '') continue
    if (current !== '' && id === current) {
      keep.push(id)
    } else {
      cancel.push(id)
    }
  }
  return { cancel, keep }
}

export function isStaleResult(
  queryId: string | null | undefined,
  currentQueryId: string | null | undefined,
  pending: readonly string[] | null | undefined,
): boolean {
  const resultId = (queryId ?? '').trim()
  if (resultId === '') return true
  const current = (currentQueryId ?? '').trim()
  if (current === '' || resultId !== current) return true
  return !(pending ?? []).some((id) => (id ?? '').trim() === resultId)
}

export function addPending(pending: readonly string[] | null | undefined, queryId: string): string[] {
  const id = (queryId ?? '').trim()
  if (id === '') return [...(pending ?? [])]
  const list = [...(pending ?? [])]
  if (!list.includes(id)) list.push(id)
  return list
}

export function removePending(pending: readonly string[] | null | undefined, queryId: string | null | undefined): string[] {
  const id = (queryId ?? '').trim()
  return [...(pending ?? [])].filter((item) => (item ?? '').trim() !== id)
}