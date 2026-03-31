/**
 * Server lock file management.
 *
 * Prevents multiple server instances from running simultaneously by writing
 * a lock file to ~/.claude/server.lock. The lock file contains JSON with
 * pid, port, host, httpUrl and startedAt.
 */

import { promises as fs } from 'fs'
import { homedir } from 'os'
import { join } from 'path'

const CLAUDE_DIR = join(homedir(), '.claude')
const LOCK_PATH = join(CLAUDE_DIR, 'server.lock')

export type ServerLockData = {
  pid: number
  port: number
  host: string
  httpUrl: string
  startedAt: number
}

/**
 * Check whether a server lock file exists and the process is still alive.
 * Returns the lock data if the server is running, or null otherwise.
 */
export async function probeRunningServer(): Promise<ServerLockData | null> {
  try {
    const raw = await fs.readFile(LOCK_PATH, 'utf-8')
    const data: ServerLockData = JSON.parse(raw)
    // Check if the process is still alive
    try {
      process.kill(data.pid, 0)
      return data
    } catch {
      // Process not running — stale lock file
      await removeServerLock()
      return null
    }
  } catch {
    return null
  }
}

/**
 * Write the server lock file.
 */
export async function writeServerLock(data: ServerLockData): Promise<void> {
  await fs.mkdir(CLAUDE_DIR, { recursive: true })
  await fs.writeFile(LOCK_PATH, JSON.stringify(data, null, 2) + '\n', 'utf-8')
}

/**
 * Remove the server lock file.
 */
export async function removeServerLock(): Promise<void> {
  try {
    await fs.unlink(LOCK_PATH)
  } catch {
    // Ignore if already removed
  }
}
