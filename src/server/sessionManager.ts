import type { ChildProcess } from 'child_process'
import { randomUUID } from 'crypto'
import type { SessionBackend } from './backends/types.js'
import type { SessionInfo, SessionState } from './types.js'

export type SessionManagerOptions = {
  idleTimeoutMs?: number
  maxSessions?: number
}

/**
 * SessionManager tracks active Claude Code sessions, spawns new ones
 * through a SessionBackend, and handles lifecycle events (idle timeout,
 * graceful shutdown, etc.).
 *
 * Each session has:
 *   - An internal WebSocket (subprocess → server) identified by sessionId
 *   - Zero or more external WebSocket clients that are bridged to it
 */
export class SessionManager {
  private sessions = new Map<string, SessionInfo>()
  private backend: SessionBackend
  private idleTimeoutMs: number
  private maxSessions: number
  private idleTimers = new Map<string, ReturnType<typeof setTimeout>>()

  /** Per-session list of connected external WebSocket clients */
  private externalClients = new Map<string, Set<{ send: (data: string) => void; close: () => void }>>()

  /** Per-session internal WebSocket from the CLI subprocess */
  private internalSockets = new Map<string, { send: (data: string) => void; close: () => void }>()

  /** Per-session message buffer for messages received before an external client connects */
  private messageBuffers = new Map<string, string[]>()

  constructor(backend: SessionBackend, opts: SessionManagerOptions = {}) {
    this.backend = backend
    this.idleTimeoutMs = opts.idleTimeoutMs ?? 600_000
    this.maxSessions = opts.maxSessions ?? 32
  }

  /**
   * Create a new session, spawning a CLI subprocess that connects back
   * to the server via --sdk-url.
   *
   * @param opts.cwd  Working directory for the subprocess.
   * @param opts.buildSdkUrl  Function that receives the session ID and returns
   *                          the WebSocket URL for --sdk-url.
   * @param opts.dangerouslySkipPermissions  Whether to skip permission prompts.
   */
  createSession(opts: {
    cwd: string
    buildSdkUrl: (sessionId: string) => string
    dangerouslySkipPermissions?: boolean
  }): SessionInfo {
    if (this.maxSessions > 0 && this.sessions.size >= this.maxSessions) {
      throw new Error(
        `Maximum number of sessions (${this.maxSessions}) reached`,
      )
    }

    const sessionId = randomUUID()
    const sdkUrl = opts.buildSdkUrl(sessionId)

    const child = this.backend.spawn({
      sessionId,
      sdkUrl,
      cwd: opts.cwd,
      dangerouslySkipPermissions: opts.dangerouslySkipPermissions,
    })

    const session: SessionInfo = {
      id: sessionId,
      status: 'starting' as SessionState,
      createdAt: Date.now(),
      workDir: opts.cwd,
      process: child,
    }

    this.sessions.set(sessionId, session)
    this.messageBuffers.set(sessionId, [])
    this.externalClients.set(sessionId, new Set())

    // Track subprocess lifecycle
    child.on('exit', (_code, _signal) => {
      this.markStopped(sessionId)
    })

    child.on('error', () => {
      this.markStopped(sessionId)
    })

    return session
  }

  /**
   * Register the internal WebSocket connection from a CLI subprocess.
   */
  registerInternalSocket(
    sessionId: string,
    ws: { send: (data: string) => void; close: () => void },
  ): boolean {
    const session = this.sessions.get(sessionId)
    if (!session) return false

    this.internalSockets.set(sessionId, ws)
    session.status = 'running'
    this.resetIdleTimer(sessionId)
    return true
  }

  /**
   * Register an external client WebSocket for a session.
   * Returns false if the session does not exist.
   */
  registerExternalClient(
    sessionId: string,
    ws: { send: (data: string) => void; close: () => void },
  ): boolean {
    const clients = this.externalClients.get(sessionId)
    if (!clients) return false

    clients.add(ws)
    this.resetIdleTimer(sessionId)

    // Flush any buffered messages to the new client
    const buffer = this.messageBuffers.get(sessionId)
    if (buffer) {
      for (const msg of buffer) {
        ws.send(msg)
      }
    }

    return true
  }

