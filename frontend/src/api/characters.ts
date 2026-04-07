import { apiFetch } from './client';
import { buildCampaignPath } from './routes';
import type { CharacterAbility, CharacterResponse, FeatResponse, ItemResponse, SkillResponse } from './types';

function buildCharacterPath(campaignId: string, suffix?: 'inventory' | 'abilities' | 'feats' | 'skills'): string {
  return suffix ? buildCampaignPath(campaignId, 'character', suffix) : buildCampaignPath(campaignId, 'character');
}

export function getCampaignCharacter(campaignId: string): Promise<CharacterResponse> {
  return apiFetch<CharacterResponse>(buildCharacterPath(campaignId));
}

export function getCampaignCharacterInventory(campaignId: string): Promise<ItemResponse[]> {
  return apiFetch<ItemResponse[]>(buildCharacterPath(campaignId, 'inventory'));
}

export function getCampaignCharacterAbilities(campaignId: string): Promise<CharacterAbility[]> {
  return apiFetch<CharacterAbility[]>(buildCharacterPath(campaignId, 'abilities'));
}

export function getCampaignCharacterFeats(campaignId: string): Promise<FeatResponse[]> {
  return apiFetch<FeatResponse[]>(buildCharacterPath(campaignId, 'feats'));
}

export function getCampaignCharacterSkills(campaignId: string): Promise<SkillResponse[]> {
  return apiFetch<SkillResponse[]>(buildCharacterPath(campaignId, 'skills'));
}
