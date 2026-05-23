// Phase 3 — capture() wrapper tests.
// Covers: no-op when uninitialized, allowlist filtering, correct call shape,
// and a per-event allowed-key table.

import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';

// ---------- Mocks ----------

const mocks = vi.hoisted(() => ({
  initSpy: vi.fn(),
  identifySpy: vi.fn(),
  resetSpy: vi.fn(),
  registerSpy: vi.fn(),
  captureSpy: vi.fn(),
}));

vi.mock('posthog-js', () => {
  const fakePosthog = {
    init: (key: string, options: Record<string, unknown>) => {
      mocks.initSpy(key, options);
    },
    identify: mocks.identifySpy,
    reset: mocks.resetSpy,
    register: mocks.registerSpy,
    capture: mocks.captureSpy,
  };
  return { default: fakePosthog };
});

import {
  initPostHog,
  capture,
  __resetPostHogForTest,
} from '../analytics/posthog';

function setEnv(key: string | undefined, host: string | undefined): void {
  const env = import.meta.env as Record<string, unknown>;
  if (key === undefined) {
    delete env.VITE_POSTHOG_KEY;
  } else {
    env.VITE_POSTHOG_KEY = key;
  }
  if (host === undefined) {
    delete env.VITE_POSTHOG_HOST;
  } else {
    env.VITE_POSTHOG_HOST = host;
  }
}

function setupWithKey(): void {
  setEnv('phc_testkey', 'https://eu.i.posthog.com');
  initPostHog();
}

beforeEach(() => {
  mocks.initSpy.mockReset();
  mocks.identifySpy.mockReset();
  mocks.resetSpy.mockReset();
  mocks.registerSpy.mockReset();
  mocks.captureSpy.mockReset();
  __resetPostHogForTest();
});

afterEach(() => {
  __resetPostHogForTest();
  setEnv(undefined, undefined);
});

// ---------- No-op when uninitialized ----------

describe('capture() — no-op without PostHog key', () => {
  it('does not call posthog.capture when key is unset', () => {
    vi.spyOn(console, 'info').mockImplementation(() => {});
    setEnv(undefined, undefined);
    initPostHog();

    capture('goal_completed', { goal_id: 'g1' });

    expect(mocks.captureSpy).not.toHaveBeenCalled();
  });

  it('does not throw when called before init', () => {
    setEnv(undefined, undefined);
    initPostHog();
    expect(() => capture('goal_deleted', { goal_id: 'g1' })).not.toThrow();
  });
});

// ---------- Allowlist filtering (belt-and-braces guarantee) ----------

describe('capture() — allowlist filtering', () => {
  it('drops properties not on the allowlist for goal_created', () => {
    setupWithKey();

    // Simulate a spread of a full domain object with extra fields.
    capture('goal_created', { goal_id: 'abc' });

    expect(mocks.captureSpy).toHaveBeenCalledTimes(1);
    const [eventName, props] = mocks.captureSpy.mock.calls[0];
    expect(eventName).toBe('goal_created');
    expect(props).toEqual({ goal_id: 'abc' });
    expect(props).not.toHaveProperty('name');
    expect(props).not.toHaveProperty('color');
    expect(props).not.toHaveProperty('created_at');
  });

  it('runtime filter strips extra keys even when bypassing TS types', () => {
    setupWithKey();

    // Cast to bypass TypeScript — simulates a footgun spread at a call site.
    const dirtyProps = {
      goal_id: 'g2',
      extra_field: 'should be dropped',
    } as unknown as Parameters<typeof capture<'goal_completed'>>[1];

    capture('goal_completed', dirtyProps);

    expect(mocks.captureSpy).toHaveBeenCalledTimes(1);
    const [, props] = mocks.captureSpy.mock.calls[0];
    expect(props).toEqual({ goal_id: 'g2' });
    expect(props).not.toHaveProperty('extra_field');
  });

  it('shake_to_report_triggered sends empty props object', () => {
    setupWithKey();

    capture('shake_to_report_triggered', {} as Record<string, never>);

    expect(mocks.captureSpy).toHaveBeenCalledTimes(1);
    const [eventName, props] = mocks.captureSpy.mock.calls[0];
    expect(eventName).toBe('shake_to_report_triggered');
    expect(props).toEqual({});
  });
});

