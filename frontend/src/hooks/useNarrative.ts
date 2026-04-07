import { useCallback, useEffect, useMemo, useRef, useState } from 'react';

import { getSessionHistory } from '../api/campaigns';
import type { StateChange } from '../api/types';
import type { WebSocketStatusPayload } from '../api/types';
import { useCampaign } from './useCampaign';
import { useWebSocket, type ConnectionStatus, type NarrativeChoice, type TurnResponseWithChoices } from './useWebSocket';

export type NarrativeEntryKind = 'player' | 'gm' | 'system';

export interface NarrativeEntry {
  id: string;
  kind: NarrativeEntryKind;
  text: string;
  timestamp: string;
  speaker?: string;
  stateChanges?: StateChange[];
  choices?: NarrativeChoice[];
  isStreaming?: boolean;
}

export interface UseNarrativeResult {
  campaignId: string | null;
  connectionStatus: ConnectionStatus;
  entries: NarrativeEntry[];
  streamingEntry: NarrativeEntry | null;
  latestResult: TurnResponseWithChoices | null;
  suggestedChoices: NarrativeChoice[];
  currentStatus: WebSocketStatusPayload | null;
  isLoading: boolean;
  error: string | null;
  sendAction: (input: string) => boolean;
}

export function useNarrative(): UseNarrativeResult {
  const { campaignId } = useCampaign();
  const { connectionStatus, error: socketError, events, isLoading, currentStatus, sendAction: sendSocketAction } = useWebSocket(campaignId);
  const [entries, setEntries] = useState<NarrativeEntry[]>([]);
  const [streamingChunks, setStreamingChunks] = useState<string[]>([]);
  const [pendingTimestamp, setPendingTimestamp] = useState<string | null>(null);
  const [latestResult, setLatestResult] = useState<TurnResponseWithChoices | null>(null);
  const [suggestedChoices, setSuggestedChoices] = useState<NarrativeChoice[]>([]);
  const [error, setError] = useState<string | null>(null);
  const processedEventCountRef = useRef(0);

  useEffect(() => {
    processedEventCountRef.current = 0;
    setEntries([]);
    setStreamingChunks([]);
    setPendingTimestamp(null);
    setLatestResult(null);
    setSuggestedChoices([]);
    setError(null);

    if (!campaignId) return;

    getSessionHistory(campaignId)
      .then((resp) => {
        const restored: NarrativeEntry[] = [];
        for (const entry of resp.entries) {
          if (entry.input_type !== 'narrative' && entry.player_input) {
            restored.push({
              id: `history-player-${entry.turn_number}`,
              kind: 'player',
              text: entry.player_input,
              timestamp: entry.created_at,
              speaker: 'You',
            });
          }
          if (entry.llm_response) {
            restored.push({
              id: `history-gm-${entry.turn_number}`,
              kind: 'gm',
              text: entry.llm_response,
              timestamp: entry.created_at,
              speaker: 'Game Master',
            });
          }
        }
        setEntries(restored);
      })
      .catch((err) => {
        console.warn('Failed to load session history:', err);
      });
  }, [campaignId]);

  useEffect(() => {
    setError(socketError);

    if (socketError !== null) {
      setPendingTimestamp(null);
    }
  }, [socketError]);

  useEffect(() => {
    if (connectionStatus === 'closed' || connectionStatus === 'error') {
      setPendingTimestamp(null);
    }
  }, [connectionStatus]);

  useEffect(() => {
    const nextEvents = events.slice(processedEventCountRef.current);
    if (nextEvents.length === 0) {
      return;
    }

    processedEventCountRef.current = events.length;

    for (const event of nextEvents) {
      if (event.kind === 'chunk') {
        setStreamingChunks((current) => [...current, event.payload.text]);
        setError(null);
        continue;
      }

      if (event.kind === 'result') {
        const choices = event.payload.choices ?? [];

        setEntries((current) => [
          ...current,
          {
            id: `${event.envelope.timestamp}-gm-${current.length}`,
            kind: 'gm',
            text: event.payload.narrative,
            timestamp: event.envelope.timestamp,
            speaker: 'Game Master',
            stateChanges: event.payload.state_changes,
            choices,
          },
        ]);
        setStreamingChunks([]);
        setPendingTimestamp(null);
        setLatestResult(event.payload);
        setSuggestedChoices(choices);
        setError(null);
        continue;
      }

      if (event.kind === 'status') {
        continue;
      }

      setStreamingChunks([]);
      setPendingTimestamp(null);
      setSuggestedChoices([]);
      setError(event.payload.error);
      setEntries((current) => [
        ...current,
        {
          id: `${event.envelope.timestamp}-system-${current.length}`,
          kind: 'system',
          text: event.payload.error,
          timestamp: event.envelope.timestamp,
          speaker: 'System',
        },
      ]);
    }
  }, [events]);

  const sendAction = useCallback(
    (input: string): boolean => {
      const trimmedInput = input.trim();
      if (!trimmedInput) {
        return false;
      }

      const accepted = sendSocketAction(trimmedInput);
      if (!accepted) {
        return false;
      }

      const timestamp = new Date().toISOString();

      setEntries((current) => [
        ...current,
        {
          id: `${timestamp}-player-${current.length}`,
          kind: 'player',
          text: trimmedInput,
          timestamp,
          speaker: 'You',
        },
      ]);
      setStreamingChunks([]);
      setPendingTimestamp(timestamp);
      setLatestResult(null);
      setSuggestedChoices([]);
      setError(null);

      return true;
    },
    [sendSocketAction],
  );

  const streamingEntry = useMemo<NarrativeEntry | null>(() => {
    if (!pendingTimestamp || !isLoading) {
      return null;
    }

    return {
      id: `${pendingTimestamp}-streaming`,
      kind: 'gm',
      text: streamingChunks.join('') || 'Game Master is thinking…',
      timestamp: pendingTimestamp,
      speaker: 'Game Master',
      isStreaming: true,
    };
  }, [isLoading, pendingTimestamp, streamingChunks]);

  return {
    campaignId,
    connectionStatus,
    entries,
    streamingEntry,
    latestResult,
    suggestedChoices,
    currentStatus,
    isLoading,
    error,
    sendAction,
  };
}
