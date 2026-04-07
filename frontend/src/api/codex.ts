import { apiFetch } from './client';
import { buildCampaignPath } from './routes';
import type { LanguageResponse, CultureResponse, BeliefSystemResponse, EconomicSystemResponse } from './types';

export function listKnownLanguages(campaignId: string): Promise<LanguageResponse[]> {
  return apiFetch<LanguageResponse[]>(buildCampaignPath(campaignId, 'codex', 'languages'));
}

export function listKnownCultures(campaignId: string): Promise<CultureResponse[]> {
  return apiFetch<CultureResponse[]>(buildCampaignPath(campaignId, 'codex', 'cultures'));
}

export function listKnownBeliefSystems(campaignId: string): Promise<BeliefSystemResponse[]> {
  return apiFetch<BeliefSystemResponse[]>(buildCampaignPath(campaignId, 'codex', 'beliefs'));
}

export function listKnownEconomicSystems(campaignId: string): Promise<EconomicSystemResponse[]> {
  return apiFetch<EconomicSystemResponse[]>(buildCampaignPath(campaignId, 'codex', 'economies'));
}
