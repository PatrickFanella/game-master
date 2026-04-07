import { useState } from 'react';
import { useQuery } from '@tanstack/react-query';

import { listEncounteredNPCs, getNPCDialogue } from '../../api/encountered';
import type { EncounteredNPCResponse, DialogueEntry } from '../../api/types';
import { cn } from '../../lib/cn';

interface NPCPanelProps {
  readonly campaignId: string;
  readonly className?: string;
}

function queryErrorMessage(error: unknown): string {
  return error instanceof Error ? error.message : 'Unable to load encountered NPCs.';
}

export function NPCPanel({ campaignId, className }: NPCPanelProps) {
  const [selectedNpcId, setSelectedNpcId] = useState<string | null>(null);

  const npcsQuery = useQuery({
    queryKey: ['campaign', campaignId, 'npcs-encountered'],
    queryFn: () => listEncounteredNPCs(campaignId),
    enabled: campaignId.trim().length > 0,
  });

  if (campaignId.trim().length === 0) {
    return <PanelMessage className={className} tone="error" title="Missing campaign" message="Select a campaign before viewing NPCs." />;
  }

  if (npcsQuery.isPending) {
    return <PanelMessage className={className} tone="default" title="Loading NPCs" message="Gathering encountered NPCs for this campaign." />;
  }

  if (npcsQuery.isError) {
    return <PanelMessage className={className} tone="error" title="NPCs unavailable" message={queryErrorMessage(npcsQuery.error)} />;
  }

  const npcs = npcsQuery.data;

  if (npcs.length === 0) {
    return <PanelMessage className={className} tone="default" title="No NPCs encountered" message="NPCs you meet during the campaign will appear here with their dialogue history." />;
  }

  return (
    <section className={cn('space-y-5 border-2 border-gold/20 bg-charcoal p-5', className)}>
      <div className="border-b-2 border-gold/20 pb-5">
        <p className="font-heading text-xs font-semibold uppercase tracking-[0.2em] text-gold">NPCs</p>
        <h2 className="font-heading mt-2 text-xl font-semibold uppercase tracking-[0.1em] text-champagne">Encountered characters</h2>
        <p className="mt-2 max-w-2xl text-sm leading-6 text-pewter">
          NPCs you have met during this campaign. Select one to view dialogue history.
        </p>
      </div>

      <div className="space-y-4">
        {npcs.map((npc) => (
          <NPCCard
            key={npc.id}
            npc={npc}
            campaignId={campaignId}
            isExpanded={selectedNpcId === npc.id}
            onToggle={() => setSelectedNpcId(selectedNpcId === npc.id ? null : npc.id)}
          />
        ))}
      </div>
    </section>
  );
}

interface NPCCardProps {
  readonly npc: EncounteredNPCResponse;
  readonly campaignId: string;
  readonly isExpanded: boolean;
  readonly onToggle: () => void;
}

function NPCCard({ npc, campaignId, isExpanded, onToggle }: NPCCardProps) {
  return (
    <div className="border border-gold/20 bg-obsidian transition-all duration-200 hover:border-gold/40">
      <button
        type="button"
        onClick={onToggle}
        className="flex w-full items-start justify-between gap-4 px-5 py-4 text-left focus:outline-none focus:ring-2 focus:ring-gold focus:ring-offset-2 focus:ring-offset-obsidian"
      >
        <div className="min-w-0 flex-1">
          <div className="flex flex-wrap items-center gap-3">
            <h3 className="font-heading text-base font-semibold uppercase tracking-wide text-champagne">{npc.name}</h3>
            <span
              className={cn(
                'inline-block border px-2 py-0.5 text-[11px] font-semibold uppercase tracking-[0.2em]',
                npc.alive
                  ? 'border-jade/40 bg-jade/10 text-jade'
                  : 'border-ruby/40 bg-ruby/10 text-ruby',
              )}
            >
              {npc.alive ? 'Alive' : 'Dead'}
            </span>
          </div>
          <p className="mt-2 text-sm leading-6 text-pewter">{npc.description}</p>
          {npc.disposition != null && (
            <div className="mt-3 flex items-center gap-3">
              <span className="text-xs font-semibold uppercase tracking-[0.2em] text-pewter/80">Disposition</span>
              <div className="h-2 w-32 overflow-hidden border border-gold/20 bg-charcoal">
                <div
                  className="h-full bg-gold transition-all duration-200"
                  style={{ width: `${Math.max(0, Math.min(100, (npc.disposition + 100) / 2))}%` }}
                />
              </div>
              <span className="text-xs text-champagne/70">{npc.disposition}</span>
            </div>
          )}
        </div>
        <span className="mt-1 text-xs text-pewter/60">{isExpanded ? '\u25B2' : '\u25BC'}</span>
      </button>

      {isExpanded && (
        <NPCDialogueSection campaignId={campaignId} npcId={npc.id} />
      )}
    </div>
  );
}

interface NPCDialogueSectionProps {
  readonly campaignId: string;
  readonly npcId: string;
}

function NPCDialogueSection({ campaignId, npcId }: NPCDialogueSectionProps) {
  const dialogueQuery = useQuery({
    queryKey: ['campaign', campaignId, 'npc-dialogue', npcId],
    queryFn: () => getNPCDialogue(campaignId, npcId),
  });

  if (dialogueQuery.isPending) {
    return (
      <div className="border-t border-gold/15 px-5 py-4 text-sm text-champagne/70">Loading dialogue...</div>
    );
  }

  if (dialogueQuery.isError) {
    return (
      <div className="border-t border-ruby/30 px-5 py-4 text-sm text-ruby">
        {queryErrorMessage(dialogueQuery.error)}
      </div>
    );
  }

  const entries = dialogueQuery.data;

  if (entries.length === 0) {
    return (
      <div className="border-t border-gold/15 px-5 py-4 text-sm text-pewter">No dialogue recorded yet.</div>
    );
  }

  return (
    <div className="border-t border-gold/15 px-5 py-4">
      <p className="mb-3 text-xs font-semibold uppercase tracking-[0.2em] text-gold/70">Dialogue history</p>
      <div className="space-y-3">
        {entries.map((entry) => (
          <DialogueEntryCard key={entry.turn_number} entry={entry} />
        ))}
      </div>
    </div>
  );
}

function DialogueEntryCard({ entry }: { readonly entry: DialogueEntry }) {
  return (
    <div className="space-y-1 border-l-2 border-gold/20 pl-4 text-sm">
      <p className="text-champagne/90">
        <span className="font-semibold text-gold/80">You:</span> {entry.player_input}
      </p>
      <p className="text-champagne/70">
        <span className="font-semibold text-pewter">NPC:</span> {entry.llm_response}
      </p>
      <p className="text-[11px] text-pewter/60">Turn {entry.turn_number} · {new Date(entry.created_at).toLocaleString()}</p>
    </div>
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
          : 'border-gold/20 bg-charcoal text-champagne/70',
        className,
      )}
    >
      <h2 className="font-heading text-lg font-semibold uppercase tracking-[0.1em] text-champagne">{title}</h2>
      <p className="mt-3 text-sm leading-6">{message}</p>
    </section>
  );
}
