// Phase 6: shake-to-report detector.
//
// Samples the device accelerometer via `@capacitor/motion`, looks for three
// high-magnitude peaks inside a one-second sliding window, then fires a
// callback. Used to open the debug report modal from anywhere in the app.
//
// The algorithm is kept as a pure core (`pushSample` / `reset`) so tests can
// drive it with synthetic acceleration sequences, independent of the
// Capacitor runtime wiring.
//
// Thresholds match the plan and will likely need per-device tuning:
//   - mag = max(0, √(x² + y² + z²) − 9.8)
//   - peak: mag > 15 m/s²
//   - fire: 3 peaks inside 1000 ms
//   - debounce: peaks < 80 ms apart are collapsed — prevents a single sharp
//     impact from registering as three ring-down peaks on the same event.
//
// On pause / non-native platforms the Capacitor wrapper short-circuits;
// only the pure core runs in unit tests.

import { Capacitor } from '@capacitor/core';
import type { PluginListenerHandle } from '@capacitor/core';
import { App as CapApp } from '@capacitor/app';
import { Motion } from '@capacitor/motion';
import type { AccelListenerEvent } from '@capacitor/motion';
import { get } from 'svelte/store';
import { debugReportModalOpen } from '../stores';

// ---------- Tunable thresholds ----------

/** Gravity constant used to coarse-filter the magnitude signal. */
export const GRAVITY_MS2 = 9.8;
/** A sample counts as a "peak" when gravity-subtracted magnitude exceeds this. */
export const PEAK_THRESHOLD = 15;
/** Window (ms) in which {@link PEAK_COUNT} peaks must land to trigger a shake. */
export const PEAK_WINDOW_MS = 1000;
/** Number of qualifying peaks required inside the window to fire. */
export const PEAK_COUNT = 3;
/**
 * Minimum separation (ms) between two peaks. Without this, one hard impact
 * often produces several consecutive high-magnitude samples from the sensor's
 * ring-down and would fraudulently satisfy the 3-peak rule.
 */
export const PEAK_DEBOUNCE_MS = 80;

// ---------- Pure detection core ----------

interface DetectorState {
  /** Timestamps of peaks still inside the sliding window. */
  peakTimestamps: number[];
}

/**
 * Build a fresh detector state. The state holds timestamps of peaks that
 * haven't yet aged out of the window. Exported so tests can construct
 * independent detectors.
 */
export function createDetector(): DetectorState {
  return { peakTimestamps: [] };
}

/**
 * Convert a raw (x,y,z) acceleration into the gravity-subtracted magnitude
 * used by the detector. Clamped at zero: `mag − 9.8` can legitimately be
 * negative when the device is in free-fall or accelerating against gravity,
 * and we don't care about those regimes for shake detection.
 */
export function computeMagnitude(x: number, y: number, z: number): number {
  const raw = Math.sqrt(x * x + y * y + z * z);
  return Math.max(0, raw - GRAVITY_MS2);
}

/**
 * Feed one sample into the detector and return `true` if it completes a
 * qualifying shake (3 peaks within the window, each ≥ PEAK_DEBOUNCE_MS apart).
 *
 * When this returns `true` the detector's peak buffer is cleared so the
 * triggering peaks can't be counted a second time — the next shake needs a
 * fresh three-peak sequence.
 */
export function pushSample(
  state: DetectorState,
  sample: { ts: number; mag: number },
): boolean {
  if (sample.mag <= PEAK_THRESHOLD) return false;

  const last = state.peakTimestamps[state.peakTimestamps.length - 1];
  if (last !== undefined && sample.ts - last < PEAK_DEBOUNCE_MS) {
    // Debounce: this peak is too close to the previous one; treat it as part
    // of the same physical impact and drop it.
    return false;
  }

  state.peakTimestamps.push(sample.ts);

  // Drop peaks that fell out of the sliding window.
  const cutoff = sample.ts - PEAK_WINDOW_MS;
  while (
    state.peakTimestamps.length > 0 &&
    state.peakTimestamps[0] < cutoff
  ) {
    state.peakTimestamps.shift();
  }

  if (state.peakTimestamps.length >= PEAK_COUNT) {
    // Fire: clear the buffer so these peaks aren't reused by the next cycle.
    state.peakTimestamps = [];
    return true;
  }

  return false;
}

/**
 * Stateless helper kept for pure-algorithm tests. Walks a prebuilt sample
 * array and returns whether the last sample triggers a shake.
 */
