import { useEffect, useRef, useCallback, useState } from 'react';
import type { ScoreUpdateMessage } from '../types/api';

const WS_BASE_URL = import.meta.env.VITE_WS_URL ||
  `${window.location.protocol === 'https:' ? 'wss:' : 'ws:'}//${window.location.host}/api/v1`;

/**
 * Custom hook for WebSocket connection to the live scoreboard.
 *
 * Features:
 * - Automatic reconnection with exponential backoff
 * - Connection state tracking
 * - Message callback for score updates
 */
export function useScoreboard(
  eventId: string,
  onUpdate: (data: ScoreUpdateMessage['data']) => void
) {
  const wsRef = useRef<WebSocket | null>(null);
  const [isConnected, setIsConnected] = useState(false);
  const reconnectTimeoutRef = useRef<number>();
  const reconnectAttempts = useRef(0);

  const connect = useCallback(() => {
    if (wsRef.current?.readyState === WebSocket.OPEN) return;

    const token = localStorage.getItem('orchi_token');
    const url = `${WS_BASE_URL}/ws/scoreboard?event_id=${eventId}&token=${token}`;

    const ws = new WebSocket(url);

    ws.onopen = () => {
      setIsConnected(true);
      reconnectAttempts.current = 0;
    };

    ws.onmessage = (event) => {
      try {
        const message: ScoreUpdateMessage = JSON.parse(event.data);
        if (message.type === 'score_update') {
          onUpdate(message.data);
        }
      } catch {
        // Ignore malformed messages
      }
    };

    ws.onclose = () => {
      setIsConnected(false);
      // Exponential backoff: 1s, 2s, 4s, 8s, max 30s
      const delay = Math.min(
        1000 * Math.pow(2, reconnectAttempts.current),
        30000
      );
      reconnectAttempts.current++;
      reconnectTimeoutRef.current = window.setTimeout(connect, delay);
    };

    ws.onerror = () => {
      ws.close();
    };

    wsRef.current = ws;
  }, [eventId, onUpdate]);

  const disconnect = useCallback(() => {
    if (reconnectTimeoutRef.current) {
      clearTimeout(reconnectTimeoutRef.current);
    }
    wsRef.current?.close();
    wsRef.current = null;
    setIsConnected(false);
  }, []);

  useEffect(() => {
    if (eventId) {
      connect();
    }
    return () => disconnect();
  }, [eventId, connect, disconnect]);

  return { isConnected, disconnect };
}
