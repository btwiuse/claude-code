/**
 * HTTP + WebSocket server that implements the --sdk-url protocol.
 *
 * Endpoints:
 *   POST /sessions                              → Create a new session
 *   GET  /sessions                              → List active sessions
 *   DELETE /sessions/:id                        → Destroy a session
 *   GET  /v2/session_ingress/ws/:id  (WS)      → Internal: CLI subprocess connects here
 *   GET  /ws/:id                     (WS)      → External: SDK clients connect here
 *   GET  /health                                → Health check
 *
 * The server bridges messages between external SDK clients and
 * internal CLI subprocesses via WebSocket connections.
 */

import type { Server } from 'bun'
import type { ServerConfig } from './types.js'
import type { ServerLogger } from './serverLog.js'
import type { SessionManager } from './sessionManager.js'

type WebSocketData = {
  sessionId: string
  kind: 'internal' | 'external'
  /** Stable wrapper object registered with SessionManager so we can remove it on close. */
  wrapper?: { send: (data: string) => void; close: () => void }
}

export function startServer(
  config: ServerConfig,
  sessionManager: SessionManager,
  logger: ServerLogger,
): Server {
  const server = Bun.serve<WebSocketData>({
    port: config.port,
    hostname: config.host,
    unix: config.unix,

    fetch(req, server) {
      const url = new URL(req.url)
      const path = url.pathname

      // ── Auth check (skip health endpoint) ──
      if (path !== '/health') {
        const authHeader = req.headers.get('authorization')
        const token = authHeader?.startsWith('Bearer ')
          ? authHeader.slice(7)
          : url.searchParams.get('token')
        if (config.authToken && token !== config.authToken) {
          return new Response(JSON.stringify({ error: 'Unauthorized' }), {
            status: 401,
            headers: { 'content-type': 'application/json' },
          })
        }
      }

      // ── Health check ──
      if (path === '/health') {
        return new Response(
          JSON.stringify({ status: 'ok', sessions: sessionManager.listSessions().length }),
          { headers: { 'content-type': 'application/json' } },
        )
      }

      // ── POST /sessions — create a new session ──
      if (req.method === 'POST' && path === '/sessions') {
        return handleCreateSession(req, config, sessionManager, logger, server)
      }

      // ── GET /sessions — list sessions ──
      if (req.method === 'GET' && path === '/sessions') {
        const sessions = sessionManager.listSessions().map(s => ({
          session_id: s.id,
          status: s.status,
          created_at: s.createdAt,
          work_dir: s.workDir,
        }))
        return new Response(JSON.stringify(sessions), {
          headers: { 'content-type': 'application/json' },
        })
      }

      // ── DELETE /sessions/:id — destroy a session ──
      const deleteMatch = path.match(/^\/sessions\/([^/]+)$/)
      if (req.method === 'DELETE' && deleteMatch) {
        const sessionId = deleteMatch[1]!
        const session = sessionManager.getSession(sessionId)
        if (!session) {
          return new Response(JSON.stringify({ error: 'Session not found' }), {
            status: 404,
            headers: { 'content-type': 'application/json' },
          })
        }
        void sessionManager.destroySession(sessionId)
        return new Response(JSON.stringify({ status: 'destroying' }), {
          headers: { 'content-type': 'application/json' },
        })
      }

      // ── WebSocket: internal CLI subprocess endpoint ──
      const internalMatch = path.match(
        /^\/v2\/session_ingress\/ws\/([^/]+)$/,
      )
      if (internalMatch) {
        const sessionId = internalMatch[1]!
        const session = sessionManager.getSession(sessionId)
        if (!session) {
          return new Response('Session not found', { status: 404 })
        }
        const upgraded = server.upgrade(req, {
          data: { sessionId, kind: 'internal' as const },
        })
        if (!upgraded) {
          return new Response('WebSocket upgrade failed', { status: 400 })
        }
        return undefined as unknown as Response
      }

      // ── WebSocket: external SDK client endpoint ──
      const externalMatch = path.match(/^\/ws\/([^/]+)$/)
      if (externalMatch) {
        const sessionId = externalMatch[1]!
        const session = sessionManager.getSession(sessionId)
        if (!session) {
          return new Response('Session not found', { status: 404 })
        }
        const upgraded = server.upgrade(req, {
          data: { sessionId, kind: 'external' as const },
        })
        if (!upgraded) {
          return new Response('WebSocket upgrade failed', { status: 400 })
        }
        return undefined as unknown as Response
      }

      // ── Fallback ──
      return new Response(JSON.stringify({ error: 'Not found' }), {
        status: 404,
        headers: { 'content-type': 'application/json' },
      })
    },

    websocket: {
      open(ws) {
        const { sessionId, kind } = ws.data

        // Create a stable wrapper that SessionManager can track
        const wrapper = {
          send: (data: string) => ws.send(data),
          close: () => ws.close(),
        }
        ws.data.wrapper = wrapper

        if (kind === 'internal') {
          // CLI subprocess connected back
          const registered = sessionManager.registerInternalSocket(
            sessionId,
            wrapper,
          )
          if (registered) {
            logger.info('Internal WS connected', { sessionId })
          } else {
            logger.warn('Internal WS for unknown session', { sessionId })
            ws.close(4001, 'Session not found')
          }
        } else {
          // External SDK client connected
          const registered = sessionManager.registerExternalClient(
            sessionId,
            wrapper,
          )
          if (registered) {
            logger.info('External WS connected', { sessionId })
          } else {
            logger.warn('External WS for unknown session', { sessionId })
            ws.close(4001, 'Session not found')
          }
        }
      },

      message(ws, message) {
        const { sessionId, kind } = ws.data
        const data =
          typeof message === 'string'
            ? message
            : Buffer.from(message).toString('utf-8')

        if (kind === 'internal') {
          // Message from CLI subprocess → broadcast to external clients
          sessionManager.broadcastToExternal(sessionId, data)
        } else {
          // Message from external client → forward to CLI subprocess
          sessionManager.sendToInternal(sessionId, data)
        }
      },

      close(ws) {
        const { sessionId, kind, wrapper } = ws.data

        if (kind === 'internal') {
          logger.info('Internal WS disconnected', { sessionId })
        } else if (wrapper) {
          logger.info('External WS disconnected', { sessionId })
          sessionManager.removeExternalClient(sessionId, wrapper)
        }
      },
    },
  })

  logger.info('Server started', {
    port: server.port,
    host: config.host,
    unix: config.unix,
  })

  return server
}

