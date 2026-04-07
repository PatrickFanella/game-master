import { useContext } from 'react';
import { useQuery } from '@tanstack/react-query';

import { getCampaignCharacter } from '../../api/characters';
import { CampaignContext } from '../../context/CampaignContext';
import { cn } from '../../lib/cn';
import { AbilityList } from './AbilityList';
import { FeatBrowser } from './FeatBrowser';
import { SkillTree } from './SkillTree';
import { StatsBlock } from './StatsBlock';

interface CharacterSheetProps {
  readonly campaignId?: string;
  readonly className?: string;
}

export function CharacterSheet({ campaignId, className }: CharacterSheetProps) {
  const campaign = useContext(CampaignContext);
  const activeCampaignId = campaignId ?? campaign?.campaignId ?? '';

  const characterQuery = useQuery({
    queryKey: ['campaign', activeCampaignId, 'character'],
    queryFn: () => getCampaignCharacter(activeCampaignId),
    enabled: activeCampaignId.length > 0,
  });

  if (!activeCampaignId) {
    return (
      <PanelState
        className={className}
        title="Character unavailable"
        message="Open a campaign before loading the character sheet."
        tone="empty"
      />
    );
  }

  if (characterQuery.isPending) {
    return <PanelState className={className} title="Loading character" message="Fetching the latest character sheet…" tone="loading" />;
  }

  if (characterQuery.isError) {
    return <PanelState className={className} title="Character failed to load" message={queryErrorMessage(characterQuery.error)} tone="error" />;
  }

  const character = characterQuery.data;

  return (
    <div className={cn('grid gap-6 xl:grid-cols-[minmax(0,1.35fr)_minmax(18rem,0.95fr)]', className)}>
      <div className="space-y-6">
        <section className="deco-corners deco-corners-jade deco-pattern border-2 border-jade/25 bg-charcoal p-6">
          <div className="flex flex-wrap items-start justify-between gap-4">
            <div className="space-y-3">
              <div>
                <p className="font-heading text-xs font-semibold uppercase tracking-[0.2em] text-jade">Character</p>
                <h2 className="font-heading mt-2 text-2xl font-semibold uppercase tracking-[0.1em] text-champagne">{character.name}</h2>
              </div>
              <p className="max-w-2xl text-sm leading-7 text-champagne/70">
                {character.description.trim().length > 0 ? character.description : 'No character description has been recorded yet.'}
              </p>
            </div>
            <div className="inline-flex rounded-sm border border-jade/30 bg-jade/10 px-3 py-1 text-xs font-semibold uppercase tracking-[0.2em] text-jade">
              {humanizeInlineValue(character.status)}
            </div>
          </div>

          <div className="mt-6 grid gap-4 sm:grid-cols-3">
            <MetricCard label="HP" value={`${character.hp} / ${character.max_hp}`} accent="text-jade" />
            <MetricCard label="Level" value={String(character.level)} accent="text-gold" />
            <XPProgressCard level={character.level} experience={character.experience} />
          </div>
        </section>

        <StatsBlock stats={character.stats} />
      </div>

      <div className="space-y-6">
        <section className="border-2 border-jade/20 bg-charcoal p-6">
          <h3 className="font-heading text-lg font-semibold uppercase tracking-[0.1em] text-champagne">Current standing</h3>
          <dl className="mt-4 space-y-3 text-sm text-champagne/70">
            <SummaryRow label="Status" value={humanizeInlineValue(character.status)} />
            <SummaryRow label="Character ID" value={character.id} mono />
            <SummaryRow
              label="Current location"
              value={character.current_location_id ? character.current_location_id : 'Location not recorded'}
              mono={Boolean(character.current_location_id)}
            />
          </dl>
        </section>

        <AbilityList abilities={character.abilities} />

        {campaign?.campaign?.rules_mode === 'crunch' && (
          <>
            <FeatBrowser campaignId={activeCampaignId} />
            <SkillTree campaignId={activeCampaignId} />
          </>
        )}
      </div>
    </div>
  );
}

/** Returns cumulative XP needed to reach the next level. Mirrors progression/leveling.go. */
function nextLevelThreshold(level: number): number {
  return 1000 * level * (level + 1) / 2;
}

function XPProgressCard({ level, experience }: { readonly level: number; readonly experience: number }) {
  const threshold = nextLevelThreshold(level);
  const prevThreshold = level > 1 ? nextLevelThreshold(level - 1) : 0;
  const xpInLevel = experience - prevThreshold;
  const xpNeeded = threshold - prevThreshold;
  const pct = xpNeeded > 0 ? Math.min(100, Math.round((xpInLevel / xpNeeded) * 100)) : 100;

  return (
    <div className="rounded-none border border-jade/20 bg-charcoal/80 p-4 transition-all duration-200 hover:border-jade/40">
      <p className="text-xs font-semibold uppercase tracking-[0.2em] text-pewter/80">Experience</p>
      <p className="mt-2 text-lg font-semibold tracking-tight text-sapphire">
        {experience} <span className="text-sm text-pewter">/ {threshold}</span>
      </p>
      <div className="mt-2 h-2 w-full overflow-hidden bg-midnight/50">
        <div
          className="h-full bg-sapphire transition-all duration-500"
          style={{ width: `${pct}%` }}
        />
      </div>
      <p className="mt-1 text-[10px] uppercase tracking-[0.15em] text-pewter/70">{pct}% to level {level + 1}</p>
    </div>
  );
}

function MetricCard({
  label,
  value,
  accent,
}: {
  readonly label: string;
  readonly value: string;
  readonly accent: string;
}) {
  return (
    <div className="rounded-none border border-jade/20 bg-charcoal/80 p-4 transition-all duration-200 hover:border-jade/40">
      <p className="text-xs font-semibold uppercase tracking-[0.2em] text-pewter/80">{label}</p>
      <p className={cn('mt-3 text-2xl font-semibold tracking-tight text-champagne', accent)}>{value}</p>
    </div>
  );
}

function SummaryRow({
  label,
  value,
  mono = false,
}: {
  readonly label: string;
  readonly value: string;
  readonly mono?: boolean;
}) {
  return (
    <div className="flex items-start justify-between gap-4">
      <dt className="text-pewter">{label}</dt>
      <dd className={cn('max-w-[60%] text-right text-champagne/80', mono ? 'font-mono text-xs leading-6 text-champagne/60' : '')}>{value}</dd>
    </div>
  );
}

function PanelState({
  title,
  message,
  tone,
  className,
}: {
  readonly title: string;
  readonly message: string;
  readonly tone: 'loading' | 'error' | 'empty';
  readonly className?: string;
}) {
  const toneClasses =
    tone === 'error'
      ? 'border-ruby/40 bg-ruby/10 text-ruby'
      : 'border-gold/20 bg-charcoal text-champagne/80';

  return (
    <section className={cn('border p-6', toneClasses, className)}>
      <h2 className="font-heading text-lg font-semibold uppercase tracking-[0.1em] text-champagne">{title}</h2>
      <p className="mt-2 text-sm leading-6 text-inherit">{message}</p>
    </section>
  );
}

function queryErrorMessage(error: unknown): string {
  return error instanceof Error ? error.message : 'Unable to load the character sheet.';
}

function humanizeInlineValue(value: string): string {
  return value
    .replace(/[_-]+/g, ' ')
    .replace(/\s+/g, ' ')
    .trim()
    .replace(/\b\w/g, (letter) => letter.toUpperCase());
}
