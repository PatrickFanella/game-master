import { apiFetch } from './client';
import { buildCampaignPath } from './routes';
import type { RelationshipResponse } from './types';

export function listAwareRelationships(campaignId: string): Promise<RelationshipResponse[]> {
  return apiFetch<RelationshipResponse[]>(buildCampaignPath(campaignId, 'relationships'));
}
