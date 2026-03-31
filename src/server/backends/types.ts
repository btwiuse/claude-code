import type { ChildProcess } from 'child_process'

/**
 * A SessionBackend is responsible for spawning and managing the lifecycle of
 * individual Claude Code CLI processes. Different backends can enforce
 * varying levels of isolation (e.g. bare process, container, VM).
 */
export interface SessionBackend {
  /**
   * Spawn a new CLI session that communicates over the given sdkUrl.
   *
   * @param opts.sessionId  Unique identifier for this session.
   * @param opts.sdkUrl     WebSocket URL the subprocess should connect back to.
   * @param opts.cwd        Working directory for the subprocess.
   * @param opts.dangerouslySkipPermissions  Whether to skip permission prompts.
   * @returns The spawned child process.
   */
  spawn(opts: {
    sessionId: string
    sdkUrl: string
    cwd: string
    dangerouslySkipPermissions?: boolean
  }): ChildProcess
}
