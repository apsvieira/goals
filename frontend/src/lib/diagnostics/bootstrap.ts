// Phase 4 bootstrap: wires the breadcrumb emitter to the rest of the app
// without touching it. Called once from main.ts before Svelte mounts.
//
// Responsibilities:
//   - Restore the persisted ring buffer.
//   - Patch console.{log,info,warn,error} to mirror into breadcrumbs.
//   - Register window.onerror / unhandledrejection → error breadcrumb.
//   - Hook visibilitychange/beforeunload/Capacitor-pause → persist().
//   - Install wrapFetch() so network breadcrumbs flow automatically.
//
// The function is idempotent: calling twice is a no-op. Tests flip an
// internal "initialized" flag via __resetBootstrapForTest().

import { Capacitor } from '@capacitor/core';
import { App as CapApp } from '@capacitor/app';
import {
  emit,
  persist,
  restore,
  type BreadcrumbLevel,
} from './breadcrumbs';
import { wrapFetch } from './net';
import { startShakeDetector } from './shake';
import { initSentry } from './sentry';
import { openDebugReport } from '../stores';

let initialized = false;
// Module-level handle for the shake detector's stop function so repeated
// initDiagnostics() calls stay idempotent (we leave the existing listener
// running rather than stacking a new one).
let stopShake: (() => void) | undefined;

// Captured at bootstrap time BEFORE patching so the patched console can call
// straight into the originals without re-entering the breadcrumb pipeline.
// Using `| undefined` lets tests detect whether bootstrap has run.
let originalConsole:
  | {
      log: (...args: unknown[]) => void;
      info: (...args: unknown[]) => void;
      warn: (...args: unknown[]) => void;
      error: (...args: unknown[]) => void;
    }
  | undefined;

const MESSAGE_MAX = 200;

function stringifyArg(a: unknown): string {
  if (typeof a === 'string') return a;
  if (a instanceof Error) return a.message || a.name || 'Error';
  try {
    const s = JSON.stringify(a);
    return s === undefined ? String(a) : s;
  } catch {
    return String(a);
  }
}

function truncate(s: string, max = MESSAGE_MAX): string {
  return s.length > max ? s.slice(0, max) : s;
}

function consoleLevel(method: 'log' | 'info' | 'warn' | 'error'): BreadcrumbLevel {
  if (method === 'warn') return 'warn';
  if (method === 'error') return 'error';
  return 'info';
}

function patchConsole(): void {
  if (typeof console === 'undefined') return;
  const orig = {
    log: console.log.bind(console),
    info: console.info.bind(console),
    warn: console.warn.bind(console),
    error: console.error.bind(console),
  };
  originalConsole = orig;

  (['log', 'info', 'warn', 'error'] as const).forEach((method) => {
    const original = orig[method];
    console[method] = (...args: unknown[]): void => {
      // Always call the original first. If it throws (shouldn't, but
      // defense-in-depth) we still want the crumb.
      try {
        original(...args);
      } catch {
        // swallow — diagnostic pipeline must stay alive
      }
      try {
        const first = args.length > 0 ? stringifyArg(args[0]) : '';
        const message = truncate(first);
        const data =
          args.length > 1
            ? { args: args.slice(1).map((a) => truncate(stringifyArg(a))) }
            : undefined;
        emit({
          ts: Date.now(),
          category: 'log',
          level: consoleLevel(method),
          message,
          ...(data ? { data } : {}),
        });
      } catch {
        // never let breadcrumb emission break console
      }
    };
  });
}

