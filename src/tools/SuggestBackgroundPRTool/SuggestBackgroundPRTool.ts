import type { Tool } from '../../Tool.js'

// Stub: SuggestBackgroundPRTool is an ant-only tool not yet restored.
export const SuggestBackgroundPRTool: Tool = {
  name: 'suggest_background_pr',
  async description() {
    return 'Suggest background PR tool (not yet restored)'
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
    return 'SuggestBackgroundPR'
  },
  async validateInput() {
    return { result: false, message: 'SuggestBackgroundPRTool is not yet restored' }
  },
  inputJSONSchema: { type: 'object' as const, properties: {} },
  async *call() {
    yield { type: 'result' as const, resultForAssistant: 'SuggestBackgroundPRTool is not yet restored', data: undefined }
  },
}