// ── Handler: POST /sessions ──

async function handleCreateSession(
  req: Request,
  config: ServerConfig,
  sessionManager: SessionManager,
  logger: ServerLogger,
  server: Server,
): Promise<Response> {
  let body: { cwd?: string; dangerously_skip_permissions?: boolean }
  try {
    body = await req.json()
  } catch {
    return new Response(JSON.stringify({ error: 'Invalid JSON body' }), {
      status: 400,
      headers: { 'content-type': 'application/json' },
    })
  }

  const cwd = body.cwd ?? config.workspace ?? process.cwd()
  const actualPort = server.port

  let session
  try {
    session = sessionManager.createSession({
      cwd,
      buildSdkUrl: (sessionId) =>
        `ws://127.0.0.1:${actualPort}/v2/session_ingress/ws/${sessionId}`,
      dangerouslySkipPermissions: body.dangerously_skip_permissions,
    })
  } catch (err) {
    const message = err instanceof Error ? err.message : String(err)
    logger.error('Failed to create session', { error: message })
    return new Response(JSON.stringify({ error: message }), {
      status: 503,
      headers: { 'content-type': 'application/json' },
    })
  }

  logger.info('Session created', {
    sessionId: session.id,
    cwd,
  })

  // Build the external-facing ws_url for clients to connect to
  const wsHost = config.host === '0.0.0.0' ? '127.0.0.1' : config.host
  const wsUrl = `ws://${wsHost}:${actualPort}/ws/${session.id}`

  return new Response(
    JSON.stringify({
      session_id: session.id,
      ws_url: wsUrl,
      work_dir: cwd,
    }),
    {
      status: 201,
      headers: { 'content-type': 'application/json' },
    },
  )
}
