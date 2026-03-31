import { spawn as cpSpawn, type ChildProcess } from 'child_process'
import { resolve, dirname } from 'path'
import { fileURLToPath } from 'url'
import type { SessionBackend } from './types.js'

const __dirname = dirname(fileURLToPath(import.meta.url))
/** Resolve the dev entry point relative to this module (repo root / src/dev-entry.ts). */
const DEV_ENTRY = resolve(__dirname, '..', '..', 'dev-entry.ts')

/**
 * DangerousBackend spawns Claude Code CLI processes directly on the host
 * with no sandboxing. Suitable for local development and trusted
 * single-user environments.
 *
 * The subprocess is launched with:
 *   bun run <dev-entry.ts> \
 *     -p --input-format stream-json --output-format stream-json \
 *     --sdk-url <ws://...>
 *
 * This gives us a headless, streaming-JSON Claude session whose I/O is
 * bridged through the WebSocket connection back to the server.
 */
export class DangerousBackend implements SessionBackend {
  spawn(opts: {
    sessionId: string
    sdkUrl: string
    cwd: string
    dangerouslySkipPermissions?: boolean
  }): ChildProcess {
    const args = [
      'run',
      DEV_ENTRY,
      '-p',
      '--input-format',
      'stream-json',
      '--output-format',
      'stream-json',
      '--sdk-url',
      opts.sdkUrl,
    ]

    if (opts.dangerouslySkipPermissions) {
      args.push('--dangerously-skip-permissions')
    }

    const child = cpSpawn('bun', args, {
      cwd: opts.cwd,
      stdio: ['ignore', 'pipe', 'pipe'],
      env: {
        ...process.env,
        // Prevent the child from inheriting any interactive terminal settings
        TERM: 'dumb',
      },
    })

    return child
  }
}
