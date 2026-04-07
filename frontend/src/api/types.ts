export interface CampaignCreateRequest {
  name: string;
  description: string;
  genre: string;
  tone: string;
  themes: string[];
}

export interface CampaignResponse {
  id: string;
  name: string;
  description: string;
  genre: string;
  tone: string;
  themes: string[];
  status: string;
  created_by: string;
  created_at: string;
  updated_at: string;
}

export interface CampaignListResponse {
  campaigns: CampaignResponse[];
}

export interface CharacterAbility {
  name: string;
  description?: string;
}

export interface CharacterResponse {
  id: string;
  campaign_id: string;
  user_id: string;
  name: string;
  description: string;
  stats: Record<string, unknown>;
  hp: number;
  max_hp: number;
  experience: number;
  level: number;
  status: string;
  abilities: CharacterAbility[];
  current_location_id?: string;
}

export interface LocationConnectionResponse {
  to_location_id: string;
  description: string;
  bidirectional: boolean;
  travel_time: string;
}

export interface LocationResponse {
  id: string;
  campaign_id: string;
  name: string;
  description: string;
  region: string;
  location_type: string;
  properties: Record<string, unknown>;
  connections: LocationConnectionResponse[];
}

export interface NPCResponse {
  id: string;
  campaign_id: string;
  name: string;
  description: string;
  personality: string;
  disposition: number;
  faction_id?: string;
  faction?: string;
  alive: boolean;
  hp?: number;
  stats: Record<string, unknown>;
  properties: Record<string, unknown>;
}

export interface QuestObjectiveResponse {
  id: string;
  description: string;
  completed: boolean;
  order_index: number;
}

export interface QuestResponse {
  id: string;
  campaign_id: string;
  parent_quest_id?: string;
  title: string;
  description: string;
  quest_type: string;
  status: string;
  objectives: QuestObjectiveResponse[];
}

export interface ItemResponse {
  id: string;
  campaign_id: string;
  player_character_id?: string;
  name: string;
  description: string;
  item_type: string;
  rarity: string;
  properties: Record<string, unknown>;
  equipped: boolean;
  quantity: number;
}

export interface SessionLogEntry {
  turn_number: number;
  player_input: string;
  input_type: string;
  llm_response: string;
  created_at: string;
}

export interface SessionHistoryResponse {
  entries: SessionLogEntry[];
}

export interface ActionRequest {
  input: string;
}

export interface StateChange {
  entity_type: string;
  entity_id: string;
  change_type: string;
  details: Record<string, unknown>;
}

export interface TurnResult {
  narrative: string;
  state_changes: StateChange[];
}

export type TurnResponse = TurnResult;

export interface WebSocketMessageEnvelope<TPayload = unknown> {
  type: string;
  payload: TPayload;
  timestamp: string;
}

export interface WebSocketChunkPayload {
  text: string;
}

export interface WebSocketErrorPayload {
  error: string;
}


export interface CampaignProfile {
  genre: string;
  tone: string;
  themes: string[];
  world_type: string;
  danger_level: string;
  political_complexity: string;
}

export interface CharacterProfile {
  name: string;
  concept: string;
  background: string;
  personality: string;
  motivations: string[];
  strengths: string[];
  weaknesses: string[];
}

export interface CampaignProposal {
  name: string;
  summary: string;
  profile: CampaignProfile;
}

export type StartupInterviewResponse<TProfile> =
  | {
      session_id: string;
      message: string;
      done: false;
      profile?: undefined;
    }
  | {
      session_id: string;
      message: string;
      done: true;
      profile: TProfile;
    };

export type CampaignInterviewResponse = StartupInterviewResponse<CampaignProfile>;

export interface CampaignInterviewStepRequest {
  session_id: string;
  input: string;
}

export interface GenerateCampaignProposalsRequest {
  genre: string;
  setting_style: string;
  tone: string;
}

export interface GenerateCampaignProposalsResponse {
  proposals: CampaignProposal[];
}

export interface GenerateCampaignNameRequest {
  campaign_profile: CampaignProfile;
}

export interface GenerateCampaignNameResponse {
  name: string;
}

export type CharacterInterviewResponse = StartupInterviewResponse<CharacterProfile>;

export interface CharacterInterviewStartRequest {
  campaign_profile: CampaignProfile;
}

export interface CharacterInterviewStepRequest {
  session_id: string;
  input: string;
}

export interface OpeningSceneResponse {
  narrative: string;
  choices: string[];
}

export interface BuildWorldRequest {
  name: string;
  summary: string;
  campaign_profile: CampaignProfile;
  character_profile: CharacterProfile;
}

export interface BuildWorldResponse {
  campaign: CampaignResponse;
  opening_scene: OpeningSceneResponse;
}

// NPC Panel types
export interface EncounteredNPCResponse {
  id: string;
  campaign_id: string;
  name: string;
  description: string;
  disposition?: number;
  alive: boolean;
  faction_id?: string;
}

export interface DialogueEntry {
  turn_number: number;
  player_input: string;
  llm_response: string;
  created_at: string;
}

// Facts/Lore types
export interface FactResponse {
  id: string;
  campaign_id: string;
  fact: string;
  category: string;
  source: string;
  superseded_by?: string;
  player_known: boolean;
  created_at: string;
}

// Codex types
export interface LanguageResponse {
  id: string;
  campaign_id: string;
  name: string;
  description: string;
  player_known: boolean;
  created_at: string;
}

export interface CultureResponse {
  id: string;
  campaign_id: string;
  name: string;
  language_id?: string;
  belief_system_id?: string;
  player_known: boolean;
  created_at: string;
}

export interface BeliefSystemResponse {
  id: string;
  campaign_id: string;
  name: string;
  player_known: boolean;
  created_at: string;
}

export interface EconomicSystemResponse {
  id: string;
  campaign_id: string;
  name: string;
  player_known: boolean;
  created_at: string;
}

// Relationships types
export interface RelationshipResponse {
  id: string;
  campaign_id: string;
  source_entity_type: string;
  source_entity_id: string;
  target_entity_type: string;
  target_entity_id: string;
  relationship_type: string;
  description: string;
  strength?: number;
  player_aware: boolean;
  created_at: string;
}