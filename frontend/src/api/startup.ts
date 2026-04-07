import { apiFetch } from './client';
import type {
  BuildWorldResponse,
  CampaignInterviewResponse,
  CampaignProfile,
  CharacterInterviewResponse,
  CharacterProfile,
  GenerateCampaignNameResponse,
  GenerateCampaignProposalsRequest,
  GenerateCampaignProposalsResponse,
} from './types';

const STARTUP_BASE_PATH = '/campaigns/start';

function buildStartupPath(...segments: string[]): string {
  return segments.length > 0 ? `${STARTUP_BASE_PATH}/${segments.join('/')}` : STARTUP_BASE_PATH;
}

export function startCampaignInterview(): Promise<CampaignInterviewResponse> {
  return apiFetch<CampaignInterviewResponse>(buildStartupPath('campaign-interview'), {
    method: 'POST',
  });
}

export function stepCampaignInterview(sessionId: string, input: string): Promise<CampaignInterviewResponse> {
  return apiFetch<CampaignInterviewResponse>(buildStartupPath('campaign-interview', encodeURIComponent(sessionId)), {
    method: 'POST',
    body: { input },
  });
}

export function generateCampaignProposals(
  request: GenerateCampaignProposalsRequest,
): Promise<GenerateCampaignProposalsResponse> {
  return apiFetch<GenerateCampaignProposalsResponse>(buildStartupPath('proposals'), {
    method: 'POST',
    body: request,
  });
}

export function generateCampaignName(profile: CampaignProfile): Promise<GenerateCampaignNameResponse> {
  return apiFetch<GenerateCampaignNameResponse>(buildStartupPath('name'), {
    method: 'POST',
    body: { profile },
  });
}

export function startCharacterInterview(campaignProfile: CampaignProfile): Promise<CharacterInterviewResponse> {
  return apiFetch<CharacterInterviewResponse>(buildStartupPath('character-interview'), {
    method: 'POST',
    body: { campaign_profile: campaignProfile },
  });
}

export function stepCharacterInterview(sessionId: string, input: string): Promise<CharacterInterviewResponse> {
  return apiFetch<CharacterInterviewResponse>(buildStartupPath('character-interview', encodeURIComponent(sessionId)), {
    method: 'POST',
    body: { input },
  });
}

export function buildCampaignWorld(request: {
  name: string;
  summary: string;
  profile: CampaignProfile;
  character_profile: CharacterProfile;
  rules_mode?: string;
}): Promise<BuildWorldResponse> {
  return apiFetch<BuildWorldResponse>(buildStartupPath('world'), {
    method: 'POST',
    body: request,
  });
}
