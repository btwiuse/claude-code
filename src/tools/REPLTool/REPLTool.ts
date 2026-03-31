import type { Tool } from '../../Tool.js'
import { REPL_TOOL_NAME } from './constants.js'

// Stub: REPLTool is an ant-only tool not yet restored.
export const REPLTool: Tool = {
  name: REPL_TOOL_NAME,
  async description() {
    return 'REPL tool (not yet restored)'
  },
  async prompt() {
    return ''
  },
  isReadOnly() {
    return false
  },
  isEnabled() {
    return false
  },
  userFacingName() {
    return 'REPL'
  },
  async validateInput() {
    return { result: false, message: 'REPLTool is not yet restored' }
  },
  inputJSONSchema: { type: 'object' as const, properties: {} },
  async *call() {
    yield { type: 'result' as const, resultForAssistant: 'REPLTool is not yet restored', data: undefined }
  },
}
