import { useState } from 'react';
import { useQuery } from '@tanstack/react-query';

import { getCampaignCharacterFeats } from '../../api/characters';
import { cn } from '../../lib/cn';

interface FeatBrowserProps {
  readonly campaignId: string;
  readonly className?: string;
}

export function FeatBrowser({ campaignId, className }: FeatBrowserProps) {
  const [isExpanded, setIsExpanded] = useState(false);

  const featsQuery = useQuery({
    queryKey: ['campaign', campaignId, 'character', 'feats'],
    queryFn: () => getCampaignCharacterFeats(campaignId),
    enabled: campaignId.length > 0,
  });

  return (
    <section className={cn('border-2 border-jade/20 bg-charcoal', className)}>
      <button
        type="button"
        onClick={() => setIsExpanded(!isExpanded)}
        className="flex w-full items-center justify-between p-6 text-left transition hover:bg-jade/5"
      >
        <div>
          <p className="font-heading text-xs font-semibold uppercase tracking-[0.2em] text-jade">Crunch</p>
          <h3 className="font-heading mt-1 text-lg font-semibold uppercase tracking-[0.1em] text-champagne">
            Feats
          </h3>
        </div>
        <span className="text-sm text-pewter">{isExpanded ? '\u25B2' : '\u25BC'}</span>
      </button>

      {isExpanded && (
        <div className="border-t border-jade/15 px-6 pb-6">
          {featsQuery.isPending && (
            <p className="py-4 text-sm text-pewter">Loading feats...</p>
          )}

          {featsQuery.isError && (
            <p className="py-4 text-sm text-ruby">Failed to load feats.</p>
          )}

          {featsQuery.isSuccess && featsQuery.data.length === 0 && (
            <div className="mt-4 border border-dashed border-jade/15 bg-charcoal/50 px-4 py-6 text-sm text-pewter">
              No feats have been granted yet. Feats are earned through gameplay in crunch mode.
            </div>
          )}

          {featsQuery.isSuccess && featsQuery.data.length > 0 && (
            <ul className="mt-4 space-y-3">
              {featsQuery.data.map((feat) => (
                <li
                  key={feat.id}
                  className="border border-jade/20 bg-charcoal/80 p-4 transition-all duration-200 hover:border-sapphire/30"
                >
                  <div className="flex items-start justify-between gap-3">
                    <div className="space-y-1">
                      <h4 className="text-base font-semibold text-champagne">{feat.name}</h4>
                      <p className="text-sm leading-6 text-champagne/70">{feat.description}</p>
                    </div>
                    {feat.bonus_value !== 0 && (
                      <span className="inline-flex shrink-0 items-center rounded-sm border border-gold/30 bg-gold/10 px-2 py-1 text-xs font-semibold text-gold">
                        +{feat.bonus_value} {feat.bonus_type}
                      </span>
                    )}
                  </div>
                </li>
              ))}
            </ul>
          )}
        </div>
      )}
    </section>
  );
}
