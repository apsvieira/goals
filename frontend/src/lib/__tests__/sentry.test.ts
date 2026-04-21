// Phase 7 — Sentry wiring tests. Sentry's SDK starts transports and reaches
// for native bridges, so we mock `@sentry/capacitor` + `@sentry/svelte` at
// the module boundary and assert on call shape instead of running the real
// SDK.

import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';

// ---------- Mocks ----------
// Declared at module scope so both the mock factory (hoisted) and the tests
// reference the same spies. vi.mock() is hoisted above imports, so plain
// top-level `const` bindings can't be referenced inside — we use vi.hoisted().

const mocks = vi.hoisted(() => ({
  initSpy: vi.fn(),
  setUserSpy: vi.fn(),
  addBreadcrumbSpy: vi.fn(),
  svelteInitSpy: vi.fn(),
}));

vi.mock('@sentry/capacitor', () => ({
  init: mocks.initSpy,
  setUser: mocks.setUserSpy,
  addBreadcrumb: mocks.addBreadcrumbSpy,
}));

vi.mock('@sentry/svelte', () => ({
  init: mocks.svelteInitSpy,
}));

// Real breadcrumbs module — we want the forwarder to receive real emits.
import { emit, __resetForTest, setClock } from '../diagnostics/breadcrumbs';
import {
  initSentry,
  setSentryUser,
  stripPIIFromEvent,
  __resetSentryForTest,
} from '../diagnostics/sentry';

// Convenience: set / clear the DSN env var for a single test. Vite exposes
// env via `import.meta.env`; vitest's jsdom env is happy with direct mutation.
function setDsn(value: string | undefined): void {
  if (value === undefined) {
    delete (import.meta.env as Record<string, unknown>).VITE_SENTRY_DSN;
  } else {
    (import.meta.env as Record<string, unknown>).VITE_SENTRY_DSN = value;
  }
}

describe('initSentry — no DSN', () => {
  beforeEach(() => {
    mocks.initSpy.mockReset();
    mocks.setUserSpy.mockReset();
    mocks.addBreadcrumbSpy.mockReset();
    mocks.svelteInitSpy.mockReset();
    __resetSentryForTest();
    __resetForTest();
    setClock(() => 1_700_000_000_000);
    setDsn(undefined);
  });

  afterEach(() => {
    __resetSentryForTest();
    __resetForTest();
    setDsn(undefined);
  });

  it('does not call Sentry.init when VITE_SENTRY_DSN is unset', () => {
    const infoSpy = vi.spyOn(console, 'info').mockImplementation(() => {});
    initSentry();

    expect(mocks.initSpy).not.toHaveBeenCalled();
    expect(infoSpy).toHaveBeenCalledWith('Sentry disabled (no DSN)');
    infoSpy.mockRestore();
  });

  it('does not register a breadcrumb forwarder when DSN is missing', () => {
    vi.spyOn(console, 'info').mockImplementation(() => {});
    initSentry();

    // Emit a crumb AFTER init — forwarder should not be wired.
    emit({
      ts: 1_700_000_000_000,
      category: 'nav',
      level: 'info',
      message: 'home → settings',
    });

    expect(mocks.addBreadcrumbSpy).not.toHaveBeenCalled();
  });

  it('setSentryUser is a no-op when DSN is missing', () => {
    vi.spyOn(console, 'info').mockImplementation(() => {});
    initSentry();

    setSentryUser('user-123');
    setSentryUser(null);

    expect(mocks.setUserSpy).not.toHaveBeenCalled();
  });
});

