// Test: wrapFetch reads X-Request-Id from responses and registers it as a
// PostHog super-property so every subsequent capture includes request_id.
// This validates the cross-phase correlation plan decision:
// "frontend reads it in wrapFetch and attaches it to PostHog events as a property."

import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';

// ---------- Mocks ----------

const mocks = vi.hoisted(() => ({
  registerSpy: vi.fn(),
}));

vi.mock('posthog-js', () => {
  const fakePosthog = {
    register: mocks.registerSpy,
  };
  return { default: fakePosthog };
});

import { wrapFetch, __resetWrapFetchForTest } from '../diagnostics/net';

// Minimal breadcrumb store mock — net.ts calls emit() which needs the module.
vi.mock('../diagnostics/breadcrumbs', () => ({
  emit: vi.fn(),
}));

function makeFetchWithRequestId(requestId: string): typeof fetch {
  return vi.fn().mockResolvedValue(
    new Response(null, {
      status: 200,
      headers: { 'X-Request-Id': requestId },
    }),
  );
}

function makeFetchWithoutRequestId(): typeof fetch {
  return vi.fn().mockResolvedValue(
    new Response(null, { status: 200 }),
  );
}

describe('wrapFetch — X-Request-Id → posthog.register', () => {
  beforeEach(() => {
    mocks.registerSpy.mockReset();
    __resetWrapFetchForTest();
  });

  afterEach(() => {
    __resetWrapFetchForTest();
  });

  it('calls posthog.register({ request_id }) when response carries X-Request-Id', async () => {
    const expectedId = 'test-req-id-abc123';
    (globalThis as { fetch?: typeof fetch }).fetch = makeFetchWithRequestId(expectedId);

    wrapFetch();

    await (globalThis as { fetch: typeof fetch }).fetch('/api/v1/sync', { method: 'POST' });

    expect(mocks.registerSpy).toHaveBeenCalledTimes(1);
    expect(mocks.registerSpy).toHaveBeenCalledWith({ request_id: expectedId });
  });

  it('does not call posthog.register when response has no X-Request-Id', async () => {
    (globalThis as { fetch?: typeof fetch }).fetch = makeFetchWithoutRequestId();

    wrapFetch();

    await (globalThis as { fetch: typeof fetch }).fetch('/api/v1/goals');

    expect(mocks.registerSpy).not.toHaveBeenCalled();
  });

  it('updates the super-property on each response with a fresh request_id', async () => {
    const id1 = 'first-request-id';
    const id2 = 'second-request-id';

    let callCount = 0;
    (globalThis as { fetch?: typeof fetch }).fetch = vi.fn().mockImplementation(() => {
      callCount++;
      return Promise.resolve(
        new Response(null, {
          status: 200,
          headers: { 'X-Request-Id': callCount === 1 ? id1 : id2 },
        }),
      );
    });

    wrapFetch();

    const patchedFetch = (globalThis as { fetch: typeof fetch }).fetch;
    await patchedFetch('/api/v1/goals');
    await patchedFetch('/api/v1/sync');

    expect(mocks.registerSpy).toHaveBeenCalledTimes(2);
    expect(mocks.registerSpy).toHaveBeenNthCalledWith(1, { request_id: id1 });
    expect(mocks.registerSpy).toHaveBeenNthCalledWith(2, { request_id: id2 });
  });
});
