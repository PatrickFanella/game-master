import { useCallback, useState } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';

import { createEntry, deleteEntry, listEntries, listSummaries, triggerSummarize } from '../../api/journal';
import type { JournalEntryResponse, SessionSummaryResponse } from '../../api/types';

interface JournalPanelProps {
  readonly campaignId: string;
}

export function JournalPanel({ campaignId }: JournalPanelProps) {
  const [activeSection, setActiveSection] = useState<'chronicle' | 'notes'>('chronicle');

  return (
    <div className="space-y-4">
      <div className="flex flex-wrap items-center gap-3 border-2 border-gold/20 bg-charcoal px-5 py-4">
        <h2 className="font-heading text-lg font-semibold uppercase tracking-wide text-champagne">Journal</h2>
        <div className="ml-auto flex gap-2">
          <SectionButton label="Chronicle" active={activeSection === 'chronicle'} onClick={() => setActiveSection('chronicle')} />
          <SectionButton label="Notes" active={activeSection === 'notes'} onClick={() => setActiveSection('notes')} />
        </div>
      </div>

      {activeSection === 'chronicle' ? (
        <ChronicleSection campaignId={campaignId} />
      ) : (
        <NotesSection campaignId={campaignId} />
      )}
    </div>
  );
}

function SectionButton({ label, active, onClick }: { readonly label: string; readonly active: boolean; readonly onClick: () => void }) {
  return (
    <button
      type="button"
      onClick={onClick}
      className={`px-3 py-1.5 text-xs font-semibold uppercase tracking-wide transition-all duration-200 ${
        active
          ? 'border-2 border-gold bg-gold/10 text-gold'
          : 'border-2 border-gold/20 bg-charcoal text-champagne/70 hover:border-gold hover:text-gold'
      }`}
    >
      {label}
    </button>
  );
}

function ChronicleSection({ campaignId }: { readonly campaignId: string }) {
  const queryClient = useQueryClient();

  const summariesQuery = useQuery({
    queryKey: ['campaign', campaignId, 'journal', 'summaries'],
    queryFn: () => listSummaries(campaignId),
    enabled: campaignId.length > 0,
  });

  const summarizeMutation = useMutation({
    mutationFn: () => triggerSummarize(campaignId),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['campaign', campaignId, 'journal', 'summaries'] });
    },
  });

  const summaries = summariesQuery.data ?? [];
  const sorted = [...summaries].sort((a, b) => a.from_turn - b.from_turn);

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <p className="text-sm text-pewter">Auto-generated session summaries, ordered by turn range.</p>
        <button
          type="button"
          onClick={() => summarizeMutation.mutate()}
          disabled={summarizeMutation.isPending}
          className="inline-flex items-center justify-center border-2 border-gold/30 px-3 py-1.5 text-xs font-semibold uppercase tracking-wide text-champagne transition-all duration-200 hover:border-gold hover:text-gold focus:outline-none focus:ring-2 focus:ring-gold focus:ring-offset-2 focus:ring-offset-obsidian disabled:opacity-50"
        >
          {summarizeMutation.isPending ? 'Summarizing...' : 'Summarize'}
        </button>
      </div>

      {summarizeMutation.isError ? (
        <div className="border border-ruby/40 bg-ruby/10 px-4 py-3 text-sm text-ruby">Failed to generate summary.</div>
      ) : null}

      {summariesQuery.isPending ? (
        <div className="border border-gold/20 bg-charcoal p-6 text-sm text-champagne/70">Loading chronicle...</div>
      ) : sorted.length === 0 ? (
        <div className="flex min-h-32 flex-col items-center justify-center border border-dashed border-gold/15 bg-charcoal/50 px-6 text-center">
          <p className="font-heading text-sm font-semibold uppercase tracking-[0.2em] text-pewter/80">No summaries yet</p>
          <p className="mt-2 max-w-md text-sm leading-7 text-pewter">
            Summaries are auto-generated every 10 turns, or you can trigger one manually.
          </p>
        </div>
      ) : (
        <div className="space-y-3">
          {sorted.map((summary) => (
            <SummaryCard key={summary.id} summary={summary} />
          ))}
        </div>
      )}
    </div>
  );
}

function SummaryCard({ summary }: { readonly summary: SessionSummaryResponse }) {
  return (
    <div className="border-2 border-midnight/40 bg-obsidian p-5 transition-all duration-200 hover:border-midnight/60">
      <div className="mb-3 flex items-center justify-between">
        <p className="font-heading text-xs font-semibold uppercase tracking-[0.2em] text-gold">
          Turns {summary.from_turn} &ndash; {summary.to_turn}
        </p>
        <time className="text-xs text-pewter" dateTime={summary.created_at}>
          {new Date(summary.created_at).toLocaleDateString()}
        </time>
      </div>
      <p className="whitespace-pre-wrap text-sm leading-7 text-champagne/80">{summary.summary}</p>
    </div>
  );
}