  /**
   * Unregister an external client WebSocket.
   */
  removeExternalClient(
    sessionId: string,
    ws: { send: (data: string) => void; close: () => void },
  ): void {
    const clients = this.externalClients.get(sessionId)
    if (clients) {
      clients.delete(ws)
      // Start idle timer if no clients remain
      if (clients.size === 0) {
        this.resetIdleTimer(sessionId)
      }
    }
  }

  /**
   * Forward a message from the CLI subprocess to all connected external clients.
   */
  broadcastToExternal(sessionId: string, data: string): void {
    const clients = this.externalClients.get(sessionId)
    if (!clients || clients.size === 0) {
      // Buffer the message if no clients are connected yet
      const buffer = this.messageBuffers.get(sessionId)
      if (buffer) {
        buffer.push(data)
        // Limit buffer size to prevent unbounded memory growth
        if (buffer.length > 5000) {
          buffer.splice(0, buffer.length - 5000)
        }
      }
      return
    }

    for (const client of clients) {
      client.send(data)
    }
  }

  /**
   * Forward a message from an external client to the CLI subprocess.
   */
  sendToInternal(sessionId: string, data: string): boolean {
    const ws = this.internalSockets.get(sessionId)
    if (!ws) return false
    ws.send(data)
    this.resetIdleTimer(sessionId)
    return true
  }

  /**
   * Get session info by ID.
   */
  getSession(sessionId: string): SessionInfo | undefined {
    return this.sessions.get(sessionId)
  }

  /**
   * List all sessions.
   */
  listSessions(): SessionInfo[] {
    return Array.from(this.sessions.values())
  }

  /**
   * Destroy a single session (kill subprocess, close sockets).
   */
  async destroySession(sessionId: string): Promise<void> {
    this.clearIdleTimer(sessionId)

    const session = this.sessions.get(sessionId)
    if (!session) return

    session.status = 'stopping'

    // Close internal socket
    const internal = this.internalSockets.get(sessionId)
    if (internal) {
      internal.close()
      this.internalSockets.delete(sessionId)
    }

    // Close all external clients
    const clients = this.externalClients.get(sessionId)
    if (clients) {
      for (const client of clients) {
        client.close()
      }
      clients.clear()
      this.externalClients.delete(sessionId)
    }

    // Kill subprocess
    if (session.process && !session.process.killed) {
      session.process.kill('SIGTERM')
      // Give it a moment to exit gracefully
      await new Promise<void>(resolve => {
        const timer = setTimeout(() => {
          if (session.process && !session.process.killed) {
            session.process.kill('SIGKILL')
          }
          resolve()
        }, 5000)
        session.process?.on('exit', () => {
          clearTimeout(timer)
          resolve()
        })
      })
    }

    this.messageBuffers.delete(sessionId)
    session.status = 'stopped'
    this.sessions.delete(sessionId)
  }

  /**
   * Destroy all sessions (used during server shutdown).
   */
  async destroyAll(): Promise<void> {
    const ids = Array.from(this.sessions.keys())
    await Promise.all(ids.map(id => this.destroySession(id)))
  }

  private markStopped(sessionId: string): void {
    const session = this.sessions.get(sessionId)
    if (session) {
      session.status = 'stopped'
      session.process = null
    }
    this.internalSockets.delete(sessionId)
    this.clearIdleTimer(sessionId)
  }

  private resetIdleTimer(sessionId: string): void {
    this.clearIdleTimer(sessionId)
    if (this.idleTimeoutMs <= 0) return

    const clients = this.externalClients.get(sessionId)
    // Only start idle timer when no external clients are connected
    if (clients && clients.size > 0) return

    const timer = setTimeout(() => {
      void this.destroySession(sessionId)
    }, this.idleTimeoutMs)
    // Don't let the idle timer prevent process exit
    if (typeof timer === 'object' && 'unref' in timer) {
      timer.unref()
    }
    this.idleTimers.set(sessionId, timer)
  }

  private clearIdleTimer(sessionId: string): void {
    const timer = this.idleTimers.get(sessionId)
    if (timer) {
      clearTimeout(timer)
      this.idleTimers.delete(sessionId)
    }
  }
}
