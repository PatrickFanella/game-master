import { cn } from '../../lib/cn';
import type { CombatantInfo } from './types';

interface CombatantCardProps {
  readonly combatant: CombatantInfo;
  readonly conditions: string[];
  readonly isActive: boolean;
}

export function CombatantCard({ combatant, conditions, isActive }: CombatantCardProps) {
  const hpPercent = combatant.maxHp > 0 ? (combatant.hp / combatant.maxHp) * 100 : 0;

  // Border color: ruby for enemies, jade for allies, gold for player
  const borderColor = combatant.isPlayer
    ? 'border-gold'
    : 'border-ruby';
  const borderIdle = combatant.isPlayer
    ? 'border-gold/30'
    : 'border-ruby/30';

  return (
    <div
      className={cn(
        'border-2 bg-charcoal p-4 transition-all duration-200',
        isActive ? `${borderColor} shadow-ruby-sm` : borderIdle,
      )}
    >
      {/* Header */}
      <div className="flex items-start justify-between gap-2">
        <div>
          <h4
            className={cn(
              'font-heading text-sm font-semibold uppercase tracking-wide',
              combatant.isPlayer ? 'text-gold' : 'text-ruby',
            )}
          >
            {combatant.name}
          </h4>
          <span className="text-[10px] uppercase tracking-widest text-pewter">
            Initiative {combatant.initiative}
          </span>
        </div>
        {isActive ? (
          <span className="inline-flex rounded-sm border border-ruby/30 bg-ruby/15 px-2 py-0.5 text-[10px] font-bold uppercase tracking-[0.2em] text-ruby">
            Active
          </span>
        ) : null}
      </div>

      {/* HP Bar */}
      <div className="mt-3">
        <div className="mb-1 flex items-center justify-between text-[10px] uppercase tracking-widest text-pewter">
          <span>HP</span>
          <span>
            {combatant.hp} / {combatant.maxHp}
          </span>
        </div>
        <div className="h-2 w-full bg-obsidian/60">
          <div
            className={cn(
              'h-full transition-all duration-500',
              hpPercent > 50
                ? 'bg-gradient-to-r from-jade/80 to-jade'
                : hpPercent > 25
                  ? 'bg-gradient-to-r from-gold/80 to-gold'
                  : 'bg-gradient-to-r from-ruby/80 to-ruby',
            )}
            style={{ width: `${Math.max(0, Math.min(100, hpPercent))}%` }}
          />
        </div>
      </div>

      {/* Conditions */}
      {conditions.length > 0 ? (
        <div className="mt-3 flex flex-wrap gap-1">
          {conditions.map((condition) => (
            <span
              key={condition}
              className="inline-flex rounded-sm border border-gold/20 bg-gold/10 px-2 py-0.5 text-[10px] font-medium uppercase tracking-[0.15em] text-gold"
            >
              {condition}
            </span>
          ))}
        </div>
      ) : null}
    </div>
  );
}
