import { cn } from '../../lib/cn';
import type { CombatantInfo } from './types';

interface InitiativeTrackerProps {
  readonly combatants: CombatantInfo[];
  readonly currentTurnIndex: number;
}

export function InitiativeTracker({ combatants, currentTurnIndex }: InitiativeTrackerProps) {
  if (combatants.length === 0) {
    return null;
  }

  return (
    <div className="border-2 border-ruby/30 bg-charcoal px-4 py-3">
      <h3 className="mb-3 font-heading text-xs font-semibold uppercase tracking-[0.2em] text-ruby">
        Initiative Order
      </h3>
      <div className="flex gap-2 overflow-x-auto pb-1">
        {combatants.map((combatant, index) => {
          const isActive = index === currentTurnIndex;
          const hpPercent = combatant.maxHp > 0 ? (combatant.hp / combatant.maxHp) * 100 : 0;

          return (
            <div
              key={combatant.id}
              className={cn(
                'flex min-w-[5rem] flex-shrink-0 flex-col items-center gap-1 border-2 px-3 py-2 transition-all duration-200',
                isActive
                  ? 'border-ruby bg-ruby/15 shadow-ruby'
                  : combatant.isPlayer
                    ? 'border-gold/20 bg-gold/5'
                    : 'border-pewter/20 bg-charcoal',
              )}
            >
              <span
                className={cn(
                  'text-[10px] font-bold uppercase tracking-widest',
                  isActive ? 'text-ruby' : 'text-pewter',
                )}
              >
                {combatant.initiative}
              </span>
              <span
                className={cn(
                  'max-w-[6rem] truncate text-xs font-semibold',
                  combatant.isPlayer ? 'text-gold' : 'text-champagne',
                )}
              >
                {combatant.name}
              </span>
              <div className="h-1 w-full bg-obsidian/50">
                <div
                  className={cn(
                    'h-full transition-all duration-300',
                    hpPercent > 50 ? 'bg-jade' : hpPercent > 25 ? 'bg-gold' : 'bg-ruby',
                  )}
                  style={{ width: `${Math.max(0, Math.min(100, hpPercent))}%` }}
                />
              </div>
            </div>
          );
        })}
      </div>
    </div>
  );
}
