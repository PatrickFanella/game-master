import { useCallback, useEffect, useMemo, useRef, useState } from 'react';

import { API_BASE } from '../api/client';
import type {
  ActionRequest,
  StateChange,
  TurnResponse,
  WebSocketChunkPayload,
  WebSocketErrorPayload,
  WebSocketMessageEnvelope,
  WebSocketStatusPayload,
} from '../api/types';

export interface NarrativeChoice {
  id: string;
  text: string;
}

export interface TurnResponseWithChoices extends TurnResponse {
  choices: NarrativeChoice[];
}

export type ConnectionStatus = 'idle' | 'connecting' | 'open' | 'closed' | 'error';

export type ParsedWebSocketEvent =
  | {
      kind: 'chunk';
      envelope: WebSocketMessageEnvelope<WebSocketChunkPayload>;
      payload: WebSocketChunkPayload;
    }
  | {
      kind: 'result';
      envelope: WebSocketMessageEnvelope<TurnResponseWithChoices>;
      payload: TurnResponseWithChoices;
    }
  | {
      kind: 'error';
      envelope: WebSocketMessageEnvelope<WebSocketErrorPayload>;
      payload: WebSocketErrorPayload;
    }
  | {
      kind: 'status';
      envelope: WebSocketMessageEnvelope<WebSocketStatusPayload>;
      payload: WebSocketStatusPayload;
    };

export interface UseWebSocketResult {
  connectionStatus: ConnectionStatus;
  isConnected: boolean;
  isLoading: boolean;
  error: string | null;
  events: ParsedWebSocketEvent[];
  currentStatus: WebSocketStatusPayload | null;
  combatActive: boolean;
  sendAction: (input: string) => boolean;
}

type OutboundActionEnvelope = {
  type: 'action';
  payload: ActionRequest;
};

const SOCKET_UNAVAILABLE_MESSAGE = 'Live connection unavailable.';
const MALFORMED_MESSAGE_ERROR = 'Received a malformed live update from the server.';
const RECONNECT_BASE_DELAY_MS = 1000;
const RECONNECT_MAX_DELAY_MS = 30000;
const RECONNECT_MAX_ATTEMPTS = 10;

