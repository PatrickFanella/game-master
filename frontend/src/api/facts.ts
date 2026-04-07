import { apiFetch } from './client';
import { buildCampaignPath } from './routes';
import type { FactResponse } from './types';

export function listKnownFacts(campaignId: string): Promise<FactResponse[]> {
  return apiFetch<FactResponse[]>(buildCampaignPath(campaignId, 'facts'));
}
