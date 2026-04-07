import { apiFetch } from './client';
import { buildCampaignPath } from './routes';
import type { EncounteredNPCResponse, DialogueEntry } from './types';

export function listEncounteredNPCs(campaignId: string): Promise<EncounteredNPCResponse[]> {
  return apiFetch<EncounteredNPCResponse[]>(buildCampaignPath(campaignId, 'npcs', 'encountered'));
}

export function getNPCDialogue(campaignId: string, npcId: string): Promise<DialogueEntry[]> {
  return apiFetch<DialogueEntry[]>(buildCampaignPath(campaignId, 'npcs', npcId, 'dialogue'));
}
