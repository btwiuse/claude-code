/**
 * Headless connection runner for the `claude open <cc-url>` command.
 *
 * Connects to a remote Claude Code session server, sends an optional prompt,
 * and streams the output to stdout in the requested format.
 */

import type { DirectConnectConfig } from './directConnectManager.js'
import type { StdoutMessage } from '../entrypoints/sdk/controlTypes.js'

/**
 * Run a headless session against a direct-connect server.
 *
 * @param config       Connection configuration (serverUrl, sessionId, wsUrl, authToken)
 * @param prompt       Initial user prompt to send (empty string for interactive stdin)
 * @param outputFormat Output format: "text", "json", or "stream-json"
 * @param interactive  If true, read follow-up messages from stdin
 */
export async function runConnectHeadless(
  config: DirectConnectConfig,
  prompt: string,
  outputFormat: string,
  interactive: boolean,
): Promise<void> {
  const headers: Record<string, string> = {}
  if (config.authToken) {
    headers['authorization'] = `Bearer ${config.authToken}`
  }

  const ws = new WebSocket(config.wsUrl, {
    headers,
  } as unknown as string[])

  const messages: StdoutMessage[] = []
  let exited = false

  return new Promise<void>((resolve, reject) => {
    ws.addEventListener('open', () => {
      // Send initial prompt if provided
      if (prompt) {
        const userMessage = JSON.stringify({
          type: 'user',
          message: {
            role: 'user',
            content: prompt,
          },
          parent_tool_use_id: null,
          session_id: '',
        })
        ws.send(userMessage)
      }

      // In interactive mode, pipe stdin lines as user messages
      if (interactive) {
        readStdinLines(ws)
      }
    })

    ws.addEventListener('message', (event) => {
      const data = typeof event.data === 'string' ? event.data : ''
      const lines = data.split('\n').filter((l: string) => l.trim())

      for (const line of lines) {
        let parsed: StdoutMessage
        try {
          parsed = JSON.parse(line) as StdoutMessage
        } catch {
          continue
        }

        if (outputFormat === 'stream-json') {
          process.stdout.write(line + '\n')
        } else if (outputFormat === 'json') {
          messages.push(parsed)
        } else {
          // text format — extract readable content
          writeTextOutput(parsed)
        }

        // Auto-approve all permission requests in headless mode
        if (parsed.type === 'control_request') {
          const response = JSON.stringify({
            type: 'control_response',
            response: {
              subtype: 'success',
              request_id: (parsed as { request_id: string }).request_id,
              response: { behavior: 'allow' },
            },
          })
          ws.send(response)
        }
      }
    })

    ws.addEventListener('close', () => {
      if (exited) return
      exited = true

      if (outputFormat === 'json') {
        process.stdout.write(JSON.stringify(messages, null, 2) + '\n')
      }
      resolve()
    })

    ws.addEventListener('error', () => {
      if (exited) return
      exited = true
      reject(new Error('WebSocket connection error'))
    })
  })
}

function writeTextOutput(msg: StdoutMessage): void {
  if (msg.type === 'assistant') {
    const content = (msg as { message?: { content?: unknown } }).message
      ?.content
    if (typeof content === 'string') {
      process.stdout.write(content)
    } else if (Array.isArray(content)) {
      for (const block of content) {
        if (
          typeof block === 'object' &&
          block !== null &&
          'type' in block &&
          block.type === 'text' &&
          'text' in block
        ) {
          process.stdout.write(String(block.text))
        }
      }
    }
  } else if (msg.type === 'result') {
    const result = msg as {
      result?: string
      subtype?: string
    }
    if (result.subtype === 'success' && result.result) {
      process.stdout.write(result.result + '\n')
    }
  }
}

function readStdinLines(ws: WebSocket): void {
  const decoder = new TextDecoder()
  const stdin = process.stdin
  if (!stdin.readable) return

  stdin.on('data', (chunk: Buffer) => {
    const text = decoder.decode(chunk).trim()
    if (!text) return

    if (ws.readyState === WebSocket.OPEN) {
      ws.send(
        JSON.stringify({
          type: 'user',
          message: { role: 'user', content: text },
          parent_tool_use_id: null,
          session_id: '',
        }),
      )
    }
  })

  stdin.on('end', () => {
    ws.close()
  })
}
