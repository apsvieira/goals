// Phase 2 — PostHog wiring tests. posthog-js starts real network transports,
// so we mock `posthog-js` at the module boundary and assert on call shape.

import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';

// ---------- Mocks ----------

const mocks = vi.hoisted(() => ({
  initSpy: vi.fn(),
  identifySpy: vi.fn(),
  resetSpy: vi.fn(),
  registerSpy: vi.fn(),
}));

// Capture the `before_send` option from `init` so we can test the strip path.
let capturedBeforeSend: ((event: Record<string, unknown> | null) => Record<string, unknown> | null) | undefined;

vi.mock('posthog-js', () => {
  const fakePosthog = {
    init: (key: string, options: Record<string, unknown>) => {
      mocks.initSpy(key, options);
      if (typeof options.before_send === 'function') {
        capturedBeforeSend = options.before_send as typeof capturedBeforeSend;
      }
    },
    identify: mocks.identifySpy,
    reset: mocks.resetSpy,
    register: mocks.registerSpy,
  };
  return { default: fakePosthog };
});

import {
  initPostHog,
  setPostHogUser,
  stripPIIFromEvent,
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

describe('initPostHog — no env vars', () => {
  beforeEach(() => {
    mocks.initSpy.mockReset();
    mocks.identifySpy.mockReset();
    mocks.resetSpy.mockReset();
    mocks.registerSpy.mockReset();
    capturedBeforeSend = undefined;
    __resetPostHogForTest();
    setEnv(undefined, undefined);
  });

  afterEach(() => {
    __resetPostHogForTest();
    setEnv(undefined, undefined);
  });

  it('does not call posthog.init when VITE_POSTHOG_KEY is unset', () => {
    const infoSpy = vi.spyOn(console, 'info').mockImplementation(() => {});
    initPostHog();

    expect(mocks.initSpy).not.toHaveBeenCalled();
    expect(infoSpy).toHaveBeenCalledWith('PostHog disabled (no key/host)');
    infoSpy.mockRestore();
  });

  it('does not call posthog.init when VITE_POSTHOG_HOST is unset', () => {
    const infoSpy = vi.spyOn(console, 'info').mockImplementation(() => {});
    setEnv('phc_testkey', undefined);
    initPostHog();

    expect(mocks.initSpy).not.toHaveBeenCalled();
    infoSpy.mockRestore();
  });

  it('setPostHogUser is a no-op when key is missing', () => {
    vi.spyOn(console, 'info').mockImplementation(() => {});
    initPostHog();

    setPostHogUser('user-123');
    setPostHogUser(null);

    expect(mocks.identifySpy).not.toHaveBeenCalled();
    expect(mocks.resetSpy).not.toHaveBeenCalled();
  });
});

describe('initPostHog — with env vars', () => {
  beforeEach(() => {
    mocks.initSpy.mockReset();
    mocks.identifySpy.mockReset();
    mocks.resetSpy.mockReset();
    mocks.registerSpy.mockReset();
    capturedBeforeSend = undefined;
    __resetPostHogForTest();
    setEnv('phc_testkey123', 'https://eu.i.posthog.com');
  });

  afterEach(() => {
    __resetPostHogForTest();
    setEnv(undefined, undefined);
  });

  it('calls posthog.init exactly once with the configured key and options', () => {
    initPostHog();

    expect(mocks.initSpy).toHaveBeenCalledTimes(1);
    const [key, options] = mocks.initSpy.mock.calls[0];
    expect(key).toBe('phc_testkey123');
    expect(options.api_host).toBe('https://eu.i.posthog.com');
    expect(options.person_profiles).toBe('identified_only');
    expect(options.autocapture).toBe(false);
    expect(options.capture_pageview).toBe(true);
    expect(options.capture_pageleave).toBe(true);
    expect(options.disable_session_recording).toBe(true);
  });

  it('initPostHog is idempotent: calling multiple times only inits once', () => {
    initPostHog();
    initPostHog();
    initPostHog();

    expect(mocks.initSpy).toHaveBeenCalledTimes(1);
  });

  it('registers the environment super-property synchronously after init', () => {
    // environment is registered synchronously on the top-level posthog instance
    // (not via a loaded callback) so it is present on the first auto-$pageview.
    initPostHog();

    expect(mocks.registerSpy).toHaveBeenCalledTimes(1);
    const [props] = mocks.registerSpy.mock.calls[0];
    expect(props).toHaveProperty('environment');
    expect(typeof props.environment).toBe('string');
    expect(props.environment.length).toBeGreaterThan(0);
  });

  it('before_send strips goal_name from event properties', () => {
    initPostHog();

    expect(capturedBeforeSend).toBeDefined();
    const event = { properties: { goal_name: 'Run 5k', event_type: 'page_view' } };
    const result = capturedBeforeSend!(event);

    expect(result).not.toBeNull();
    const props = (result as Record<string, unknown>).properties as Record<string, unknown>;
    expect(props.goal_name).toBeUndefined();
    expect(props.event_type).toBe('page_view');
  });

  it('before_send strips email and $ip from event properties', () => {
    initPostHog();

    expect(capturedBeforeSend).toBeDefined();
    const event = { properties: { email: 'user@example.com', $ip: '1.2.3.4', $event_type: 'pageview' } };
    const result = capturedBeforeSend!(event);

    expect(result).not.toBeNull();
    const props = (result as Record<string, unknown>).properties as Record<string, unknown>;
    expect(props.email).toBeUndefined();
    expect(props.$ip).toBeUndefined();
    expect(props.$event_type).toBe('pageview');
  });

  it('before_send passes null through unchanged', () => {
    initPostHog();

    expect(capturedBeforeSend).toBeDefined();
    const result = capturedBeforeSend!(null);
    expect(result).toBeNull();
  });

  it('setPostHogUser(userId) calls posthog.identify with the id', () => {
    initPostHog();

    setPostHogUser('user-abc');

    expect(mocks.identifySpy).toHaveBeenCalledTimes(1);
    expect(mocks.identifySpy).toHaveBeenCalledWith('user-abc');
    expect(mocks.resetSpy).not.toHaveBeenCalled();
  });

  it('setPostHogUser(null) calls posthog.reset', () => {
    initPostHog();

    setPostHogUser(null);

    expect(mocks.resetSpy).toHaveBeenCalledTimes(1);
    expect(mocks.identifySpy).not.toHaveBeenCalled();
  });
});

describe('stripPIIFromEvent — defense-in-depth scrubbing', () => {
  it('removes goal_name from properties', () => {
    const event = { properties: { goal_name: 'secret goal', keep: 'yes' } };
    const out = stripPIIFromEvent(event);
    expect((out.properties as Record<string, unknown>).goal_name).toBeUndefined();
    expect((out.properties as Record<string, unknown>).keep).toBe('yes');
  });

  it('removes email and $ip from properties', () => {
    const event = { properties: { email: 'a@b.com', $ip: '10.0.0.1', other: 1 } };
    const out = stripPIIFromEvent(event);
    const props = out.properties as Record<string, unknown>;
    expect(props.email).toBeUndefined();
    expect(props.$ip).toBeUndefined();
    expect(props.other).toBe(1);
  });

  it('handles events with no properties', () => {
    const event: Record<string, unknown> = { event: '$pageview' };
    const out = stripPIIFromEvent(event);
    expect(out).toEqual({ event: '$pageview' });
  });
});
