/**
 * Lightweight structured logger for the session server.
 */

export type ServerLogLevel = 'debug' | 'info' | 'warn' | 'error'

export interface ServerLogger {
  debug(msg: string, extra?: Record<string, unknown>): void
  info(msg: string, extra?: Record<string, unknown>): void
  warn(msg: string, extra?: Record<string, unknown>): void
  error(msg: string, extra?: Record<string, unknown>): void
}

const LEVEL_RANK: Record<ServerLogLevel, number> = {
  debug: 0,
  info: 1,
  warn: 2,
  error: 3,
}

export function createServerLogger(
  minLevel: ServerLogLevel = 'info',
): ServerLogger {
  const min = LEVEL_RANK[minLevel]

  function emit(
    level: ServerLogLevel,
    msg: string,
    extra?: Record<string, unknown>,
  ) {
    if (LEVEL_RANK[level] < min) return
    const ts = new Date().toISOString()
    const base: Record<string, unknown> = { ts, level, msg }
    if (extra) Object.assign(base, extra)
    const line = JSON.stringify(base)
    process.stderr.write(line + '\n')
  }

  return {
    debug: (msg, extra) => emit('debug', msg, extra),
    info: (msg, extra) => emit('info', msg, extra),
    warn: (msg, extra) => emit('warn', msg, extra),
    error: (msg, extra) => emit('error', msg, extra),
  }
}