function NotesSection({ campaignId }: { readonly campaignId: string }) {
  const queryClient = useQueryClient();
  const [newTitle, setNewTitle] = useState('');
  const [newContent, setNewContent] = useState('');

  const entriesQuery = useQuery({
    queryKey: ['campaign', campaignId, 'journal', 'entries'],
    queryFn: () => listEntries(campaignId),
    enabled: campaignId.length > 0,
  });

  const createMutation = useMutation({
    mutationFn: () => createEntry(campaignId, { title: newTitle.trim(), content: newContent.trim() }),
    onSuccess: () => {
      setNewTitle('');
      setNewContent('');
      void queryClient.invalidateQueries({ queryKey: ['campaign', campaignId, 'journal', 'entries'] });
    },
  });

  const deleteMutation = useMutation({
    mutationFn: (entryId: string) => deleteEntry(campaignId, entryId),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['campaign', campaignId, 'journal', 'entries'] });
    },
  });

  const handleCreate = useCallback(() => {
    if (newContent.trim().length === 0) return;
    createMutation.mutate();
  }, [createMutation, newContent]);

  const entries = entriesQuery.data ?? [];
  const sorted = [...entries].sort((a, b) => new Date(b.created_at).getTime() - new Date(a.created_at).getTime());

  return (
    <div className="space-y-4">
      <div className="border-2 border-gold/20 bg-charcoal p-5 space-y-3">
        <p className="font-heading text-xs font-semibold uppercase tracking-[0.2em] text-gold">New entry</p>
        <input
          type="text"
          placeholder="Title (optional)"
          value={newTitle}
          onChange={(e) => setNewTitle(e.target.value)}
          className="w-full border border-gold/20 bg-obsidian px-3 py-2 text-sm text-champagne placeholder:text-pewter/50 focus:border-gold focus:outline-none"
        />
        <textarea
          placeholder="Write your notes here..."
          value={newContent}
          onChange={(e) => setNewContent(e.target.value)}
          rows={4}
          className="w-full resize-y border border-gold/20 bg-obsidian px-3 py-2 text-sm leading-6 text-champagne placeholder:text-pewter/50 focus:border-gold focus:outline-none"
        />
        <div className="flex justify-end">
          <button
            type="button"
            onClick={handleCreate}
            disabled={newContent.trim().length === 0 || createMutation.isPending}
            className="inline-flex items-center justify-center bg-gold/90 px-4 py-2 text-sm font-semibold uppercase tracking-wide text-obsidian transition hover:bg-gold focus:outline-none focus:ring-2 focus:ring-gold focus:ring-offset-2 focus:ring-offset-obsidian disabled:cursor-not-allowed disabled:bg-charcoal disabled:text-pewter"
          >
            {createMutation.isPending ? 'Saving...' : 'Save entry'}
          </button>
        </div>
        {createMutation.isError ? (
          <div className="border border-ruby/40 bg-ruby/10 px-4 py-2 text-sm text-ruby">Failed to save entry.</div>
        ) : null}
      </div>

      {entriesQuery.isPending ? (
        <div className="border border-gold/20 bg-charcoal p-6 text-sm text-champagne/70">Loading notes...</div>
      ) : sorted.length === 0 ? (
        <div className="flex min-h-32 flex-col items-center justify-center border border-dashed border-gold/15 bg-charcoal/50 px-6 text-center">
          <p className="font-heading text-sm font-semibold uppercase tracking-[0.2em] text-pewter/80">No notes yet</p>
          <p className="mt-2 max-w-md text-sm leading-7 text-pewter">
            Add personal notes about quests, NPCs, or anything you want to remember.
          </p>
        </div>
      ) : (
        <div className="space-y-3">
          {sorted.map((entry) => (
            <EntryCard
              key={entry.id}
              entry={entry}
              onDelete={() => deleteMutation.mutate(entry.id)}
              isDeleting={deleteMutation.isPending}
            />
          ))}
        </div>
      )}
    </div>
  );
}

function EntryCard({
  entry,
  onDelete,
  isDeleting,
}: {
  readonly entry: JournalEntryResponse;
  readonly onDelete: () => void;
  readonly isDeleting: boolean;
}) {
  return (
    <div className="border-2 border-midnight/40 bg-obsidian p-5 transition-all duration-200 hover:border-midnight/60">
      <div className="mb-3 flex items-center justify-between">
        <h3 className="font-heading text-sm font-semibold uppercase tracking-wide text-champagne">
          {entry.title || 'Untitled'}
        </h3>
        <div className="flex items-center gap-3">
          <time className="text-xs text-pewter" dateTime={entry.created_at}>
            {new Date(entry.created_at).toLocaleDateString()}
          </time>
          <button
            type="button"
            onClick={onDelete}
            disabled={isDeleting}
            className="text-xs font-semibold uppercase tracking-wide text-ruby/70 transition hover:text-ruby disabled:opacity-50"
          >
            Delete
          </button>
        </div>
      </div>
      <p className="whitespace-pre-wrap text-sm leading-7 text-champagne/80">{entry.content}</p>
    </div>
  );
}