export function useWebSocket(campaignId: string | null | undefined): UseWebSocketResult {
  const [connectionStatus, setConnectionStatus] = useState<ConnectionStatus>('idle');
  const [error, setError] = useState<string | null>(null);
  const [events, setEvents] = useState<ParsedWebSocketEvent[]>([]);
  const [isAwaitingResponse, setIsAwaitingResponse] = useState(false);
  const [currentStatus, setCurrentStatus] = useState<WebSocketStatusPayload | null>(null);
  const [combatActive, setCombatActive] = useState(false);
  const [reconnectNonce, setReconnectNonce] = useState(0);
  const socketRef = useRef<WebSocket | null>(null);
  const reconnectTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const reconnectAttemptsRef = useRef(0);

  useEffect(() => {
    const activeCampaignId = campaignId?.trim() ?? '';

    if (reconnectTimerRef.current) {
      clearTimeout(reconnectTimerRef.current);
      reconnectTimerRef.current = null;
    }

    if (!activeCampaignId) {
      setEvents([]);
      setError(null);
      setIsAwaitingResponse(false);
      setCurrentStatus(null);
      setConnectionStatus('idle');
      socketRef.current = null;
      return undefined;
    }

    const socket = new WebSocket(buildCampaignWebSocketURL(activeCampaignId));
    socketRef.current = socket;
    setConnectionStatus('connecting');

    let allowReconnect = true;
    const isCurrentSocket = () => socketRef.current === socket;

    const scheduleReconnect = (nextStatus: ConnectionStatus, nextError: string | null) => {
      if (!isCurrentSocket()) {
        return;
      }

      setConnectionStatus(nextStatus);
      setError(nextError);
      setIsAwaitingResponse(false);

      if (!allowReconnect) {
        return;
      }

      reconnectAttemptsRef.current += 1;
      if (reconnectAttemptsRef.current > RECONNECT_MAX_ATTEMPTS) {
        setError('Connection lost. Refresh the page to reconnect.');
        return;
      }

      const delay = Math.min(
        RECONNECT_BASE_DELAY_MS * Math.pow(2, reconnectAttemptsRef.current - 1),
        RECONNECT_MAX_DELAY_MS,
      );

      reconnectTimerRef.current = setTimeout(() => {
        if (!allowReconnect) {
          return;
        }

        setReconnectNonce((current) => current + 1);
      }, delay);
    };

    const handleOpen = () => {
      if (!isCurrentSocket()) {
        return;
      }

      if (reconnectTimerRef.current) {
        clearTimeout(reconnectTimerRef.current);
        reconnectTimerRef.current = null;
      }

      setConnectionStatus('open');
      setError(null);
      reconnectAttemptsRef.current = 0;
    };

    const handleClose = () => {
      scheduleReconnect('closed', null);
    };

    const handleSocketError = () => {
      scheduleReconnect('error', SOCKET_UNAVAILABLE_MESSAGE);
    };

    const handleMessage = (event: MessageEvent<string>) => {
      if (!isCurrentSocket()) {
        return;
      }

      const decodedEvent = decodeWebSocketEvent(event.data);
      if (decodedEvent === null) {
        setError(MALFORMED_MESSAGE_ERROR);
        setIsAwaitingResponse(false);
        return;
      }

      setEvents((current) => [...current, decodedEvent]);

      if (decodedEvent.kind === 'status') {
        setCurrentStatus(decodedEvent.payload);
        if (decodedEvent.payload.stage === 'combat_start') {
          setCombatActive(true);
        } else if (decodedEvent.payload.stage === 'combat_end') {
          setCombatActive(false);
        }
        return;
      }

      if (decodedEvent.kind === 'chunk') {
        setError(null);
        return;
      }

      if (decodedEvent.kind === 'result') {
        setError(null);
        setCurrentStatus(null);
        setIsAwaitingResponse(false);
        if ('combat_active' in decodedEvent.payload) {
          setCombatActive(Boolean(decodedEvent.payload.combat_active));
        }
        return;
      }

      setError(decodedEvent.payload.error);
      setCurrentStatus(null);
      setIsAwaitingResponse(false);
    };

    socket.addEventListener('open', handleOpen);
    socket.addEventListener('close', handleClose);
    socket.addEventListener('error', handleSocketError);
    socket.addEventListener('message', handleMessage);

    return () => {
      allowReconnect = false;

      if (reconnectTimerRef.current) {
        clearTimeout(reconnectTimerRef.current);
        reconnectTimerRef.current = null;
      }

      socket.removeEventListener('open', handleOpen);
      socket.removeEventListener('close', handleClose);
      socket.removeEventListener('error', handleSocketError);
      socket.removeEventListener('message', handleMessage);

      if (socketRef.current === socket) {
        socketRef.current = null;
      }

      if (socket.readyState === WebSocket.OPEN || socket.readyState === WebSocket.CONNECTING) {
        socket.close(1000, 'campaign_changed');
      }
    };
  }, [campaignId, reconnectNonce]);

  const sendAction = useCallback(
    (input: string): boolean => {
      const trimmedInput = input.trim();
      if (!trimmedInput) {
        return false;
      }

      if (!(campaignId?.trim())) {
        setError(SOCKET_UNAVAILABLE_MESSAGE);
        return false;
      }

      const socket = socketRef.current;
      if (socket === null || socket.readyState !== WebSocket.OPEN) {
        setError(SOCKET_UNAVAILABLE_MESSAGE);
        return false;
      }

      const message: OutboundActionEnvelope = {
        type: 'action',
        payload: { input: trimmedInput },
      };

      try {
        socket.send(JSON.stringify(message));
        setError(null);
        setIsAwaitingResponse(true);
        return true;
      } catch {
        setError(SOCKET_UNAVAILABLE_MESSAGE);
        setIsAwaitingResponse(false);
        return false;
      }
    },
    [campaignId],
  );

  const isConnected = connectionStatus === 'open';
  const isLoading = connectionStatus === 'connecting' || isAwaitingResponse;

  return useMemo(
    () => ({
      connectionStatus,
      isConnected,
      isLoading,
      error,
      events,
      currentStatus,
      combatActive,
      sendAction,
    }),
    [combatActive, connectionStatus, currentStatus, error, events, isConnected, isLoading, sendAction],
  );
}