describe('initSentry — with DSN', () => {
  beforeEach(() => {
    mocks.initSpy.mockReset();
    mocks.setUserSpy.mockReset();
    mocks.addBreadcrumbSpy.mockReset();
    mocks.svelteInitSpy.mockReset();
    __resetSentryForTest();
    __resetForTest();
    setClock(() => 1_700_000_000_000);
    setDsn('https://pub@o1.ingest.sentry.io/2');
  });

  afterEach(() => {
    __resetSentryForTest();
    __resetForTest();
    setDsn(undefined);
  });

  it('calls Sentry.init exactly once with the configured options', () => {
    initSentry();

    expect(mocks.initSpy).toHaveBeenCalledTimes(1);
    const [options, siblingInit] = mocks.initSpy.mock.calls[0];
    expect(options.dsn).toBe('https://pub@o1.ingest.sentry.io/2');
    expect(options.tracesSampleRate).toBe(0);
    expect(options.replaysSessionSampleRate).toBe(0);
    expect(options.replaysOnErrorSampleRate).toBe(0);
    // The release defaults to 'dev' unless VITE_APP_VERSION is set.
    expect(typeof options.release).toBe('string');
    expect(options.environment).toBeDefined();
    // Sibling init must be the svelte init mock so browser-side features
    // (error handling / context) boot via @sentry/svelte.
    expect(siblingInit).toBe(mocks.svelteInitSpy);
  });

  it('integrations factory keeps only the GlobalHandlers integration', () => {
    initSentry();

    const [options] = mocks.initSpy.mock.calls[0];
    const defaults = [
      { name: 'Breadcrumbs' },
      { name: 'GlobalHandlers' },
      { name: 'HttpContext' },
      { name: 'InboundFilters' },
    ];
    const kept = options.integrations(defaults);
    expect(kept).toEqual([{ name: 'GlobalHandlers' }]);
  });

  it('beforeSend strips goal_name from extra and coerces user to {id}', () => {
    initSentry();

    const [options] = mocks.initSpy.mock.calls[0];
    const event = {
      extra: { goal_name: 'Run 5k', route: 'home' },
      user: { id: 'u-9', email: 'leak@example.com', ip_address: '1.2.3.4' },
    };
    const scrubbed = options.beforeSend(event);

    expect(scrubbed.extra).toEqual({ route: 'home' });
    expect(scrubbed.user).toEqual({ id: 'u-9' });
    expect((scrubbed.user as Record<string, unknown>).email).toBeUndefined();
    expect(
      (scrubbed.user as Record<string, unknown>).ip_address,
    ).toBeUndefined();
  });

  it('forwards every scrubbed breadcrumb to Sentry.addBreadcrumb', () => {
    initSentry();

    emit({
      ts: 1_700_000_000_000,
      category: 'nav',
      level: 'info',
      message: 'home → settings',
      data: { from: 'home', to: 'settings' },
    });

    expect(mocks.addBreadcrumbSpy).toHaveBeenCalledTimes(1);
    const forwarded = mocks.addBreadcrumbSpy.mock.calls[0][0];
    expect(forwarded).toMatchObject({
      category: 'nav',
      level: 'info',
      message: 'home → settings',
    });
    // Timestamp is in seconds, not ms.
    expect(forwarded.timestamp).toBe(1_700_000_000);
    expect(forwarded.data).toEqual({ from: 'home', to: 'settings' });
  });

  it('maps breadcrumb level "warn" to Sentry "warning"', () => {
    initSentry();

    emit({
      ts: 1_700_000_000_000,
      category: 'log',
      level: 'warn',
      message: 'hi',
    });

    expect(mocks.addBreadcrumbSpy).toHaveBeenCalledTimes(1);
    const forwarded = mocks.addBreadcrumbSpy.mock.calls[0][0];
    expect(forwarded).toMatchObject({
      category: 'log',
      level: 'warning',
      message: 'hi',
    });
  });

  it('passes "error" level through unchanged', () => {
    initSentry();

    emit({
      ts: 1_700_000_000_000,
      category: 'log',
      level: 'error',
      message: 'boom',
    });

    const forwarded = mocks.addBreadcrumbSpy.mock.calls[0][0];
    expect(forwarded.level).toBe('error');
  });

  it('setSentryUser(userId) calls Sentry.setUser with id only', () => {
    initSentry();

    setSentryUser('abc');

    expect(mocks.setUserSpy).toHaveBeenCalledTimes(1);
    expect(mocks.setUserSpy).toHaveBeenCalledWith({ id: 'abc' });
  });

  it('setSentryUser(null) clears the user via Sentry.setUser(null)', () => {
    initSentry();

    setSentryUser(null);

    expect(mocks.setUserSpy).toHaveBeenCalledTimes(1);
    expect(mocks.setUserSpy).toHaveBeenCalledWith(null);
  });

  it('initSentry is idempotent: calling twice still only inits once', () => {
    initSentry();
    initSentry();
    initSentry();

    expect(mocks.initSpy).toHaveBeenCalledTimes(1);
  });

  it('idempotent init does not double-register the breadcrumb forwarder', () => {
    initSentry();
    initSentry();

    emit({
      ts: 1_700_000_000_000,
      category: 'action',
      level: 'info',
      message: 'goal completed',
      data: { goal_id: 'g-1' },
    });

    // Only one addBreadcrumb call — the forwarder was registered exactly once.
    expect(mocks.addBreadcrumbSpy).toHaveBeenCalledTimes(1);
  });
});

describe('stripPIIFromEvent — defense-in-depth scrubbing', () => {
  it('removes extra.goal_name and user.email', () => {
    const event = {
      extra: { goal_name: 'leak', keep: 1 },
      user: { id: 'u1', email: 'a@b.c' },
    };

    const out = stripPIIFromEvent(event);

    expect(out.extra).toEqual({ keep: 1 });
    expect(out.user).toEqual({ id: 'u1' });
  });

  it('handles events with no extra or user', () => {
    const event: Record<string, unknown> = { message: 'boom' };
    const out = stripPIIFromEvent(event);
    expect(out).toEqual({ message: 'boom' });
  });

  it('coerces a user without id to an empty object', () => {
    const event = { user: { email: 'a@b.c' } };
    const out = stripPIIFromEvent(event);
    expect(out.user).toEqual({});
  });
});
