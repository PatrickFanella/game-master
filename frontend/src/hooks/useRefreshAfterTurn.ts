import { useEffect, useRef } from 'react';
import { useQueryClient } from '@tanstack/react-query';

import type { StateChange } from '../api/types';

interface TurnResult {
  readonly state_changes?: StateChange[];
}

const ENTITY_TYPE_TO_CACHE_SEGMENT: Record<string, string> = {
  player_character: 'character',
  item: 'inventory',
  quest: 'quests',
  location: 'locations',
  npc: 'npcs',
  world_fact: 'facts',
  language: 'codex-languages',
  culture: 'codex-cultures',
  belief_system: 'codex-beliefs',
  economic_system: 'codex-economies',
  entity_relationship: 'relationships',
};

/**
 * Invalidates React Query caches after a turn completes. Uses state_changes
 * from the turn result to selectively invalidate only affected resources.
 * Falls back to invalidating character and inventory if no state_changes.
 */
export function useRefreshAfterTurn(campaignId: string | null, latestResult: TurnResult | null) {
  const queryClient = useQueryClient();
  const prevResultRef = useRef<TurnResult | null>(null);

  useEffect(() => {
    if (!campaignId || !latestResult || latestResult === prevResultRef.current) {
      return;
    }
    prevResultRef.current = latestResult;

    const changes = latestResult.state_changes;
    if (changes && changes.length > 0) {
      const segments = new Set<string>();
      for (const change of changes) {
        const segment = ENTITY_TYPE_TO_CACHE_SEGMENT[change.entity_type];
        if (segment) {
          segments.add(segment);
        }
      }
      // Always refresh character sheet (HP, XP, level may change)
      segments.add('character');

      for (const segment of segments) {
        void queryClient.invalidateQueries({ queryKey: ['campaign', campaignId, segment] });
      }
    } else {
      // No state_changes — refresh character and inventory as defaults
      void queryClient.invalidateQueries({ queryKey: ['campaign', campaignId, 'character'] });
      void queryClient.invalidateQueries({ queryKey: ['campaign', campaignId, 'inventory'] });
    }
  }, [campaignId, latestResult, queryClient]);
}
