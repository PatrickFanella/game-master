import { useEffect, useRef } from 'react';
import { cn } from '../../lib/cn';
import type { NarrativeEntryItem } from '../narrative/NarrativeEntry';

interface CombatLogProps {
  readonly entries: NarrativeEntryItem[];
}

const CATEGORY_STYLES: Record<string, string> = {
  player: 'text-jade',
  gm: 'text-gold',
  system: 'text-pewter',
};

export function CombatLog({ entries }: CombatLogProps) {
  const scrollRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (scrollRef.current) {
      scrollRef.current.scrollTop = scrollRef.current.scrollHeight;
    }
  }, [entries.length]);

  if (entries.length === 0) {
    return (
      <div className="flex min-h-[12rem] items-center justify-center border-2 border-ruby/15 bg-charcoal/50 p-4">
        <p className="font-heading text-xs uppercase tracking-[0.2em] text-pewter/60">
          Awaiting combat narrative...
        </p>
      </div>
    );
  }

  return (
    <div className="border-2 border-ruby/20 bg-charcoal">
      <div className="border-b border-ruby/15 px-4 py-2">
        <h3 className="font-heading text-xs font-semibold uppercase tracking-[0.2em] text-ruby">
          Combat Log
        </h3>
      </div>
      <div ref={scrollRef} className="max-h-[24rem] overflow-y-auto p-4">
        <div className="space-y-3">
          {entries.map((entry) => (
            <div key={entry.id} className="border-l-2 border-ruby/20 pl-3">
              <div className="flex items-center gap-2">
                <span
                  className={cn(
                    'text-[10px] font-bold uppercase tracking-widest',
                    CATEGORY_STYLES[entry.kind] ?? 'text-pewter',
                  )}
                >
                  {entry.speaker ?? entry.kind}
                </span>
                <span className="text-[10px] uppercase tracking-widest text-pewter/50">
                  {formatTime(entry.timestamp)}
                </span>
              </div>
              <p className="mt-1 whitespace-pre-wrap text-sm leading-6 text-champagne/90">
                {entry.text}
              </p>
            </div>
          ))}
        </div>
      </div>
    </div>
  );
}

function formatTime(timestamp: string): string {
  const parsed = new Date(timestamp);
  if (Number.isNaN(parsed.getTime())) return timestamp;
  return parsed.toLocaleTimeString([], { hour: 'numeric', minute: '2-digit' });
}
