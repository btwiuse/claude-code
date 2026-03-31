import React from 'react'
import { Text } from '../ink.js'
import type { AssistantSession } from './sessionDiscovery.js'

type Props = {
  sessions: AssistantSession[]
  onSelect: (id: string) => void
  onCancel: () => void
}

export function AssistantSessionChooser(props: Props) {
  // Stub: not yet restored. Auto-cancel so callers unblock.
  React.useEffect(() => {
    props.onCancel()
  }, [props.onCancel])
  return <Text>AssistantSessionChooser (not yet restored)</Text>
}
