# Blank Screen on Android After App Update

**Date:** 2026-04-04
**Severity:** Critical (app unusable)
**Affected versions:** v1.0.3 through v1.0.8
**Fixed in:** v1.0.9

## Symptom

After updating via Play Store, the app showed a blank cream/beige screen (`#F7F3E3` background) with no content, spinner, or error. Restarting and clearing cache from Android settings had no effect. Clearing *app data* restored functionality.

## Root Cause

The service worker (`sw.js`) was registering inside Capacitor's Android WebView and caching assets with a stale, hardcoded `CACHE_VERSION` (`20260108-000000`) that hadn't been updated since January 2026.

The chain of failure:

1. On first install, the SW registered on Capacitor's local server (`https://localhost`) and pre-cached `index.html` using a cache-first strategy.
2. When the app updated via Play Store, the APK's bundled web assets changed (Vite produces content-hashed filenames like `index-DAc8cv2h.js`).
3. Because `CACHE_VERSION` never changed, the old SW remained active and kept serving its cached `index.html`, which referenced JS/CSS filenames from the old build.
4. Those old filenames no longer existed in the new APK's assets.
5. The `<script type="module">` failed silently (modules don't throw visible errors on load failure), resulting in the Svelte app never mounting.
6. Only the CSS background color rendered, producing the blank cream screen.

## Why It Was Hard to Diagnose

- No native crash (the WebView loaded fine).
- No JS console errors visible in `adb logcat` (release builds don't expose WebView console, and ES module load failures are silent).
- The loading spinner (which shows during auth check) never appeared because the JS didn't execute at all.
- The web deployment at `goal-tracker-app.fly.dev` worked fine because the server always served fresh `index.html` — the stale cache only affected the Capacitor local server.

## Fix (v1.0.9, commit 22b010d)

Three changes:

1. **Disabled SW on native** (`index.html`): Added a guard that skips `navigator.serviceWorker.register()` when `location.hostname === 'localhost'` or `location.protocol === 'capacitor:'`. The SW is unnecessary on Capacitor since assets are served directly from the APK.

2. **Automated cache versioning** (`vite.config.ts`): Added a Vite `closeBundle` plugin that replaces a `__BUILD_VERSION__` placeholder in `sw.js` with a build timestamp (`YYYYMMDDHHMMSS`). Every build now produces a unique cache version.

3. **Replaced hardcoded version** (`public/sw.js`): Changed `CACHE_VERSION` from a hardcoded date string to the `__BUILD_VERSION__` placeholder.

## Lessons

- Service workers in Capacitor WebViews are an anti-pattern. Capacitor already handles local asset serving; a SW adds a redundant caching layer that can go stale across app updates.
- Cache version strings must be automated. A manual "update this before deploy" comment is a guaranteed time bomb.
- ES module load failures are silent. When debugging a blank WebView, check whether the JS bundle actually loaded before investigating application logic.
