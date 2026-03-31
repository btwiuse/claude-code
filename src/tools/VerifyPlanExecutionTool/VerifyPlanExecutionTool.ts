import type { Tool } from '../../Tool.js'
import { VERIFY_PLAN_EXECUTION_TOOL_NAME } from './constants.js'

// Stub: VerifyPlanExecutionTool is not yet restored.
export const VerifyPlanExecutionTool: Tool = {
  name: VERIFY_PLAN_EXECUTION_TOOL_NAME,
  async description() {
    return 'Verify plan execution tool (not yet restored)'
  },
  async prompt() {
    return ''
  },
  isReadOnly() {
    return true
  },
  isEnabled() {
    return false
  },
  userFacingName() {
    return 'VerifyPlanExecution'
  },
  async validateInput() {
    return { result: false, message: 'VerifyPlanExecutionTool is not yet restored' }
  },
  inputJSONSchema: { type: 'object' as const, properties: {} },
  async *call() {
    yield { type: 'result' as const, resultForAssistant: 'VerifyPlanExecutionTool is not yet restored', data: undefined }
  },
}
