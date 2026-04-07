import { useMemo } from 'react';
import type { NarrativeEntryItem } from '../narrative/NarrativeEntry';
import { CombatActionBar } from './CombatActionBar';
import { CombatLog } from './CombatLog';
import { CombatantCard } from './CombatantCard';
import { InitiativeTracker } from './InitiativeTracker';
import type { CombatantInfo } from './types';

interface CombatViewProps {
  readonly entries: NarrativeEntryItem[];
  readonly streamingEntry: NarrativeEntryItem | null;
  readonly onAction: (text: string) => void;
  readonly isLoading: boolean;
}

/**
 * Placeholder combatant data used until the backend sends real combatant
 * lists as part of combat events.
 */
const MOCK_COMBATANTS: CombatantInfo[] = [
  { id: 'player-1', name: 'You', initiative: 18, isPlayer: true, hp: 28, maxHp: 32 },
  { id: 'enemy-1', name: 'Goblin Scout', initiative: 15, isPlayer: false, hp: 12, maxHp: 12 },
  { id: 'enemy-2', name: 'Goblin Archer', initiative: 11, isPlayer: false, hp: 9, maxHp: 10 },
];

const MOCK_CONDITIONS: Record<string, string[]> = {
  'player-1': [],
  'enemy-1': [],
  'enemy-2': [],
};

export function CombatView({ entries, streamingEntry, onAction, isLoading }: CombatViewProps) {
  const allEntries = useMemo(() => {
    if (!streamingEntry) return entries;
    return [...entries, streamingEntry];
  }, [entries, streamingEntry]);

  const combatants = MOCK_COMBATANTS;
  const currentTurnIndex = 0;

  return (
    <div className="space-y-4">
      {/* Top: Initiative Tracker */}
      <InitiativeTracker combatants={combatants} currentTurnIndex={currentTurnIndex} />

      {/* Main area: Combatants grid + Combat log */}
      <div className="grid gap-4 lg:grid-cols-[minmax(14rem,1fr)_minmax(0,2fr)]">
        {/* Left: Combatant cards */}
        <div className="space-y-3">
          <h3 className="font-heading text-xs font-semibold uppercase tracking-[0.2em] text-ruby/70">
            Combatants
          </h3>
          {combatants.map((combatant, index) => (
            <CombatantCard
              key={combatant.id}
              combatant={combatant}
              conditions={MOCK_CONDITIONS[combatant.id] ?? []}
              isActive={index === currentTurnIndex}
            />
          ))}
        </div>

        {/* Center: Combat log */}
        <CombatLog entries={allEntries} />
      </div>

      {/* Bottom: Action bar */}
      <CombatActionBar onAction={onAction} disabled={isLoading} />
    </div>
  );
}
