import { useQueryClient } from '@tanstack/react-query';
import { useRef, useState } from 'react';
import { Link, useNavigate } from 'react-router';

import {
  buildCampaignWorld,
  generateCampaignName,
  generateCampaignProposals,
  startCampaignInterview,
  startCharacterInterview,
  stepCampaignInterview,
  stepCharacterInterview,
} from '../api/startup';
import { APIError } from '../api/client';
import type { CampaignProfile, CampaignProposal, CharacterProfile } from '../api/types';
import { CampaignAttributesForm, type CampaignAttributesValue } from '../components/start/CampaignAttributesForm';
import { ChatStep, type ChatTranscriptEntry } from '../components/start/ChatStep';
import { CharacterGuidedForm, type CharacterGuidedFormData } from '../components/start/CharacterGuidedForm';
import { ConfirmationPanel } from '../components/start/ConfirmationPanel';
import { MethodPicker } from '../components/start/MethodPicker';
import { ProposalPicker, type CampaignProposalOption } from '../components/start/ProposalPicker';
import { RulesModeStep, type RulesMode } from '../components/start/RulesModeStep';
import { AppShell } from '../components/layout/AppShell';
import {
  buildCharacterProfileFromGuidedAttributes,
  summarizeCampaignProfile,
  type StartupPlaySeed,
} from '../lib/startupWorkflow';

type CampaignCreationMethod = 'interview' | 'attributes';
type CharacterCreationMethod = 'interview' | 'guided';
type WizardStep =
  | 'campaignMethod'
  | 'campaignInterview'
  | 'campaignNaming'
  | 'campaignAttributes'
  | 'campaignProposals'
  | 'rulesMode'
  | 'characterMethod'
  | 'characterInterview'
  | 'characterGuided'
  | 'confirmation'
  | 'worldBuilding';

const initialCampaignAttributes: CampaignAttributesValue = {
  genre: '',
  settingStyle: '',
  tone: '',
};

const initialGuidedCharacter: CharacterGuidedFormData = {
  name: '',
  race: '',
  class: '',
  background: '',
  alignment: '',
};

function mutationErrorMessage(error: unknown): string {
  if (error instanceof APIError) {
    return error.message;
  }

  return error instanceof Error ? error.message : 'Request failed.';
}

function cloneCampaignProfile(profile: CampaignProposalOption['profile'] | CampaignProfile): CampaignProfile {
  return {
    genre: profile.genre,
    tone: profile.tone,
    themes: [...profile.themes],
    world_type: profile.world_type,
    danger_level: profile.danger_level,
    political_complexity: profile.political_complexity,
  };
}