export function detectShake(samples: { ts: number; mag: number }[]): boolean {
  const state = createDetector();
  let fired = false;
  for (const s of samples) {
    fired = pushSample(state, s);
  }
  return fired;
}

// ---------- Capacitor wiring ----------

let capInitLogged = false;

/**
 * Start listening for shakes. Returns a stop function that removes the motion
 * listener and any app-lifecycle hooks. Calling the stop function twice is
 * safe.
 *
 * On non-native platforms this is a no-op (accelerometer data is not
 * reliable / available in browsers here); we log one `console.info` the
 * first time so developers don't wonder why shaking the laptop does
 * nothing.
 *
 * The callback is suppressed while:
 *   - the debug report modal is already open (avoids reopening it);
 *   - the app is paused (we also tear down the motion listener entirely
 *     while paused so we aren't sampling in the background).
 */
export function startShakeDetector(onShake: () => void): () => void {
  if (!Capacitor.isNativePlatform?.()) {
    if (!capInitLogged) {
      capInitLogged = true;
      // eslint-disable-next-line no-console
      console.info('[shake] skipping: non-native platform');
    }
    return () => {
      // noop
    };
  }

  const state = createDetector();
  let accelHandle: PluginListenerHandle | undefined;
  let pauseHandle: PluginListenerHandle | undefined;
  let resumeHandle: PluginListenerHandle | undefined;
  let stopped = false;
  let paused = false;

  const handleSample = (event: AccelListenerEvent): void => {
    if (stopped || paused) return;
    // Suppress while the modal is already up. We still read samples (cheap)
    // and update the detector state; that lets the user shake-to-dismiss
    // more intuitively in the future if we want, but we never fire while
    // the modal is open.
    try {
      if (get(debugReportModalOpen)) return;
    } catch {
      // If svelte store access somehow throws, err on the side of firing.
    }
    const { x, y, z } = event.accelerationIncludingGravity;
    const mag = computeMagnitude(x, y, z);
    const fired = pushSample(state, { ts: Date.now(), mag });
    if (fired) {
      try {
        onShake();
      } catch (err) {
        // eslint-disable-next-line no-console
        console.error('[shake] onShake threw', err);
      }
    }
  };

  const startAccel = (): void => {
    if (accelHandle || stopped) return;
    // addListener returns a Promise<PluginListenerHandle>; store the handle
    // once it resolves. If a stop() call arrives before resolution we
    // remove the listener as soon as the promise settles.
    let pendingRemoved = false;
    void Motion.addListener('accel', handleSample)
      .then((handle) => {
        if (pendingRemoved || stopped) {
          void handle.remove();
          return;
        }
        accelHandle = handle;
      })
      .catch((err) => {
        // eslint-disable-next-line no-console
        console.error('[shake] failed to attach motion listener', err);
      });
    // Expose a pendingRemoved flag to the outer closure via stopAccel below.
    stopAccel = (): void => {
      pendingRemoved = true;
      if (accelHandle) {
        void accelHandle.remove();
        accelHandle = undefined;
      }
    };
  };

  // Declared here so startAccel can reassign it on each (re)start.
  let stopAccel: () => void = () => {
    if (accelHandle) {
      void accelHandle.remove();
      accelHandle = undefined;
    }
  };

  // Register lifecycle listeners so we stop sampling while backgrounded.
  void CapApp.addListener('pause', () => {
    paused = true;
    // Clear partial peak state so resume doesn't re-fire from stale peaks.
    state.peakTimestamps = [];
    stopAccel();
  })
    .then((handle) => {
      if (stopped) {
        void handle.remove();
      } else {
        pauseHandle = handle;
      }
    })
    .catch(() => {
      // App plugin unavailable; continue without pause hook.
    });

  void CapApp.addListener('resume', () => {
    paused = false;
    startAccel();
  })
    .then((handle) => {
      if (stopped) {
        void handle.remove();
      } else {
        resumeHandle = handle;
      }
    })
    .catch(() => {
      // App plugin unavailable; continue without resume hook.
    });

  // Kick off initial sampling.
  startAccel();

  return function stop(): void {
    if (stopped) return;
    stopped = true;
    stopAccel();
    if (pauseHandle) {
      void pauseHandle.remove();
      pauseHandle = undefined;
    }
    if (resumeHandle) {
      void resumeHandle.remove();
      resumeHandle = undefined;
    }
  };
}

// ---------- Test helpers ----------

export function __resetShakeForTest(): void {
  capInitLogged = false;
}
