import React from 'react'
import { Text } from '../../ink.js'
import type { AgentMemoryScope } from '../../tools/AgentTool/agentMemory.js'

type Props = {
  agentType: string
  scope: AgentMemoryScope
  snapshotTimestamp: string
  onComplete: (result: 'merge' | 'keep' | 'replace') => void
  onCancel: () => void
}

export function SnapshotUpdateDialog(props: Props) {
  // Stub: not yet restored. Auto-cancel so callers unblock.
  React.useEffect(() => {
    props.onCancel()
  }, [props.onCancel])
  return <Text>SnapshotUpdateDialog (not yet restored)</Text>
}
