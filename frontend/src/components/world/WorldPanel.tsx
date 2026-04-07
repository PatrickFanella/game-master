import { useState } from 'react';

import { cn } from '../../lib/cn';
import { CodexPanel } from '../codex/CodexPanel';
import { FactsPanel } from '../facts/FactsPanel';
import { RelationshipsPanel } from '../relationships/RelationshipsPanel';

interface WorldPanelProps {
  readonly campaignId: string;
  readonly className?: string;
}

type WorldSection = 'facts' | 'codex' | 'relationships';

const worldSections: readonly { key: WorldSection; label: string }[] = [
  { key: 'facts', label: 'Facts' },
  { key: 'codex', label: 'Codex' },
  { key: 'relationships', label: 'Relationships' },
] as const;

export function WorldPanel({ campaignId, className }: WorldPanelProps) {
  const [activeSection, setActiveSection] = useState<WorldSection>('facts');

  return (
    <div className={cn('space-y-5', className)}>
      <div className="flex flex-wrap gap-2 border-2 border-jade/20 bg-charcoal p-2">
        {worldSections.map((section) => (
          <button
            key={section.key}
            type="button"
            onClick={() => setActiveSection(section.key)}
            className={cn(
              'px-4 py-2 text-sm font-semibold uppercase tracking-[0.15em] transition-all duration-200 focus:outline-none focus:ring-2 focus:ring-jade focus:ring-offset-2 focus:ring-offset-obsidian',
              activeSection === section.key
                ? 'bg-jade text-obsidian'
                : 'border border-jade/20 bg-charcoal text-champagne/70 hover:border-jade hover:text-jade hover:bg-jade/5',
            )}
          >
            {section.label}
          </button>
        ))}
      </div>

      {activeSection === 'facts' && <FactsPanel campaignId={campaignId} />}
      {activeSection === 'codex' && <CodexPanel campaignId={campaignId} />}
      {activeSection === 'relationships' && <RelationshipsPanel campaignId={campaignId} />}
    </div>
  );
}
