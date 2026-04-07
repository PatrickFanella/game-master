import { useMemo } from 'react';
import { useQuery } from '@tanstack/react-query';

import { listAwareRelationships } from '../../api/relationships';
import type { RelationshipResponse } from '../../api/types';
import { cn } from '../../lib/cn';

interface RelationshipsPanelProps {
  readonly campaignId: string;
  readonly className?: string;
}

function queryErrorMessage(error: unknown): string {
  return error instanceof Error ? error.message : 'Unable to load relationships.';
}

export function RelationshipsPanel({ campaignId, className }: RelationshipsPanelProps) {
  const relationshipsQuery = useQuery({
    queryKey: ['campaign', campaignId, 'relationships'],
    queryFn: () => listAwareRelationships(campaignId),
    enabled: campaignId.trim().length > 0,
  });

  const groupedRelationships = useMemo(() => {
    const relationships = relationshipsQuery.data ?? [];
    const groups: Record<string, RelationshipResponse[]> = {};
    for (const rel of relationships) {
      const type = rel.relationship_type || 'Other';
      if (!groups[type]) {
        groups[type] = [];
      }
      groups[type].push(rel);
    }
    return groups;
  }, [relationshipsQuery.data]);

  if (campaignId.trim().length === 0) {
    return <PanelMessage className={className} tone="error" title="Missing campaign" message="Select a campaign before viewing relationships." />;
  }

  if (relationshipsQuery.isPending) {
    return <PanelMessage className={className} tone="default" title="Loading relationships" message="Gathering known relationships for this campaign." />;
  }

  if (relationshipsQuery.isError) {
    return <PanelMessage className={className} tone="error" title="Relationships unavailable" message={queryErrorMessage(relationshipsQuery.error)} />;
  }

  const relationships = relationshipsQuery.data;

  if (relationships.length === 0) {
    return <PanelMessage className={className} tone="default" title="No relationships known" message="Relationships between entities that your character is aware of will appear here." />;
  }

  const types = Object.keys(groupedRelationships).sort();

  return (
    <section className={cn('space-y-5 border-2 border-ruby/20 bg-charcoal p-5', className)}>
      <div className="border-b-2 border-ruby/20 pb-5">
        <p className="font-heading text-xs font-semibold uppercase tracking-[0.2em] text-ruby">Relationships</p>
        <h2 className="font-heading mt-2 text-xl font-semibold uppercase tracking-[0.1em] text-champagne">Known connections</h2>
        <p className="mt-2 max-w-2xl text-sm leading-6 text-pewter">
          Relationships between entities that your character is aware of, grouped by type.
        </p>
      </div>

      <div className="space-y-6">
        {types.map((type) => (
          <div key={type}>
            <h3 className="mb-3 font-heading text-sm font-semibold uppercase tracking-[0.15em] text-ruby/80">{type}</h3>
            <div className="space-y-3">
              {groupedRelationships[type].map((rel) => (
                <div key={rel.id} className="border border-ruby/15 bg-obsidian px-5 py-4 transition-all duration-200 hover:border-ruby/30">
                  <div className="flex flex-wrap items-center gap-2 text-sm">
                    <span className="font-semibold text-champagne">{rel.source_entity_type}:{rel.source_entity_id.slice(0, 8)}</span>
                    <span className="text-ruby">&rarr;</span>
                    <span className="font-semibold text-champagne">{rel.target_entity_type}:{rel.target_entity_id.slice(0, 8)}</span>
                    <span className="text-pewter/60">({rel.relationship_type})</span>
                  </div>
                  <p className="mt-2 text-sm leading-6 text-pewter">{rel.description}</p>
                  {rel.strength != null && (
                    <div className="mt-2 flex items-center gap-3">
                      <span className="text-[11px] font-semibold uppercase tracking-[0.2em] text-pewter/80">Strength</span>
                      <div className="h-2 w-24 overflow-hidden border border-ruby/20 bg-charcoal">
                        <div
                          className="h-full bg-ruby transition-all duration-200"
                          style={{ width: `${Math.max(0, Math.min(100, rel.strength))}%` }}
                        />
                      </div>
                      <span className="text-xs text-champagne/70">{rel.strength}</span>
                    </div>
                  )}
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
          : 'border-ruby/20 bg-charcoal text-champagne/70',
        className,
      )}
    >
      <h2 className="font-heading text-lg font-semibold uppercase tracking-[0.1em] text-champagne">{title}</h2>
      <p className="mt-3 text-sm leading-6">{message}</p>
    </section>
  );
}
