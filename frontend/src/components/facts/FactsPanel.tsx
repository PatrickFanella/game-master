import { useMemo } from 'react';
import { useQuery } from '@tanstack/react-query';

import { listKnownFacts } from '../../api/facts';
import type { FactResponse } from '../../api/types';
import { cn } from '../../lib/cn';

interface FactsPanelProps {
  readonly campaignId: string;
  readonly className?: string;
}

function queryErrorMessage(error: unknown): string {
  return error instanceof Error ? error.message : 'Unable to load known facts.';
}

export function FactsPanel({ campaignId, className }: FactsPanelProps) {
  const factsQuery = useQuery({
    queryKey: ['campaign', campaignId, 'facts'],
    queryFn: () => listKnownFacts(campaignId),
    enabled: campaignId.trim().length > 0,
  });

  const groupedFacts = useMemo(() => {
    const facts = factsQuery.data ?? [];
    const groups: Record<string, FactResponse[]> = {};
    for (const fact of facts) {
      const category = fact.category || 'Uncategorized';
      if (!groups[category]) {
        groups[category] = [];
      }
      groups[category].push(fact);
    }
    return groups;
  }, [factsQuery.data]);

  if (campaignId.trim().length === 0) {
    return <PanelMessage className={className} tone="error" title="Missing campaign" message="Select a campaign before viewing facts." />;
  }

  if (factsQuery.isPending) {
    return <PanelMessage className={className} tone="default" title="Loading facts" message="Gathering known facts and lore for this campaign." />;
  }

  if (factsQuery.isError) {
    return <PanelMessage className={className} tone="error" title="Facts unavailable" message={queryErrorMessage(factsQuery.error)} />;
  }

  const facts = factsQuery.data;

  if (facts.length === 0) {
    return <PanelMessage className={className} tone="default" title="No facts discovered" message="World facts and lore your character has learned will appear here." />;
  }

  const categories = Object.keys(groupedFacts).sort();

  return (
    <section className={cn('space-y-5 border-2 border-sapphire/20 bg-charcoal p-5', className)}>
      <div className="border-b-2 border-sapphire/20 pb-5">
        <p className="font-heading text-xs font-semibold uppercase tracking-[0.2em] text-sapphire">Facts & Lore</p>
        <h2 className="font-heading mt-2 text-xl font-semibold uppercase tracking-[0.1em] text-champagne">Known world facts</h2>
        <p className="mt-2 max-w-2xl text-sm leading-6 text-pewter">
          Facts and lore discovered during the campaign, grouped by category.
        </p>
      </div>

      <div className="space-y-6">
        {categories.map((category) => (
          <div key={category}>
            <h3 className="mb-3 font-heading text-sm font-semibold uppercase tracking-[0.15em] text-sapphire/80">{category}</h3>
            <div className="space-y-3">
              {groupedFacts[category].map((fact) => (
                <div key={fact.id} className="border border-sapphire/15 bg-obsidian px-5 py-4 transition-all duration-200 hover:border-sapphire/30">
                  <p className="text-sm leading-6 text-champagne">{fact.fact}</p>
                  <div className="mt-2 flex flex-wrap items-center gap-4 text-[11px] text-pewter/70">
                    <span>Source: <span className="text-pewter">{fact.source}</span></span>
                    <span>{new Date(fact.created_at).toLocaleString()}</span>
                  </div>
                </div>
              ))}
            </div>
          </div>
        ))}
      </div>
    </section>
  );
}

interface PanelMessageProps {
  readonly title: string;
  readonly message: string;
  readonly tone: 'default' | 'error';
  readonly className?: string;
}

function PanelMessage({ title, message, tone, className }: PanelMessageProps) {
  return (
    <section
      className={cn(
        'border p-6',
        tone === 'error'
          ? 'border-ruby/40 bg-ruby/10 text-ruby'
          : 'border-sapphire/20 bg-charcoal text-champagne/70',
        className,
      )}
    >
      <h2 className="font-heading text-lg font-semibold uppercase tracking-[0.1em] text-champagne">{title}</h2>
      <p className="mt-3 text-sm leading-6">{message}</p>
    </section>
  );
}