export function CampaignCreatePage() {
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const transcriptIdRef = useRef(0);

  const [wizardStep, setWizardStep] = useState<WizardStep>('campaignMethod');

  const [campaignMethod, setCampaignMethod] = useState<CampaignCreationMethod | null>(null);
  const [campaignMethodError, setCampaignMethodError] = useState<string | null>(null);
  const [campaignInterviewError, setCampaignInterviewError] = useState<string | null>(null);
  const [campaignAttributesError, setCampaignAttributesError] = useState<string | null>(null);

  const [campaignInterviewSessionId, setCampaignInterviewSessionId] = useState<string | null>(null);
  const [campaignInterviewTranscript, setCampaignInterviewTranscript] = useState<ChatTranscriptEntry[]>([]);
  const [campaignAttributes, setCampaignAttributes] = useState<CampaignAttributesValue>(initialCampaignAttributes);
  const [campaignProposals, setCampaignProposals] = useState<CampaignProposal[]>([]);
  const [selectedProposalKey, setSelectedProposalKey] = useState<string | null>(null);

  const [campaignProfile, setCampaignProfile] = useState<CampaignProfile | null>(null);
  const [campaignName, setCampaignName] = useState('');
  const [campaignSummary, setCampaignSummary] = useState('');
  const [rulesMode, setRulesMode] = useState<RulesMode | null>(null);

  const [characterMethod, setCharacterMethod] = useState<CharacterCreationMethod | null>(null);
  const [characterMethodError, setCharacterMethodError] = useState<string | null>(null);
  const [characterInterviewError, setCharacterInterviewError] = useState<string | null>(null);
  const [characterGuidedError, setCharacterGuidedError] = useState<string | null>(null);
  const [confirmationError, setConfirmationError] = useState<string | null>(null);

  const [characterInterviewSessionId, setCharacterInterviewSessionId] = useState<string | null>(null);
  const [characterInterviewTranscript, setCharacterInterviewTranscript] = useState<ChatTranscriptEntry[]>([]);
  const [characterGuidedForm, setCharacterGuidedForm] = useState<CharacterGuidedFormData>(initialGuidedCharacter);
  const [characterProfile, setCharacterProfile] = useState<CharacterProfile | null>(null);

  const [isStartingCampaignInterview, setIsStartingCampaignInterview] = useState(false);
  const [isSteppingCampaignInterview, setIsSteppingCampaignInterview] = useState(false);
  const [isGeneratingCampaignName, setIsGeneratingCampaignName] = useState(false);
  const [isGeneratingProposals, setIsGeneratingProposals] = useState(false);
  const [isStartingCharacterInterview, setIsStartingCharacterInterview] = useState(false);
  const [isSteppingCharacterInterview, setIsSteppingCharacterInterview] = useState(false);
  const [isBuildingWorld, setIsBuildingWorld] = useState(false);

  function nextTranscriptId(): string {
    transcriptIdRef.current += 1;
    return `startup-transcript-${transcriptIdRef.current}`;
  }

  function createTranscriptEntry(role: ChatTranscriptEntry['role'], content: string): ChatTranscriptEntry {
    return {
      id: nextTranscriptId(),
      role,
      content,
    };
  }

  function resetCharacterFlow() {
    setCharacterMethod(null);
    setCharacterMethodError(null);
    setCharacterInterviewError(null);
    setCharacterGuidedError(null);
    setConfirmationError(null);
    setCharacterInterviewSessionId(null);
    setCharacterInterviewTranscript([]);
    setCharacterGuidedForm(initialGuidedCharacter);
    setCharacterProfile(null);
  }

  function resetCampaignFlow() {
    setCampaignMethodError(null);
    setCampaignInterviewError(null);
    setCampaignAttributesError(null);
    setCampaignInterviewSessionId(null);
    setCampaignInterviewTranscript([]);
    setCampaignAttributes(initialCampaignAttributes);
    setCampaignProposals([]);
    setSelectedProposalKey(null);
    setCampaignProfile(null);
    setCampaignName('');
    setCampaignSummary('');
    resetCharacterFlow();
  }

  function applyCampaignSelection(profile: CampaignProfile, name: string, summary: string) {
    setCampaignProfile(profile);
    setCampaignName(name);
    setCampaignSummary(summary);
    resetCharacterFlow();
  }

  function handleSelectCampaignMethod(method: CampaignCreationMethod) {
    if (campaignMethod !== method) {
      resetCampaignFlow();
    }

    setCampaignMethod(method);
    setCampaignMethodError(null);
  }

  function handleSelectCharacterMethod(method: CharacterCreationMethod) {
    if (characterMethod !== method) {
      setCharacterInterviewError(null);
      setCharacterGuidedError(null);
      setConfirmationError(null);
      setCharacterInterviewSessionId(null);
      setCharacterInterviewTranscript([]);
      setCharacterGuidedForm(initialGuidedCharacter);
      setCharacterProfile(null);
    }

    setCharacterMethod(method);
    setCharacterMethodError(null);
  }

  async function handleCampaignMethodContinue(method: CampaignCreationMethod) {
    setCampaignMethodError(null);
    setCampaignInterviewError(null);
    setCampaignAttributesError(null);
    setSelectedProposalKey(null);
    setCampaignProposals([]);
    resetCharacterFlow();

    if (method === 'interview') {
      setWizardStep('campaignInterview');

      if (campaignInterviewSessionId && campaignInterviewTranscript.length > 0) {
        return;
      }

      setIsStartingCampaignInterview(true);
      try {
        const response = await startCampaignInterview();
        setCampaignInterviewSessionId(response.session_id);
        setCampaignInterviewTranscript([createTranscriptEntry('assistant', response.message)]);
      } catch (error) {
        setCampaignInterviewError(mutationErrorMessage(error));
      } finally {
        setIsStartingCampaignInterview(false);
      }

      return;
    }

    setWizardStep('campaignAttributes');
  }

  async function handleCampaignInterviewSubmit(input: string) {
    if (!campaignInterviewSessionId) {
      setCampaignInterviewError('Campaign interview session missing. Go back and start again.');
      return;
    }

    setCampaignInterviewError(null);
    setIsSteppingCampaignInterview(true);

    try {
      const response = await stepCampaignInterview(campaignInterviewSessionId, input);
      setCampaignInterviewTranscript((current) => [
        ...current,
        createTranscriptEntry('user', input),
        createTranscriptEntry('assistant', response.message),
      ]);

      if (!response.done || !response.profile) {
        return;
      }

      const profile = response.profile;
      const summary = summarizeCampaignProfile(profile);

      setCampaignProfile(profile);
      setCampaignSummary(summary);
      setCampaignInterviewSessionId(null);
      setWizardStep('campaignNaming');
      setIsGeneratingCampaignName(true);

      try {
        const nameResponse = await generateCampaignName(profile);
        applyCampaignSelection(profile, nameResponse.name, summary);
        setWizardStep('rulesMode');
      } catch (error) {
        setCampaignMethodError(mutationErrorMessage(error));
        setWizardStep('campaignMethod');
      } finally {
        setIsGeneratingCampaignName(false);
      }
    } catch (error) {
      setCampaignInterviewError(mutationErrorMessage(error));
    } finally {
      setIsSteppingCampaignInterview(false);
    }
  }

  async function handleCampaignAttributesContinue(value: CampaignAttributesValue) {
    setCampaignAttributes(value);
    setCampaignAttributesError(null);
    setIsGeneratingProposals(true);

    try {
      const response = await generateCampaignProposals({
        genre: value.genre,
        setting_style: value.settingStyle,
        tone: value.tone,
      });
      setCampaignProposals(response.proposals);
      setSelectedProposalKey(null);
      setWizardStep('campaignProposals');
    } catch (error) {
      setCampaignAttributesError(mutationErrorMessage(error));
    } finally {
      setIsGeneratingProposals(false);
    }
  }

  function handleProposalSelected(proposal: CampaignProposalOption, proposalKey: string) {
    const profile = cloneCampaignProfile(proposal.profile);

    setSelectedProposalKey(proposalKey);
    setCampaignAttributesError(null);
    setCampaignMethodError(null);
    setCampaignProfile(profile);
    setCampaignName(proposal.name);
    setCampaignSummary(proposal.summary);
  }

  function handleProposalContinue(proposal: CampaignProposalOption) {
    applyCampaignSelection(cloneCampaignProfile(proposal.profile), proposal.name, proposal.summary);
    setWizardStep('rulesMode');
  }

  async function handleCharacterMethodContinue(method: CharacterCreationMethod) {
    if (!campaignProfile) {
      setCharacterMethodError('Campaign profile missing. Finish the campaign setup first.');
      return;
    }

    setCharacterMethodError(null);
    setCharacterInterviewError(null);
    setCharacterGuidedError(null);
    setConfirmationError(null);

    if (method === 'interview') {
      setWizardStep('characterInterview');

      if (characterInterviewSessionId && characterInterviewTranscript.length > 0) {
        return;
      }

      setIsStartingCharacterInterview(true);
      try {
        const response = await startCharacterInterview(campaignProfile);
        setCharacterInterviewSessionId(response.session_id);
        setCharacterInterviewTranscript([createTranscriptEntry('assistant', response.message)]);
      } catch (error) {
        setCharacterInterviewError(mutationErrorMessage(error));
      } finally {
        setIsStartingCharacterInterview(false);
      }

      return;
    }

    setWizardStep('characterGuided');
  }

  async function handleCharacterInterviewSubmit(input: string) {
    if (!characterInterviewSessionId) {
      setCharacterInterviewError('Character interview session missing. Go back and start again.');
      return;
    }

    setCharacterInterviewError(null);
    setIsSteppingCharacterInterview(true);

    try {
      const response = await stepCharacterInterview(characterInterviewSessionId, input);
      setCharacterInterviewTranscript((current) => [
        ...current,
        createTranscriptEntry('user', input),
        createTranscriptEntry('assistant', response.message),
      ]);

      if (!response.done || !response.profile) {
        return;
      }

      setCharacterInterviewSessionId(null);
      setCharacterProfile(response.profile);
      setWizardStep('confirmation');
    } catch (error) {
      setCharacterInterviewError(mutationErrorMessage(error));
    } finally {
      setIsSteppingCharacterInterview(false);
    }
  }

  function handleGuidedCharacterSubmit(value: CharacterGuidedFormData) {
    setCharacterGuidedForm(value);
    setCharacterGuidedError(null);
    setCharacterProfile(buildCharacterProfileFromGuidedAttributes(value));
    setWizardStep('confirmation');
  }

  async function handleBeginAdventure() {
    if (!campaignProfile || !characterProfile) {
      setConfirmationError('Campaign and character details must be complete before world-building can begin.');
      return;
    }

    setConfirmationError(null);
    setWizardStep('worldBuilding');
    setIsBuildingWorld(true);

    try {
      const response = await buildCampaignWorld({
        name: campaignName,
        summary: campaignSummary,
        profile: campaignProfile,
        character_profile: characterProfile,
        rules_mode: rulesMode ?? undefined,
      });

      const startupSeed: StartupPlaySeed = {
        campaignName,
        campaignSummary,
        openingScene: response.opening_scene,
        seededAt: new Date().toISOString(),
      };

      queryClient.setQueryData(['campaign', response.campaign.id], response.campaign);
      await queryClient.invalidateQueries({ queryKey: ['campaigns'] });
      navigate(`/play/${response.campaign.id}`, { state: { startupSeed } });
    } catch (error) {
      setConfirmationError(mutationErrorMessage(error));
      setWizardStep('confirmation');
    } finally {
      setIsBuildingWorld(false);
    }
  }

  function renderWizardStep() {
    switch (wizardStep) {
      case 'campaignMethod':
        return (
          <MethodPicker<CampaignCreationMethod>
            title="Choose how to frame the campaign"
            description="Mirror the TUI startup workflow: either talk through the campaign with the guide or lock genre, setting style, and tone before generating proposals."
            methods={[
              {
                value: 'interview',
                title: 'Campaign interview',
                description: 'Answer guided questions and let the interview extract the full campaign profile and generate a fitting campaign name.',
                eyebrow: 'Conversational',
              },
              {
                value: 'attributes',
                title: 'Attribute selection',
                description: 'Choose genre, setting style, and tone first, then review generated campaign proposals before moving on.',
                eyebrow: 'Guided picks',
              },
            ]}
            selectedMethod={campaignMethod}
            onSelectMethod={handleSelectCampaignMethod}
            onContinue={handleCampaignMethodContinue}
            continueLabel="Continue"
            continueLoadingLabel="Continuing…"
            helperText="Going back later will let you switch methods, but choosing a different path resets downstream campaign and character decisions."
            errorMessage={campaignMethodError}
            isLoading={isStartingCampaignInterview}
          />
        );
      case 'campaignInterview':
        return (
          <ChatStep
            title="Describe the campaign you want to play"
            description="The guide asks follow-up questions until the campaign profile is complete, then the frontend generates the campaign name before handing off into character creation."
            transcript={campaignInterviewTranscript}
            onSubmitMessage={handleCampaignInterviewSubmit}
            onBack={() => {
              setCampaignInterviewError(null);
              setWizardStep('campaignMethod');
            }}
            submitLabel="Send reply"
            submitLoadingLabel="Sending…"
            inputLabel="Your campaign notes"
            inputPlaceholder="Dark fantasy frontier. I want doomed heroics, court intrigue, and old gods waking beneath the ice…"
            emptyStateTitle="Starting interview"
            emptyStateMessage="The guide is opening the conversation and will ask the first campaign question momentarily."
            helperText="Keep answers short if you want. The interview keeps drilling into missing details until the campaign profile is complete."
            errorMessage={campaignInterviewError}
            isLoading={isStartingCampaignInterview || isSteppingCampaignInterview || isGeneratingCampaignName}
            autoFocus={campaignInterviewTranscript.length > 0}
          />
        );
      case 'campaignNaming':
        return (
          <LoadingStageCard
            eyebrow="Campaign name"
            title="Naming your campaign"
            description="The campaign profile is locked in. Generating the campaign title before character creation begins."
          />
        );
      case 'campaignAttributes':
        return (
          <CampaignAttributesForm
            value={campaignAttributes}
            onChange={setCampaignAttributes}
            onContinue={handleCampaignAttributesContinue}
            onBack={() => {
              setCampaignAttributesError(null);
              setWizardStep('campaignMethod');
            }}
            errorMessage={campaignAttributesError}
            isLoading={isGeneratingProposals}
          />
        );
      case 'campaignProposals':
        return (
          <ProposalPicker
            proposals={campaignProposals}
            selectedProposalKey={selectedProposalKey}
            onSelectProposal={handleProposalSelected}
            onContinue={handleProposalContinue}
            onBack={() => {
              setCampaignAttributesError(null);
              setWizardStep('campaignAttributes');
            }}
            errorMessage={campaignAttributesError}
          />
        );
      case 'rulesMode':
        return (
          <RulesModeStep
            selectedMode={rulesMode}
            onSelectMode={setRulesMode}
            onContinue={() => setWizardStep('characterMethod')}
            onBack={() => {
              if (campaignMethod === 'interview') {
                setWizardStep('campaignMethod');
              } else {
                setWizardStep('campaignProposals');
              }
            }}
          />
        );
      case 'characterMethod':
        return (
          <MethodPicker<CharacterCreationMethod>
            title="Choose how to create the hero"
            description={
              campaignName
                ? `Campaign locked: ${campaignName}. Next, either talk through the character in an interview or build them from guided D&D-style attributes.`
                : 'Choose how to create the hero who will enter the opening scene.'
            }
            methods={[
              {
                value: 'interview',
                title: 'Character interview',
                description: 'Let the guide ask natural follow-up questions and extract the complete CharacterProfile for world-building.',
                eyebrow: 'Conversational',
              },
              {
                value: 'guided',
                title: 'Guided character build',
                description: 'Pick name, race, class, background, and alignment. The frontend mirrors the Go character builder semantics to produce the final profile.',
                eyebrow: 'Structured picks',
              },
            ]}
            selectedMethod={characterMethod}
            onSelectMethod={handleSelectCharacterMethod}
            onContinue={handleCharacterMethodContinue}
            onBack={() => {
              setCharacterMethodError(null);
              setWizardStep('rulesMode');
            }}
            continueLabel="Continue"
            continueLoadingLabel="Continuing…"
            helperText={campaignSummary || undefined}
            errorMessage={characterMethodError}
            isLoading={isStartingCharacterInterview}
          />
        );
      case 'characterInterview':
        return (
          <ChatStep
            title="Shape the player character"
            description={
              campaignName
                ? `Talk through the kind of protagonist who belongs in ${campaignName}. The interview finishes once the backend has a complete CharacterProfile.`
                : 'Talk through the kind of protagonist who belongs in this campaign. The interview finishes once the backend has a complete CharacterProfile.'
            }
            transcript={characterInterviewTranscript}
            onSubmitMessage={handleCharacterInterviewSubmit}
            onBack={() => {
              setCharacterInterviewError(null);
              setWizardStep('characterMethod');
            }}
            submitLabel="Send reply"
            submitLoadingLabel="Sending…"
            inputLabel="Your character notes"
            inputPlaceholder="A scarred elven ranger who watches borders, trusts no throne, and still answers the call to defend the innocent…"
            emptyStateTitle="Starting interview"
            emptyStateMessage="The guide is opening the character interview and will ask the first question momentarily."
            helperText="The backend interview stays authoritative here, so failures keep the transcript and session state in place whenever possible."
            errorMessage={characterInterviewError}
            isLoading={isStartingCharacterInterview || isSteppingCharacterInterview}
            autoFocus={characterInterviewTranscript.length > 0}
          />
        );
      case 'characterGuided':
        return (
          <CharacterGuidedForm
            value={characterGuidedForm}
            onChange={setCharacterGuidedForm}
            onSubmit={handleGuidedCharacterSubmit}
            onBack={() => {
              setCharacterGuidedError(null);
              setWizardStep('characterMethod');
            }}
            errorMessage={characterGuidedError}
          />
        );
      case 'confirmation':
        if (!campaignProfile || !characterProfile) {
          return (
            <LoadingStageCard
              eyebrow="Confirmation"
              title="Waiting for campaign details"
              description="Campaign and character selections are still settling. Go back if this state persists."
            />
          );
        }

        return (
          <ConfirmationPanel
            campaignName={campaignName}
            campaignSummary={campaignSummary}
            campaignProfile={campaignProfile}
            characterProfile={characterProfile}
            onBegin={handleBeginAdventure}
            onBack={() => {
              setConfirmationError(null);
              setWizardStep('characterMethod');
            }}
            isBeginning={isBuildingWorld}
            errorMessage={confirmationError}
          />
        );
      case 'worldBuilding':
        return (
          <LoadingStageCard
            eyebrow="World build"
            title="Building the opening scene"
            description="World generation is running. Once the campaign is ready, the play page opens with the seeded opening scene already visible."
          />
        );
      default: {
        const exhaustiveStep: never = wizardStep;
        return exhaustiveStep;
      }
    }
  }

  return (
    <AppShell
      title="Start a new campaign"
      description="Replace the old flat create form with the TUI-style startup wizard: frame the campaign, shape the hero, confirm the setup, then hand off straight into the opening scene."
      actions={
        <Link
          to="/"
          className="inline-flex items-center justify-center border border-gold/30 px-4 py-2 text-sm font-semibold uppercase tracking-wide text-champagne transition hover:border-gold hover:text-gold focus:outline-none focus:ring-2 focus:ring-gold focus:ring-offset-2 focus:ring-offset-obsidian"
        >
          Back to campaigns
        </Link>
      }
    >
      <div className="space-y-6">
        <WizardProgress currentStep={wizardStep} />
        {renderWizardStep()}
      </div>
    </AppShell>
  );
}

