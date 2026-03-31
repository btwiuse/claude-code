import React from 'react'
import { Text } from '../../ink.js'
import { homedir } from 'os'
import { join } from 'path'

type Props = {
  defaultDir: string
  onInstalled: (dir: string) => void
  onCancel: () => void
  onError: (message: string) => void
}

export function NewInstallWizard(props: Props) {
  // Stub: not yet restored. Auto-cancel so callers unblock.
  React.useEffect(() => {
    props.onCancel()
  }, [props.onCancel])
  return <Text>NewInstallWizard (not yet restored)</Text>
}

export async function computeDefaultInstallDir(): Promise<string> {
  return join(homedir(), '.claude', 'assistant')
}
