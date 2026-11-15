import { useCallback, useEffect, useRef, useState } from 'react';
import { api, WS_URL } from '../api/client';
import type { LiveAlert } from '../api/types';

export type SocketStatus = 'connecting' | 'open' | 'reconnecting';

const MAX_ALERTS = 100;
const MAX_BACKOFF_MS = 30_000;

/**
 * Connects to /ws/alerts, keeps the newest alerts (deduped by event_id) and
 * reconnects with exponential backoff. Recent alerts are backfilled from the
 * REST API on mount so a page reload doesn't lose the feed.
 */
export function useAlertsSocket(onAlert?: (a: LiveAlert) => void) {
  const [alerts, setAlerts] = useState<LiveAlert[]>([]);
  const [status, setStatus] = useState<SocketStatus>('connecting');
  const [unread, setUnread] = useState(0);
  const onAlertRef = useRef(onAlert);
  onAlertRef.current = onAlert;

  const merge = useCallback((incoming: LiveAlert[]) => {
    setAlerts((prev) => {
      const seen = new Set(prev.map((a) => a.event_id));
      const fresh = incoming.filter((a) => !seen.has(a.event_id));
      return fresh.length ? [...fresh, ...prev].slice(0, MAX_ALERTS) : prev;
    });
  }, []);

  useEffect(() => {
    api.recentAlertEvents(30).then(merge).catch(() => {
      /* backfill is best-effort */
    });
  }, [merge]);

  useEffect(() => {
    let ws: WebSocket | null = null;
    let retry = 0;
    let timer: ReturnType<typeof setTimeout> | undefined;
    let disposed = false;

    const connect = () => {
      setStatus(retry === 0 ? 'connecting' : 'reconnecting');
      ws = new WebSocket(WS_URL);

      ws.onopen = () => {
        retry = 0;
        setStatus('open');
      };
      ws.onmessage = (ev) => {
        try {
          const alert = JSON.parse(ev.data) as LiveAlert;
          if (!alert.event_id) return;
          merge([alert]);
          setUnread((n) => n + 1);
          onAlertRef.current?.(alert);
        } catch {
          /* ignore malformed frames */
        }
      };
      ws.onclose = () => {
        if (disposed) return;
        setStatus('reconnecting');
        const delay = Math.min(1000 * 2 ** retry, MAX_BACKOFF_MS);
        retry += 1;
        timer = setTimeout(connect, delay);
      };
      ws.onerror = () => ws?.close();
    };

    connect();
    return () => {
      disposed = true;
      clearTimeout(timer);
      ws?.close();
    };
  }, [merge]);

  const markRead = useCallback(() => setUnread(0), []);
  const clear = useCallback(() => {
    setAlerts([]);
    setUnread(0);
  }, []);

  return { alerts, status, unread, markRead, clear };
}