function WizardProgress({ currentStep }: { readonly currentStep: WizardStep }) {
  const steps = [
    {
      key: 'campaign' as const,
      label: 'Campaign setup',
      isActive: ['campaignMethod', 'campaignInterview', 'campaignNaming', 'campaignAttributes', 'campaignProposals'].includes(currentStep),
      isComplete: ['rulesMode', 'characterMethod', 'characterInterview', 'characterGuided', 'confirmation', 'worldBuilding'].includes(currentStep),
    },
    {
      key: 'rules' as const,
      label: 'Rules mode',
      isActive: currentStep === 'rulesMode',
      isComplete: ['characterMethod', 'characterInterview', 'characterGuided', 'confirmation', 'worldBuilding'].includes(currentStep),
    },
    {
      key: 'character' as const,
      label: 'Character setup',
      isActive: ['characterMethod', 'characterInterview', 'characterGuided'].includes(currentStep),
      isComplete: ['confirmation', 'worldBuilding'].includes(currentStep),
    },
    {
      key: 'confirm' as const,
      label: 'Confirmation',
      isActive: currentStep === 'confirmation',
      isComplete: currentStep === 'worldBuilding',
    },
    {
      key: 'world' as const,
      label: 'World handoff',
      isActive: currentStep === 'worldBuilding',
      isComplete: false,
    },
  ];

  return (
    <section className="border-2 border-gold/20 bg-charcoal p-5">
      <div className="flex flex-wrap gap-3">
        {steps.map((step, index) => {
          const tone = step.isActive
            ? 'border-gold bg-gold/10 text-gold'
            : step.isComplete
              ? 'border-jade/30 bg-jade/10 text-jade'
              : 'border-gold/15 bg-obsidian text-champagne/70';

          return (
            <div key={step.key} className={`inline-flex items-center gap-3 border-2 px-4 py-3 text-sm transition-all duration-200 ${tone}`}>
              <span className="font-heading text-xs font-semibold uppercase tracking-[0.2em] text-pewter/80">0{index + 1}</span>
              <span className="font-medium">{step.label}</span>
            </div>
          );
        })}
      </div>
    </section>
  );
}

function LoadingStageCard({
  eyebrow,
  title,
  description,
}: {
  readonly eyebrow: string;
  readonly title: string;
  readonly description: string;
}) {
  return (
    <section className="border-2 border-gold/20 bg-charcoal p-6">
      <div className="space-y-3 border-b-2 border-gold/20 pb-5">
        <p className="font-heading text-xs font-semibold uppercase tracking-[0.2em] text-gold">{eyebrow}</p>
        <h2 className="font-heading text-2xl font-semibold uppercase tracking-wide text-champagne">{title}</h2>
        <p className="max-w-2xl text-sm leading-7 text-champagne/70">{description}</p>
      </div>
      <div className="flex min-h-64 items-center justify-center">
        <div className="border border-gold/20 bg-obsidian px-6 py-5 text-sm text-champagne/70">Working…</div>
      </div>
    </section>
  );
}
