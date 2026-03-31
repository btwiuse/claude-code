import type { ServerConfig } from './types.js'

/**
 * Print a human-readable startup banner to stderr.
 */
export function printBanner(
  config: ServerConfig,
  authToken: string,
  actualPort: number,
): void {
  const lines: string[] = []
  lines.push('')
  lines.push('╔══════════════════════════════════════════════╗')
  lines.push('║        Claude Code Session Server            ║')
  lines.push('╚══════════════════════════════════════════════╝')
  lines.push('')

  if (config.unix) {
    lines.push(`  Socket:     ${config.unix}`)
  } else {
    lines.push(`  Listening:  http://${config.host}:${actualPort}`)
  }

  lines.push(`  Auth token: ${authToken}`)

  if (config.workspace) {
    lines.push(`  Workspace:  ${config.workspace}`)
  }

  if (config.maxSessions) {
    lines.push(`  Max sessions: ${config.maxSessions}`)
  }

  if (config.idleTimeoutMs) {
    lines.push(`  Idle timeout: ${config.idleTimeoutMs}ms`)
  }

  lines.push('')
  lines.push('  Connect with:')

  if (config.unix) {
    lines.push(`    claude cc+unix://${config.unix}?token=${authToken}`)
  } else {
    const host = config.host === '0.0.0.0' ? 'localhost' : config.host
    lines.push(
      `    claude cc://${host}:${actualPort}?token=${authToken}`,
    )
  }

  lines.push('')

  process.stderr.write(lines.join('\n') + '\n')
}
