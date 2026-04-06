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

export type SyncEvent =
  | { id: string; type: 'goal_upsert';     timestamp: string; synced: boolean; payload: GoalUpsertPayload }
  | { id: string; type: 'goal_delete';      timestamp: string; synced: boolean; payload: GoalDeletePayload }
  | { id: string; type: 'completion_set';   timestamp: string; synced: boolean; payload: CompletionSetPayload }
  | { id: string; type: 'completion_unset'; timestamp: string; synced: boolean; payload: CompletionUnsetPayload };
