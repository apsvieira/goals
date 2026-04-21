import { describe, it, expect } from 'vitest';
import {
  createDetector,
  pushSample,
  detectShake,
  computeMagnitude,
  PEAK_THRESHOLD,
  PEAK_WINDOW_MS,
  PEAK_DEBOUNCE_MS,
} from '../diagnostics/shake';

/**
 * Pure-algorithm coverage. All tests drive the detector via `pushSample` or
 * the wrapping `detectShake` helper; the Capacitor plugin is never loaded.
 */
describe('shake detection — pure core', () => {
  it('deliberate shake fires (3 peaks at 0/200/400 ms, mag=18)', () => {
    const samples = [
      { ts: 0, mag: 18 },
      { ts: 200, mag: 18 },
      { ts: 400, mag: 18 },
    ];
    expect(detectShake(samples)).toBe(true);
  });

  it("walking noise doesn't fire (10 samples, mag in [2, 12])", () => {
    // Deterministic pseudo-random sequence in [2, 12] — seeded by index so
    // the test is reproducible and doesn't rely on Math.random.
    const samples = Array.from({ length: 10 }, (_, i) => ({
      ts: i * 100,
      mag: 2 + ((i * 7) % 10),
    }));
    // Sanity: nothing in the sequence should cross the 15 m/s² threshold.
    for (const s of samples) expect(s.mag).toBeLessThanOrEqual(PEAK_THRESHOLD);
    expect(detectShake(samples)).toBe(false);
  });

  it("single hard impact doesn't fire (one sample at mag=30)", () => {
    expect(detectShake([{ ts: 0, mag: 30 }])).toBe(false);
  });

  it("too-fast peaks don't fire (ts=0/30/60 all < debounce)", () => {
    // All three peaks are inside PEAK_DEBOUNCE_MS of the previous one, so
    // the detector should collapse them into a single impact and not fire.
    const samples = [
      { ts: 0, mag: 20 },
      { ts: 30, mag: 20 },
      { ts: 60, mag: 20 },
    ];
    // Guard: confirm our fixture actually violates the debounce rule so the
    // test fails if PEAK_DEBOUNCE_MS changes out from under us.
    expect(PEAK_DEBOUNCE_MS).toBeGreaterThan(60);
    expect(detectShake(samples)).toBe(false);
  });

  it("peaks outside 1-second window don't fire (ts=0/500/1500)", () => {
    // First peak ages out before the third arrives, leaving only two peaks
    // inside the sliding window.
    const samples = [
      { ts: 0, mag: 20 },
      { ts: 500, mag: 20 },
      { ts: 1500, mag: 20 },
    ];
    expect(detectShake(samples)).toBe(false);
  });

  it('peaks exactly at window edge fire (ts=0/500/999)', () => {
    const samples = [
      { ts: 0, mag: 20 },
      { ts: 500, mag: 20 },
      { ts: 999, mag: 20 },
    ];
    // Sanity: the span is inside the window by one ms.
    expect(999 - 0).toBeLessThan(PEAK_WINDOW_MS);
    expect(detectShake(samples)).toBe(true);
  });

  it("mag under threshold doesn't count (3 samples at mag=14)", () => {
    const samples = [
      { ts: 0, mag: 14 },
      { ts: 200, mag: 14 },
      { ts: 400, mag: 14 },
    ];
    expect(detectShake(samples)).toBe(false);
  });

  it('fires once per qualifying sequence (4th peak does not re-fire)', () => {
    const state = createDetector();
    // Three peaks → fire.
    expect(pushSample(state, { ts: 0, mag: 20 })).toBe(false);
    expect(pushSample(state, { ts: 200, mag: 20 })).toBe(false);
    expect(pushSample(state, { ts: 400, mag: 20 })).toBe(true);

    // 4th peak 100 ms after the firing event must not re-fire: the 3 peaks
    // that triggered were consumed.
    expect(pushSample(state, { ts: 500, mag: 20 })).toBe(false);

    // Two additional qualifying peaks after a fresh run-up should fire
    // again (3 total new peaks: ts=500, 700, 900).
    expect(pushSample(state, { ts: 700, mag: 20 })).toBe(false);
    expect(pushSample(state, { ts: 900, mag: 20 })).toBe(true);
  });
});

describe('shake detection — computeMagnitude', () => {
  it('subtracts gravity from the vector magnitude', () => {
    // A device sitting still on a table reads |a| ≈ 9.8 m/s², so the
    // gravity-subtracted magnitude should be near zero.
    expect(computeMagnitude(0, 0, 9.8)).toBeCloseTo(0, 5);
  });

  it('clamps at zero when vector magnitude is below gravity', () => {
    // Free-fall / decelerating past gravity: |a| < 9.8. Must not go
    // negative — the detector operates on non-negative magnitudes.
    expect(computeMagnitude(0, 0, 5)).toBe(0);
  });

  it('returns positive magnitude for a vigorous shake', () => {
    // A shake producing ~25 m/s² on one axis clears the 15 m/s² peak
    // threshold once gravity is removed.
    const mag = computeMagnitude(25, 0, 0);
    expect(mag).toBeGreaterThan(PEAK_THRESHOLD);
  });
});
