import type { NarrativeEntryItem } from '../narrative/NarrativeEntry';

interface LogPanelProps {
  readonly entries: NarrativeEntryItem[];
  readonly streamingEntry: NarrativeEntryItem | null;
  readonly isLoading: boolean;
  readonly error: string | null;
}

export function LogPanel({ entries, streamingEntry, isLoading, error }: LogPanelProps) {
  const allEntries = streamingEntry ? [...entries, streamingEntry] : entries;

  return (
    <div className="flex flex-col gap-2 p-4 font-mono text-sm">
      {error && (
        <p className="text-red-400">{error}</p>
      )}
      {allEntries.length === 0 && !isLoading && (
        <p className="text-stone-500 italic">No log entries yet.</p>
      )}
      {allEntries.map((entry) => (
        <div key={entry.id} className="flex gap-3 text-stone-300">
          <span className="shrink-0 text-stone-600">{entry.timestamp}</span>
          <span className="text-amber-400/70">[{entry.kind}]</span>
          <span>{entry.text}</span>
        </div>
      ))}
      {isLoading && (
        <p className="text-stone-500 animate-pulse">...</p>
      )}
    </div>
  );
}