function registerErrorListeners(): void {
  if (typeof window === 'undefined') return;

  window.addEventListener('error', (event: ErrorEvent) => {
    try {
      const msg = truncate(
        event.message || (event.error && String(event.error)) || 'window error',
      );
      emit({
        ts: Date.now(),
        category: 'log',
        level: 'error',
        message: msg,
        data: {
          source: 'onerror',
          ...(event.filename ? { filename: event.filename } : {}),
          ...(typeof event.lineno === 'number' ? { lineno: event.lineno } : {}),
        },
      });
    } catch {
      // noop
    }
  });

  window.addEventListener('unhandledrejection', (event: PromiseRejectionEvent) => {
    try {
      const reason = event.reason;
      const msg = truncate(
        reason instanceof Error
          ? reason.message || reason.name || 'unhandled rejection'
          : stringifyArg(reason),
      );
      emit({
        ts: Date.now(),
        category: 'log',
        level: 'error',
        message: msg,
        data: { source: 'unhandledrejection' },
      });
    } catch {
      // noop
    }
  });
}

function registerLifecycleListeners(): void {
  if (typeof document !== 'undefined') {
    // visibilitychange fires on the Document (not Window). Only persist when
    // the page transitions to hidden — the natural flush point before the
    // browser may suspend or evict the tab. No-op if IndexedDB is unavailable.
    document.addEventListener('visibilitychange', () => {
      if (document.visibilityState === 'hidden') void persist();
    });
  }
  if (typeof window !== 'undefined') {
    window.addEventListener('beforeunload', () => {
      void persist();
    });
  }

  // Native lifecycle via Capacitor App plugin. `pause` is the closest
  // equivalent to web's visibilitychange → hidden on Android.
  try {
    if (typeof Capacitor !== 'undefined' && Capacitor.isNativePlatform?.()) {
      void CapApp.addListener('pause', () => {
        void persist();
      });
    }
  } catch {
    // Capacitor plugin unavailable in this environment; skip.
  }
}

/**
 * Initialize the diagnostics pipeline. Safe to call multiple times — only
 * the first call has effect.
 */
export async function initDiagnostics(): Promise<void> {
  if (initialized) return;
  initialized = true;

  // Restore first so any crumbs emitted during the rest of bootstrap don't
  // clobber the previous session's buffer (restore replaces buffer wholesale).
  try {
    await restore();
  } catch {
    // restore is best-effort; keep going
  }

  // Initialize Sentry BEFORE patching console so Sentry's own GlobalHandlers
  // integration sees the native console/error hooks rather than our patched
  // versions. Our breadcrumb forwarder picks up scrubbed crumbs via
  // subscribe() either way.
  try {
    initSentry();
  } catch {
    // Sentry init must never block app bootstrap.
  }

  patchConsole();
  registerErrorListeners();
  registerLifecycleListeners();

  // Wrap fetch last so patched console doesn't miss fetch-related logs during
  // setup. wrapFetch is itself idempotent via a wrapped-flag.
  try {
    wrapFetch();
  } catch {
    // fetch might not exist in some SSR contexts
  }

  // Shake-to-report. startShakeDetector internally no-ops on non-native
  // platforms, so calling unconditionally is safe; we still guard here to
  // keep the intent (and the Capacitor import footprint) explicit.
  try {
    if (
      typeof Capacitor !== 'undefined' &&
      Capacitor.isNativePlatform?.() &&
      !stopShake
    ) {
      stopShake = startShakeDetector(openDebugReport);
    }
  } catch {
    // Motion plugin unavailable; continue without shake detection.
  }
}

// ---------- Test helpers ----------

/**
 * Restore the module's original console methods and internal state. Only
 * meant for vitest — the symbol is exported but not part of the public API.
 */
export function __resetBootstrapForTest(): void {
  if (originalConsole && typeof console !== 'undefined') {
    console.log = originalConsole.log;
    console.info = originalConsole.info;
    console.warn = originalConsole.warn;
    console.error = originalConsole.error;
  }
  originalConsole = undefined;
  initialized = false;
  if (stopShake) {
    try {
      stopShake();
    } catch {
      // ignore
    }
    stopShake = undefined;
  }
}

export function __isBootstrapInitialized(): boolean {
  return initialized;
}
