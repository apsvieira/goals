# Android UI Fixes Design

## Problems

1. The Android status bar (top) and navigation bar (bottom) remain visible after login instead of auto-hiding.
2. The back button does nothing — it should navigate back or exit the app.
3. The app layout doesn't account for system bar overlays, causing content to be hidden behind them.

## Solution

### 1. Immersive Mode (Auto-hide System Bars)

In `MainActivity.java`, configure immersive sticky mode after `super.onCreate()`:

- **API 30+ (Android 11+):** Use `WindowInsetsController` with `BEHAVIOR_SHOW_TRANSIENT_BARS_BY_SWIPE` to hide both status and navigation bars.
- **API 24-29 (minSdk):** Use legacy `SYSTEM_UI_FLAG_IMMERSIVE_STICKY | SYSTEM_UI_FLAG_HIDE_NAVIGATION | SYSTEM_UI_FLAG_FULLSCREEN` flags.
- Make system bars transparent so they overlay the app content when revealed via swipe.

### 2. Edge-to-Edge Layout with Safe Area Insets

Enable edge-to-edge rendering and use CSS safe-area insets to protect content:

- **`MainActivity.java`** — Add `WindowCompat.setDecorFitsSystemWindows(window, false)` so the WebView extends behind system bars.
- **`index.html`** — Add `viewport-fit=cover` to the viewport meta tag to expose `env(safe-area-inset-*)` CSS values.
- **`App.svelte` CSS** — Add `padding-top: env(safe-area-inset-top)` and `padding-bottom: env(safe-area-inset-bottom)` to `.app-container`.

Result: the app draws behind bars at all times, content stays in the safe area, bars overlay transparently on swipe with no layout reflow.

### 3. Back Button Handling

Add a `backButton` listener in `App.svelte` using the `@capacitor/app` plugin (already installed):

1. If a modal or profile panel is open → close it.
2. If on a non-home route (e.g. `/privacy`) → `history.back()`.
3. If on the home screen with nothing open → `App.exitApp()`.

## Files to Modify

| File | Changes |
|------|---------|
| `frontend/android/app/src/main/java/software/maleficent/tinytracker/MainActivity.java` | Immersive mode + edge-to-edge setup |
| `frontend/index.html` | Add `viewport-fit=cover` to viewport meta |
| `frontend/src/App.svelte` | Safe-area CSS padding + back button listener |