function buildCampaignWebSocketURL(campaignId: string): string {
  const origin = window.location.origin;
  const base = API_BASE.startsWith('http://') || API_BASE.startsWith('https://') ? API_BASE : `${origin}${API_BASE}`;
  const url = new URL(base);

  url.protocol = url.protocol === 'https:' ? 'wss:' : 'ws:';
  url.pathname = `${url.pathname.replace(/\/$/, '')}/campaigns/${encodeURIComponent(campaignId)}/ws`;
  url.search = '';
  url.hash = '';

  return url.toString();
}

function decodeWebSocketEvent(raw: string): ParsedWebSocketEvent | null {
  let parsed: unknown;
  try {
    parsed = JSON.parse(raw) as unknown;
  } catch {
    return null;
  }

  if (!isEnvelope(parsed)) {
    return null;
  }

  switch (parsed.type) {
    case 'chunk':
      if (!isChunkPayload(parsed.payload)) {
        return null;
      }

      return {
        kind: 'chunk',
        envelope: {
          ...parsed,
          payload: parsed.payload,
        },
        payload: parsed.payload,
      };
    case 'result': {
      const resultPayload = normalizeTurnResponse(parsed.payload);
      if (resultPayload === null) {
        return null;
      }

      return {
        kind: 'result',
        envelope: {
          ...parsed,
          payload: resultPayload,
        },
        payload: resultPayload,
      };
    }
    case 'status':
      if (!isStatusPayload(parsed.payload)) {
        return null;
      }

      return {
        kind: 'status',
        envelope: {
          ...parsed,
          payload: parsed.payload,
        },
        payload: parsed.payload,
      };
    case 'error':
      if (!isErrorPayload(parsed.payload)) {
        return null;
      }

      return {
        kind: 'error',
        envelope: {
          ...parsed,
          payload: parsed.payload,
        },
        payload: parsed.payload,
      };
    default:
      return null;
  }
}

function isEnvelope(value: unknown): value is WebSocketMessageEnvelope<unknown> {
  return isRecord(value) && typeof value.type === 'string' && 'payload' in value && typeof value.timestamp === 'string';
}

function isChunkPayload(value: unknown): value is WebSocketChunkPayload {
  return isRecord(value) && typeof value.text === 'string';
}

function isErrorPayload(value: unknown): value is WebSocketErrorPayload {
  return isRecord(value) && typeof value.error === 'string';
}

function isStatusPayload(value: unknown): value is WebSocketStatusPayload {
  return isRecord(value) && typeof value.stage === 'string' && typeof value.description === 'string';
}

function normalizeTurnResponse(value: unknown): TurnResponseWithChoices | null {
  if (!isRecord(value) || typeof value.narrative !== 'string' || !Array.isArray(value.state_changes)) {
    return null;
  }

  const stateChanges = value.state_changes;
  if (!stateChanges.every(isStateChange)) {
    return null;
  }

  return {
    narrative: value.narrative,
    state_changes: stateChanges,
    combat_active: typeof value.combat_active === 'boolean' ? value.combat_active : false,
    choices: normalizeChoices(value.choices),
  };
}

function normalizeChoices(value: unknown): NarrativeChoice[] {
  if (!Array.isArray(value)) {
    return [];
  }

  return value.filter(isNarrativeChoice);
}

function isNarrativeChoice(value: unknown): value is NarrativeChoice {
  return isRecord(value) && typeof value.id === 'string' && typeof value.text === 'string';
}

function isStateChange(value: unknown): value is StateChange {
  return (
    isRecord(value) &&
    typeof value.entity_type === 'string' &&
    typeof value.entity_id === 'string' &&
    typeof value.change_type === 'string' &&
    isRecord(value.details)
  );
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null;
}
