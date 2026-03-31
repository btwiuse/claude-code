/**
 * Parse a cc:// or cc+unix:// URL into server connection parameters.
 *
 * Supported formats:
 *   cc://host:port                     → http://host:port
 *   cc://host:port?token=<tok>         → http://host:port, authToken=<tok>
 *   cc+unix:///path/to/socket          → http+unix:///path/to/socket
 *   cc+unix:///path/to/socket?token=T  → http+unix:///path/to/socket, authToken=T
 */
export function parseConnectUrl(raw: string): {
  serverUrl: string
  authToken?: string
} {
  // cc+unix:// → treat the path portion as a unix socket
  if (raw.startsWith('cc+unix://')) {
    const withoutScheme = raw.slice('cc+unix://'.length)
    const qIdx = withoutScheme.indexOf('?')
    const socketPath = qIdx === -1 ? withoutScheme : withoutScheme.slice(0, qIdx)
    const params = qIdx === -1 ? '' : withoutScheme.slice(qIdx + 1)
    const token = new URLSearchParams(params).get('token') ?? undefined
    return {
      serverUrl: `http+unix://${socketPath}`,
      authToken: token,
    }
  }

  // cc://host:port[/path][?token=...]
  if (raw.startsWith('cc://')) {
    const asHttp = raw.replace(/^cc:\/\//, 'http://')
    const url = new URL(asHttp)
    const token = url.searchParams.get('token') ?? undefined
    // Remove token from URL so it doesn't leak into the server URL
    url.searchParams.delete('token')
    // Construct clean server URL (protocol + host + pathname without trailing slash)
    const pathname = url.pathname === '/' ? '' : url.pathname.replace(/\/$/, '')
    const serverUrl = `http://${url.host}${pathname}`
    return {
      serverUrl,
      authToken: token,
    }
  }

  throw new Error(`Unsupported connect URL scheme: ${raw}`)
}
