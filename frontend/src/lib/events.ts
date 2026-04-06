export interface GoalUpsertPayload {
  id: string;
  name: string;
  color: string;
  position: number;
  target_count?: number;
  target_period?: 'week' | 'month';
}

export interface GoalDeletePayload {
  id: string;
}

export interface CompletionSetPayload {
  goal_id: string;
  date: string;
}

export interface CompletionUnsetPayload {
  goal_id: string;
  date: string;
}

export type SyncEventPayload =
  | GoalUpsertPayload
  | GoalDeletePayload
  | CompletionSetPayload
  | CompletionUnsetPayload;

export interface SyncEvent {
  id: string;
  type: 'goal_upsert' | 'goal_delete' | 'completion_set' | 'completion_unset';
  timestamp: string;
  synced: boolean;
  payload: SyncEventPayload;
}
