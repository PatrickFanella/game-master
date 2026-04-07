import { useEffect, useMemo, useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import { Link, useLocation, useParams } from 'react-router';

import { getCampaign } from '../api/campaigns';
import type { CampaignResponse, OpeningSceneResponse } from '../api/types';
import { CharacterSheet } from '../components/character/CharacterSheet';
import { InventoryPanel } from '../components/inventory/InventoryPanel';
import { AppShell } from '../components/layout/AppShell';
import { TabBar } from '../components/layout/TabBar';
import { LogPanel } from '../components/logs/LogPanel';
import { ChoiceList } from '../components/narrative/ChoiceList';
import { NarrativePanel } from '../components/narrative/NarrativePanel';
import type { NarrativeEntryItem } from '../components/narrative/NarrativeEntry';
import { PlayerInput } from '../components/narrative/PlayerInput';
import { NPCPanel } from '../components/npcs/NPCPanel';
import { QuestPanel } from '../components/quests/QuestPanel';
import { WorldPanel } from '../components/world/WorldPanel';
import { CampaignContext, useCampaignState } from '../context/CampaignContext';
import { useCampaign } from '../hooks/useCampaign';
import { useNarrative, type UseNarrativeResult } from '../hooks/useNarrative';
import { useRefreshAfterTurn } from '../hooks/useRefreshAfterTurn';
import type { StartupPlaySeed } from '../lib/startupWorkflow';

const playTabs = [
  { key: 'narrative', label: 'Narrative' },
  {
    key: 'character',
    label: 'Character',
    activeTone: 'bg-jade text-obsidian',
    hoverTone: 'border border-jade/20 bg-charcoal text-champagne/70 hover:border-jade hover:text-jade hover:bg-jade/5',
  },
  { key: 'inventory', label: 'Inventory' },
  {
    key: 'quests',
    label: 'Quests',
    activeTone: 'bg-sapphire text-obsidian',
    hoverTone: 'border border-sapphire/20 bg-charcoal text-champagne/70 hover:border-sapphire hover:text-sapphire hover:bg-sapphire/5',
  },
  {
    key: 'npcs',
    label: 'NPCs',
    activeTone: 'bg-gold text-obsidian',
    hoverTone: 'border border-gold/20 bg-charcoal text-champagne/70 hover:border-gold hover:text-gold hover:bg-gold/5',
  },
  {
    key: 'world',
    label: 'World',
    activeTone: 'bg-jade text-obsidian',
    hoverTone: 'border border-jade/20 bg-charcoal text-champagne/70 hover:border-jade hover:text-jade hover:bg-jade/5',
  },
  {
    key: 'logs',
    label: 'Logs',
    activeTone: 'bg-pewter text-obsidian',
    hoverTone: 'border border-pewter/20 bg-charcoal text-champagne/70 hover:border-pewter hover:text-pewter hover:bg-pewter/5',
  },
] as const;

type CampaignPlayTab = (typeof playTabs)[number]['key'];
type SharedNarrativeState = Pick<
  UseNarrativeResult,
  'connectionStatus' | 'entries' | 'streamingEntry' | 'suggestedChoices' | 'isLoading' | 'error' | 'sendAction'
>;

interface SeededNarrativeState {
  readonly entries: NarrativeEntryItem[];
  readonly suggestedChoices: readonly { id: string; text: string }[];
}

function queryErrorMessage(error: unknown): string {
  return error instanceof Error ? error.message : 'Unable to load the campaign.';
}

export function CampaignPlayPage() {
  const { id } = useParams();
  const location = useLocation();
  const campaignId = id?.trim() ?? '';
  const [activeTab, setActiveTab] = useState<CampaignPlayTab>('narrative');
  const startupSeed = useMemo(() => readStartupSeed(location.state), [location.state]);

  const campaignQuery = useQuery({
    queryKey: ['campaign', campaignId],
    queryFn: () => getCampaign(campaignId),
    enabled: campaignId.length > 0,
  });

  if (!campaignId) {
    return (
      <AppShell title="Play campaign" description="A campaign id is required before the play view can load." actions={<BackToCampaignsLink />}>
        <ErrorPanel message="Missing campaign id. Open a campaign from the campaign list and try again." />
      </AppShell>
    );
  }

  if (campaignQuery.isPending) {
    return (
      <AppShell title="Loading campaign" description="Preparing the play table and connecting the live narrative feed." actions={<BackToCampaignsLink />}>
        <LoadingPanel message="Loading campaign…" />
      </AppShell>
    );
  }

  if (campaignQuery.isError) {
    return (
      <AppShell title="Play campaign" description="The campaign could not be loaded." actions={<BackToCampaignsLink />}>
        <ErrorPanel message={queryErrorMessage(campaignQuery.error)} />
      </AppShell>
    );
  }

  return (
    <CampaignPlayWorkspace
      campaignId={campaignId}
      campaign={campaignQuery.data}
      activeTab={activeTab}
      onTabChange={setActiveTab}
      startupSeed={startupSeed}
    />
  );
}

interface CampaignPlayWorkspaceProps {
  readonly campaignId: string;
  readonly campaign: CampaignResponse;
  readonly activeTab: CampaignPlayTab;
  readonly onTabChange: (tab: CampaignPlayTab) => void;
  readonly startupSeed: StartupPlaySeed | null;
}

function CampaignPlayWorkspace({ campaignId, campaign, activeTab, onTabChange, startupSeed }: CampaignPlayWorkspaceProps) {
  const campaignState = useCampaignState(campaign);
  const { setActiveCampaign, setActiveCampaignId } = campaignState;

  useEffect(() => {
    setActiveCampaignId(campaignId);
  }, [campaignId, setActiveCampaignId]);

  useEffect(() => {
    setActiveCampaign(campaign);
  }, [campaign, setActiveCampaign]);

  return (
    <CampaignContext.Provider value={campaignState}>
      <CampaignPlayContent
        campaignId={campaignId}
        campaign={campaign}
        activeTab={activeTab}
        onTabChange={onTabChange}
        startupSeed={startupSeed}
      />
    </CampaignContext.Provider>
  );
}

function CampaignPlayContent({
  campaignId,
  campaign,
  activeTab,
  onTabChange,
  startupSeed,
}: {
  readonly campaignId: string;
  readonly campaign: CampaignResponse;
  readonly activeTab: CampaignPlayTab;
  readonly onTabChange: (tab: CampaignPlayTab) => void;
  readonly startupSeed: StartupPlaySeed | null;
}) {
  const narrative = useNarrative();
  const seededNarrative = useMemo(() => buildSeededNarrativeState(campaign, startupSeed, narrative.entries), [campaign, narrative.entries, startupSeed]);
  useRefreshAfterTurn(campaignId, narrative.latestResult);

  const levelUpMessage = useMemo(() => {
    const changes = narrative.latestResult?.state_changes;
    if (!changes) return null;
    const levelChange = changes.find((c) => c.entity_type === 'player_character' && c.change_type === 'level');
    if (!levelChange) return null;
    const newLevel = levelChange.details?.new_value;
    return `Level Up! You reached level ${newLevel ?? '??'}`;
  }, [narrative.latestResult]);

  return (
    <AppShell title={campaign.name} description={campaign.description || 'Live narrative play for this campaign.'} actions={<BackToCampaignsLink />}>
      <div className="space-y-6">
        {levelUpMessage ? <LevelUpBanner message={levelUpMessage} /> : null}
        <CampaignSummary campaign={campaign} campaignSummary={startupSeed?.campaignSummary ?? null} />
        <TabBar tabs={playTabs} activeTab={activeTab} onChange={onTabChange} />
        <PlayTabContent campaignId={campaignId} activeTab={activeTab} narrative={narrative} seededNarrative={seededNarrative} />
      </div>
    </AppShell>
  );
}

function PlayTabContent({
  campaignId,
  activeTab,
  narrative,
  seededNarrative,
}: {
  readonly campaignId: string;
  readonly activeTab: CampaignPlayTab;
  readonly narrative: SharedNarrativeState;
  readonly seededNarrative: SeededNarrativeState;
}) {
  switch (activeTab) {
    case 'narrative':
      return <NarrativeTab narrative={narrative} seededNarrative={seededNarrative} />;
    case 'character':
      return <CharacterSheet campaignId={campaignId} />;
    case 'inventory':
      return <InventoryPanel />;
    case 'quests':
      return <QuestPanel campaignId={campaignId} />;
    case 'npcs':
      return <NPCPanel campaignId={campaignId} />;
    case 'world':
      return <WorldPanel campaignId={campaignId} />;
    case 'logs':
      return (
        <LogPanel
          entries={seededNarrative.entries}
          streamingEntry={narrative.streamingEntry}
          isLoading={narrative.isLoading}
          error={narrative.error}
        />
      );
    default: {
      const exhaustiveTab: never = activeTab;
      return exhaustiveTab;
    }
  }
}

function NarrativeTab({
  narrative,
  seededNarrative,
}: {
  readonly narrative: SharedNarrativeState;
  readonly seededNarrative: SeededNarrativeState;
}) {
  const { campaign } = useCampaign();
  const { connectionStatus, streamingEntry, isLoading, error, sendAction } = narrative;
  const suggestedChoices = useMemo(() => {
    if (narrative.suggestedChoices.length > 0) {
      return narrative.suggestedChoices;
    }

    if (narrative.entries.length > 0) {
      return [];
    }

    return seededNarrative.suggestedChoices;
  }, [narrative.entries.length, narrative.suggestedChoices, seededNarrative.suggestedChoices]);

  return (
    <div className="grid gap-6 xl:grid-cols-[minmax(0,2fr)_minmax(18rem,1fr)]">
      <div className="space-y-4">
          <div className="flex flex-wrap items-center justify-between gap-4 border-2 border-gold/20 bg-charcoal px-5 py-4">
          <div>
            <h2 className="font-heading text-lg font-semibold uppercase tracking-wide text-champagne">Narrative log</h2>
            <p className="mt-1 text-sm text-pewter">{campaign?.name ?? 'Campaign'} · live turn-by-turn scene feed</p>
          </div>
          <ConnectionBadge status={connectionStatus} isLoading={isLoading} />
        </div>

        <NarrativePanel
          entries={seededNarrative.entries}
          streamingEntry={streamingEntry}
          isLoading={isLoading}
          emptyState={
            <div className="flex min-h-64 flex-1 flex-col items-center justify-center border border-dashed border-gold/15 bg-charcoal/50 px-6 text-center">
              <p className="font-heading text-sm font-semibold uppercase tracking-[0.2em] text-pewter/80">Awaiting first move</p>
              <p className="mt-3 max-w-md text-sm leading-7 text-pewter">
                Send an action to begin. Player moves, GM narration, system notices, and suggested choices will collect here.
              </p>
            </div>
          }
        />

        <ChoiceList
          choices={[...suggestedChoices]}
          onSelectChoice={(choiceText) => {
            sendAction(choiceText);
          }}
          disabled={isLoading}
        />

        <PlayerInput onSendAction={sendAction} disabled={connectionStatus === 'connecting'} isLoading={isLoading} autoFocus />

        {error ? <ErrorPanel message={error} /> : null}
      </div>

      <aside className="space-y-4">
        <section className="border-2 border-sapphire/20 bg-midnight/20 p-5">
          <h2 className="font-heading text-lg font-semibold uppercase tracking-wide text-champagne">Session state</h2>
          <dl className="mt-4 space-y-3 text-sm text-champagne/70">
            <SummaryRow label="Campaign status" value={campaign?.status || 'Unknown'} />
            <SummaryRow label="Live connection" value={connectionStatus} capitalize />
            <SummaryRow label="Themes" value={campaign && campaign.themes.length > 0 ? campaign.themes.join(', ') : 'None set'} />
            <SummaryRow label="Suggested choices" value={String(suggestedChoices.length)} />
          </dl>
          <p className="mt-4 text-sm leading-6 text-pewter">
            Suggested choices route through the same action pipeline as freeform input. Edit them in the input box if you want to elaborate.
          </p>
        </section>

        <section className="border-2 border-sapphire/20 bg-midnight/20 p-5">
          <h2 className="font-heading text-lg font-semibold uppercase tracking-wide text-champagne">Connection guide</h2>
          <ul className="mt-4 space-y-3 text-sm leading-6 text-champagne/70">
            <li>Use Tab buttons or keys 1–5 later to move between views.</li>
            <li>Streaming GM text appears inline before the final response settles.</li>
            <li>System messages stay in the narrative log so failures are visible, not silent.</li>
          </ul>
        </section>
      </aside>
    </div>
  );
}

function LevelUpBanner({ message }: { readonly message: string }) {
  return (
    <div className="animate-pulse border-2 border-gold/50 bg-gold/10 px-6 py-4 text-center shadow-gold">
      <p className="font-heading text-lg font-semibold uppercase tracking-[0.15em] text-gold">{message}</p>
    </div>
  );
}

function ConnectionBadge({ status, isLoading }: { readonly status: string; readonly isLoading: boolean }) {
  const tone =
    status === 'open'
      ? 'border-jade/40 bg-jade/10 text-jade'
      : status === 'connecting'
        ? 'border-gold/40 bg-gold/10 text-gold'
        : status === 'error'
          ? 'border-ruby/40 bg-ruby/10 text-ruby'
          : 'border-pewter/30 bg-pewter/10 text-pewter';

  return (
    <div className={`rounded-sm border px-3 py-1 text-xs font-semibold uppercase tracking-[0.2em] ${tone}`}>
      {isLoading ? 'GM responding' : status}
    </div>
  );
}

function CampaignSummary({
  campaign,
  campaignSummary,
}: {
  readonly campaign: CampaignResponse;
  readonly campaignSummary: string | null;
}) {
  return (
    <section className="grid gap-4 border-2 border-midnight/30 bg-midnight/10 p-5 text-sm text-champagne/70 md:grid-cols-3">
      <SummaryItem label="Genre" value={campaign.genre || 'Unspecified'} />
      <SummaryItem label="Tone" value={campaign.tone || 'Unspecified'} />
      <SummaryItem label="Themes" value={campaign.themes.length > 0 ? campaign.themes.join(', ') : 'None set'} />
      {campaignSummary ? <SummaryItem label="Startup summary" value={campaignSummary} className="md:col-span-3" /> : null}
    </section>
  );
}

function SummaryItem({
  label,
  value,
  className = '',
}: {
  readonly label: string;
  readonly value: string;
  readonly className?: string;
}) {
  return (
    <div className={`space-y-1 ${className}`.trim()}>
      <p className="font-heading text-xs font-semibold uppercase tracking-[0.2em] text-sapphire">{label}</p>
      <p className="leading-6 text-champagne">{value}</p>
    </div>
  );
}

function SummaryRow({
  label,
  value,
  capitalize = false,
}: {
  readonly label: string;
  readonly value: string;
  readonly capitalize?: boolean;
}) {
  return (
    <div className="flex items-start justify-between gap-4">
      <dt className="text-pewter">{label}</dt>
      <dd className={`text-right ${capitalize ? 'capitalize' : ''}`}>{value}</dd>
    </div>
  );
}

function LoadingPanel({ message }: { readonly message: string }) {
  return <div className="border border-gold/20 bg-charcoal p-6 text-sm text-champagne/70">{message}</div>;
}

function ErrorPanel({ message }: { readonly message: string }) {
  return <div className="border border-ruby/40 bg-ruby/10 p-6 text-sm text-ruby">{message}</div>;
}

function BackToCampaignsLink() {
  return (
    <Link
      to="/"
      className="inline-flex items-center justify-center border-2 border-gold/30 px-4 py-2 text-sm font-semibold uppercase tracking-wide text-champagne transition-all duration-200 hover:border-gold hover:text-gold focus:outline-none focus:ring-2 focus:ring-gold focus:ring-offset-2 focus:ring-offset-obsidian"
    >
      Back to campaigns
    </Link>
  );
}

function buildSeededNarrativeState(
  campaign: CampaignResponse,
  startupSeed: StartupPlaySeed | null,
  liveEntries: readonly NarrativeEntryItem[],
): SeededNarrativeState {
  const startupEntry = buildStartupEntry(campaign, startupSeed?.openingScene ?? null, startupSeed?.seededAt ?? null);
  if (!startupEntry) {
    return {
      entries: [...liveEntries],
      suggestedChoices: [],
    };
  }

  const alreadyPresent = liveEntries.some((entry) => entry.kind === 'gm' && isSameOpeningSceneEntry(entry, startupEntry));
  const entries = alreadyPresent ? [...liveEntries] : [startupEntry, ...liveEntries];

  return {
    entries,
    suggestedChoices: startupEntry.choices ?? [],
  };
}

function buildStartupEntry(
  campaign: CampaignResponse,
  openingScene: OpeningSceneResponse | null,
  seededAt: string | null,
): NarrativeEntryItem | null {
  if (!openingScene || openingScene.narrative.trim().length === 0) {
    return null;
  }

  return {
    id: `startup-opening-${campaign.id}`,
    kind: 'gm',
    text: openingScene.narrative,
    timestamp: seededAt ?? campaign.created_at,
    speaker: 'Game Master',
    choices: openingScene.choices.map((choice, index) => ({ id: `startup-choice-${index + 1}`, text: choice })),
  };
}

function isSameOpeningSceneEntry(entry: NarrativeEntryItem, startupEntry: NarrativeEntryItem): boolean {
  const entryChoices = entry.choices?.map((choice) => choice.text) ?? [];
  const startupChoices = startupEntry.choices?.map((choice) => choice.text) ?? [];

  return entry.text.trim() === startupEntry.text.trim() && entryChoices.join('\u0000') === startupChoices.join('\u0000');
}

function readStartupSeed(value: unknown): StartupPlaySeed | null {
  if (!isRecord(value) || !('startupSeed' in value)) {
    return null;
  }

  return isStartupPlaySeed(value.startupSeed) ? value.startupSeed : null;
}

function isStartupPlaySeed(value: unknown): value is StartupPlaySeed {
  return (
    isRecord(value) &&
    typeof value.campaignName === 'string' &&
    typeof value.campaignSummary === 'string' &&
    typeof value.seededAt === 'string' &&
    isOpeningSceneResponse(value.openingScene)
  );
}

function isOpeningSceneResponse(value: unknown): value is OpeningSceneResponse {
  return (
    isRecord(value) &&
    typeof value.narrative === 'string' &&
    Array.isArray(value.choices) &&
    value.choices.every((choice) => typeof choice === 'string')
  );
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null;
}
