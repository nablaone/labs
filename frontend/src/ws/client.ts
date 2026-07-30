import { useUIStore, type UIEvent } from '../store/uiStore'

let ws: WebSocket | null = null
let reconnectTimer: ReturnType<typeof setTimeout> | null = null

function connect() {
  const proto = window.location.protocol === 'https:' ? 'wss' : 'ws'
  const url = `${proto}://${window.location.host}/ws`
  ws = new WebSocket(url)

  ws.onopen = () => {
    console.log('[ws] connected')
    if (reconnectTimer) {
      clearTimeout(reconnectTimer)
      reconnectTimer = null
    }
    // Fetch current UI state in case we missed events while disconnected
    fetch('/ui')
      .then(r => r.json())
      .then((components: any[]) => useUIStore.getState().seedComponents(components))
      .catch(e => console.warn('[ws] failed to seed UI state', e))
  }

  ws.onmessage = (evt) => {
    try {
      const event: UIEvent = JSON.parse(evt.data)
      useUIStore.getState().applyEvent(event)
    } catch (e) {
      console.warn('[ws] unparseable message', e)
    }
  }

  ws.onclose = () => {
    console.log('[ws] disconnected — reconnecting in 2s')
    reconnectTimer = setTimeout(connect, 2000)
  }

  ws.onerror = (e) => {
    console.error('[ws] error', e)
  }
}

export function initWS() {
  connect()
}

export function wsReady() {
  return ws?.readyState === WebSocket.OPEN
}

export function sendAction(type: string, payload: Record<string, unknown>) {
  fetch('/action', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ type, payload }),
  }).catch(e => console.warn('[action] failed', e))
}