// ---------- Correct call shape ----------

describe('capture() — correct posthog.capture arguments', () => {
  it('passes the event name as the first argument', () => {
    setupWithKey();
    capture('goal_deleted', { goal_id: 'g3' });
    expect(mocks.captureSpy.mock.calls[0][0]).toBe('goal_deleted');
  });

  it('passes filtered props as the second argument', () => {
    setupWithKey();
    capture('sync_completed', {
      ok: true,
      duration_ms: 120,
      items_pushed: 5,
      items_pulled: 3,
    });
    const [, props] = mocks.captureSpy.mock.calls[0];
    expect(props).toEqual({ ok: true, duration_ms: 120, items_pushed: 5, items_pulled: 3 });
  });
});

// ---------- Per-event allowed-key table ----------

describe('capture() — per-event allowed keys', () => {
  beforeEach(() => setupWithKey());

  const cases: Array<{
    event: Parameters<typeof capture>[0];
    validProps: Parameters<typeof capture>[1];
    expectedKeys: string[];
  }> = [
    {
      event: 'goal_created',
      validProps: { goal_id: 'g1' },
      expectedKeys: ['goal_id'],
    },
    {
      event: 'goal_completed',
      validProps: { goal_id: 'g1' },
      expectedKeys: ['goal_id'],
    },
    {
      event: 'goal_deleted',
      validProps: { goal_id: 'g1' },
      expectedKeys: ['goal_id'],
    },
    {
      event: 'view_switched',
      validProps: { view: 'week' },
      expectedKeys: ['view'],
    },
    {
      event: 'sync_completed',
      validProps: { ok: true, duration_ms: 50, items_pushed: 2, items_pulled: 1 },
      expectedKeys: ['ok', 'duration_ms', 'items_pushed', 'items_pulled'],
    },
    {
      event: 'sync_failed',
      validProps: { reason_code: 'network' },
      expectedKeys: ['reason_code'],
    },
    {
      event: 'notification_permission_changed',
      validProps: { state: 'granted' },
      expectedKeys: ['state'],
    },
    {
      event: 'debug_report_submitted',
      validProps: { size_bytes: 1024 },
      expectedKeys: ['size_bytes'],
    },
    {
      event: 'shake_to_report_triggered',
      validProps: {} as Record<string, never>,
      expectedKeys: [],
    },
  ];

  for (const { event, validProps, expectedKeys } of cases) {
    it(`${event}: emits only allowed keys [${expectedKeys.join(', ') || '(none)'}]`, () => {
      mocks.captureSpy.mockReset();
      capture(event, validProps as never);
      expect(mocks.captureSpy).toHaveBeenCalledTimes(1);
      const [name, props] = mocks.captureSpy.mock.calls[0];
      expect(name).toBe(event);
      expect(Object.keys(props).sort()).toEqual([...expectedKeys].sort());
    });
  }
});

// ---------- Type-level compile error check ----------
// The block below is never executed but must parse + typecheck.
// A call with a wrong key must be a TS error — verified by ts-expect-error.

function _typeOnlyChecks() {
  // @ts-expect-error — 'wrong_key' is not a valid property for goal_created
  capture('goal_created', { wrong_key: 1 });

  // @ts-expect-error — 'view' must be 'month' | 'week', not an arbitrary string
  capture('view_switched', { view: 'list' });

  // @ts-expect-error — 'reason_code' must be one of the union literals
  capture('sync_failed', { reason_code: 'custom_error' });

  // Valid calls — these must NOT produce errors.
  capture('goal_created', { goal_id: 'g' });
  capture('goal_completed', { goal_id: 'g' });
  capture('goal_deleted', { goal_id: 'g' });
  capture('view_switched', { view: 'month' });
  capture('sync_completed', { ok: false, duration_ms: 0, items_pushed: 0, items_pulled: 0 });
  capture('sync_failed', { reason_code: 'network' });
  capture('notification_permission_changed', { state: 'denied' });
  capture('debug_report_submitted', { size_bytes: 512 });
  capture('shake_to_report_triggered', {} as Record<string, never>);
}
// Silence unused-variable warning from TS
void (_typeOnlyChecks as unknown);
