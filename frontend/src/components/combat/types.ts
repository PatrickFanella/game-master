/** Information about a single combatant in the initiative order. */
export interface CombatantInfo {
  id: string;
  name: string;
  initiative: number;
  isPlayer: boolean;
  hp: number;
  maxHp: number;
}

/** A single entry in the combat log. */
export interface CombatLogEntry {
  id: string;
  round: number;
  action: string;
  outcome: string;
  /** 'damage' | 'healing' | 'status' | 'neutral' */
  category: 'damage' | 'healing' | 'status' | 'neutral';
  timestamp: string;
}
