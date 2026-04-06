import { describe, it, expect, beforeEach, afterEach } from 'vitest';
import { deleteDB } from 'idb';
import { generateId, generateCompletionId } from '../api';
import {
  initStorage,
  resetDB,
  saveLocalCompletion,
  getLocalCompletions,
} from '../storage';

// UUID v4 format: 8-4-4-4-12 hex digits
const UUID_V4_REGEX = /^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/i;

describe('generateId', () => {
  it('should return a valid UUID v4', () => {
    const id = generateId();
    expect(id).toMatch(UUID_V4_REGEX);
  });

  it('should return unique IDs on each call', () => {
    const ids = new Set(Array.from({ length: 100 }, () => generateId()));
    expect(ids.size).toBe(100);
  });

  it('should not include the "local-" prefix', () => {
    const id = generateId();
    expect(id).not.toMatch(/^local-/);
  });
});

describe('Completion ID format', () => {
  beforeEach(() => {
    resetDB();
  });

  afterEach(async () => {
    resetDB();
    try {
      await deleteDB('goal-tracker');
    } catch {
      // Ignore cleanup errors
    }
  });

  it('should create completions with deterministic goalId:date IDs', async () => {
    await initStorage();

    const goalId = 'abc-123';
    const date = '2026-04-05';
    const expectedId = generateCompletionId(goalId, date);

    const completion = {
      id: generateCompletionId(goalId, date),
      goal_id: goalId,
      date,
      created_at: new Date().toISOString(),
    };

    await saveLocalCompletion(completion);

    const completions = await getLocalCompletions('2026-04');
    expect(completions).toHaveLength(1);
    expect(completions[0].id).toBe(expectedId);
    expect(completions[0].id).toContain(':');
  }, 10000);

  it('should produce the same ID for the same goal and date', () => {
    const goalId = 'goal-xyz';
    const date = '2026-01-15';
    const id1 = generateCompletionId(goalId, date);
    const id2 = generateCompletionId(goalId, date);
    expect(id1).toBe(id2);
  });

  it('should produce different IDs for different dates', () => {
    const goalId = 'goal-xyz';
    const id1 = generateCompletionId(goalId, '2026-01-15');
    const id2 = generateCompletionId(goalId, '2026-01-16');
    expect(id1).not.toBe(id2);
  });

  it('should produce different IDs for different goals', () => {
    const date = '2026-01-15';
    const id1 = generateCompletionId('goal-a', date);
    const id2 = generateCompletionId('goal-b', date);
    expect(id1).not.toBe(id2);
  });

  it('should work with existing local-* prefixed IDs in storage', async () => {
    await initStorage();

    // Save a completion with old local-* ID format
    const oldCompletion = {
      id: 'local-1711234567890-abc1234',
      goal_id: 'old-goal',
      date: '2026-03-01',
      created_at: new Date().toISOString(),
    };
    await saveLocalCompletion(oldCompletion);

    // Save a completion with new deterministic ID format
    const newCompletion = {
      id: 'new-goal:2026-04-05',
      goal_id: 'new-goal',
      date: '2026-04-05',
      created_at: new Date().toISOString(),
    };
    await saveLocalCompletion(newCompletion);

    // Both should coexist in storage
    const marchCompletions = await getLocalCompletions('2026-03');
    const aprilCompletions = await getLocalCompletions('2026-04');
    expect(marchCompletions).toHaveLength(1);
    expect(marchCompletions[0].id).toBe('local-1711234567890-abc1234');
    expect(aprilCompletions).toHaveLength(1);
    expect(aprilCompletions[0].id).toBe('new-goal:2026-04-05');
  }, 10000);
});
