/**
 * Stub: cachedMicrocompact module is not yet restored.
 * Provides the types and functions consumed by microCompact.ts for the
 * CACHED_MICROCOMPACT feature path.
 */

export type CacheEditsBlock = {
  type: 'cache_edits'
  edits: Array<{ tool_use_id: string }>
}

export type CachedMCState = {
  pinnedEdits: PinnedCacheEdits[]
  toolOrder: string[]
  registeredTools: Set<string>
  deletedRefs: Set<string>
  sentToAPI: Set<string>
}

export type PinnedCacheEdits = {
  userMessageIndex: number
  block: CacheEditsBlock
}

export function createCachedMCState(): CachedMCState {
  return {
    pinnedEdits: [],
    toolOrder: [],
    registeredTools: new Set(),
    deletedRefs: new Set(),
    sentToAPI: new Set(),
  }
}

export function markToolsSentToAPI(state: CachedMCState): void {
  for (const id of state.registeredTools) {
    state.sentToAPI.add(id)
  }
}

export function resetCachedMCState(state: CachedMCState): void {
  state.pinnedEdits = []
  state.toolOrder = []
  state.registeredTools.clear()
  state.deletedRefs.clear()
  state.sentToAPI.clear()
}

export function getCachedMCConfig(): {
  triggerThreshold: number
  keepRecent: number
} {
  return { triggerThreshold: 10, keepRecent: 3 }
}

export function registerToolResult(
  state: CachedMCState,
  toolUseId: string,
): void {
  state.registeredTools.add(toolUseId)
  state.toolOrder.push(toolUseId)
}

export function registerToolMessage(
  _state: CachedMCState,
  _groupIds: string[],
): void {
  // no-op in stub
}

export function getToolResultsToDelete(state: CachedMCState): string[] {
  const config = getCachedMCConfig()
  const sent = [...state.sentToAPI].filter(
    (id) => !state.deletedRefs.has(id),
  )
  if (sent.length <= config.triggerThreshold) {
    return []
  }
  return sent.slice(0, sent.length - config.keepRecent)
}

export function createCacheEditsBlock(
  state: CachedMCState,
  toolsToDelete: string[],
): CacheEditsBlock | null {
  if (toolsToDelete.length === 0) {
    return null
  }
  for (const id of toolsToDelete) {
    state.deletedRefs.add(id)
  }
  return {
    type: 'cache_edits',
    edits: toolsToDelete.map((id) => ({ tool_use_id: id })),
  }
}
