# Weekly Calendar View Toggle — Design Plan

> **Status:** Implemented — shipped in v1.3.0 (2026-05-23). Week-mode header label was simplified post-review to show just the month name (the original "Apr 27 – May 3" range crowded the header on narrow phones); see commit `e024406`.

**Goal:** Let users toggle the per-goal calendar between the current full-month grid and a current-week-only grid, with the toggle living in the header (Google-Calendar-style) and persisted as a device-local preference.

**Problem:** On mobile, the 42-cell month grid takes ~6 rows of vertical space per goal. Users with many goals end up scrolling a lot, and only the trailing week (or two) is interactively meaningful anyway because of the existing 7-day completion lockout (`DayGrid.svelte:11`). A "current week" view gives ~6× more goals on screen at once on a phone.

**Approach:** Add a `calendarView: 'month' | 'week'` device-local preference (mirroring `notification-settings.ts`). When set to `'week'`, render a single 7-cell row per goal aligned to the same Sun–Sat header, with prev/next stepping one week at a time. The change is **view-layer only** — the data fetching, completion model, and progress-bar logic stay the same.

**Tech stack:** Svelte 5, TypeScript, `@capacitor/preferences`, Vitest, Playwright. No backend or schema changes.

---

## Design Decisions

### Storage and state

- New module `frontend/src/lib/view-settings.ts`, structured like `notification-settings.ts`:
  - `ViewSettings { calendarView: 'month' | 'week' }`
  - `DEFAULT_VIEW_SETTINGS = { calendarView: 'month' }`
  - `loadViewSettings()`, `saveViewSettings()`, `updateViewSettings(patch)`, `hydrateViewSettings()`
  - Exposes a `viewSettings` Svelte `writable` store hydrated on module load (skipped under Vitest, same pattern as `notification-settings.ts:63`).
  - Capacitor `Preferences` works on web too (falls back to `localStorage`), so the toggle is per-device.
- **Not** synced to the backend. Matches the existing precedent for notification settings — these are UI-only preferences.

### View model

- Add `buildWeekGrid(referenceDate: Date): CalendarCell[]` to `calendar.ts`. Returns 7 cells (Sunday of the containing week through Saturday) using the same `CalendarCell` shape, so `DayGrid.svelte` and `WeekdayHeader.svelte` can render unchanged.
- `isCurrentMonth` on each cell in week mode marks whether the cell falls in the same month as the reference date. This drives the existing muted styling for cells that spill into an adjacent month at week boundaries (no special-casing needed in `DaySquare`).
- In `App.svelte`, the existing reactive
  ```ts
  $: cells = (() => {
    const [year, month] = currentMonth.split('-').map(Number);
    return buildMonthGrid(year, month);
  })();
  ```
  becomes a branch on `view`: month → `buildMonthGrid(year, month)`; week → `buildWeekGrid(focalDate)`.

### Navigation

- Track focal state as a single `Date` (call it `focalDate`). In month mode the day component is ignored; in week mode it picks the visible week.
- `prevPeriod()` / `nextPeriod()` replace the existing `prevMonth` / `nextMonth`. They step by 1 month or 7 days based on the active view. Wire the existing prev/next buttons, keyboard arrows (`App.svelte:589-600`), and swipe handler (`App.svelte:294-308`) to the unified functions.
- "Next" disabled rule:
  - Month mode: today's month is visible (existing behavior).
  - Week mode: today's week (Sunday containing today) is visible.

### Header label

- `MonthNav.svelte` currently takes `month: string` (YYYY-MM) and computes a localized month name. Refactor to take a precomputed `label: string` plus `onPrev`/`onNext`/`disableNext`. Move label computation into `App.svelte`:
  - Month: `"April"` (current behavior, via existing month-name keys).
  - Week: `"Apr 27 – May 3"` (localized via `Intl.DateTimeFormat` with `month: 'short', day: 'numeric'`). When the week crosses a year boundary, include the year on the side that changes.
- Rename the component to `PeriodNav.svelte` to reflect the broader role. Drop the now-unused month-name lookup table.

### Data loading

- **No change to `getCalendar(currentMonth)`.** The goals list comes from this call regardless of view. In week mode we fetch the calendar of the month containing `focalDate`; the extra completions returned are harmless (they're just filtered out at render time by the 7-cell `cells` array).
- The leading-tail fetch (`prevMonthTailRange` in `App.svelte:219`) stays for month mode. In week mode it's not needed — the visible week always falls within one or two months and we already fetch the month(s) covering it. If `focalDate`'s week straddles two months, fetch both month calendars in parallel and merge their completion arrays the same way the current code merges `prev tail` + `current`.
- `getCurrentPeriodCompletions()` for progress bars is unaffected — period progress is always anchored to the real "today," not the focal date.

### Header toggle

- New component `ViewToggle.svelte` rendered inside `Header.svelte`, placed immediately to the right of `PeriodNav` (between the period label and the sync-cloud indicator).
- **Shape:** a compact two-segment pill control with the labels "M" / "W" (short codes; full words "Month" / "Week" used as `aria-label` and tooltip). Two segments, mutually exclusive, current selection styled with the accent color — same visual language as the existing `add-btn`. Two options doesn't justify a dropdown; a segmented toggle is one tap to either state and clearly shows current mode at a glance.
- **Behavior:** click writes through `updateViewSettings({ calendarView })` immediately. The component subscribes to the `viewSettings` store so it reflects the current value, including any future programmatic changes.
- **Mobile layout (≤480px):** the header already hides `user-name`, `chevron`, and `sync-cloud` at this breakpoint to make room. The toggle is small enough (≈3rem wide) to keep visible — it carries the feature's value on mobile specifically, so we keep it. If space is genuinely tight after measuring on device, the fallback is to drop the inter-segment gap and shrink to icon-only (calendar-grid vs. calendar-week-row glyphs).
- **No Profile preferences section.** The header is the only entry point. This avoids two places that can disagree and keeps Profile focused on identity + stats + export.
- i18n keys to add (top-level `viewToggle` namespace), with locked-in values:
  - `month` — "Month" / "Mês" (full words, for `aria-label` and tooltip)
  - `week` — "Week" / "Semana"
  - `monthShort` — "M" / "M" (button label)
  - `weekShort` — "W" / "S" (button label — pt-BR uses "S" for Semana)
  - Keys are kept separate from the existing `month.*` namespace (which holds month names like `month.jan`) to avoid collision.

### Edge cases and interactions

- **7-day lockout** (`DayGrid.svelte:11-36`) keeps working as-is. In week mode, the visible week is mostly interactive when current and mostly read-only for historic weeks — the lockout already enforces this from full-date comparison.
- **Future weeks:** disabled at the nav layer by `disableNext`. The lockout disables individual future cells inside the current week as a backstop.
- **Number-key shortcut** in `App.svelte:631-669` maps `1`–`9` etc. to a day-of-month for the focused goal. In week mode the same shortcut should target the corresponding day inside the focal week, not the focal month. Simplest semantics: keep the current behavior (day number within the focal *month*), but allow it only if that date falls inside the visible week. Document this in the keyboard-help text if it exists; otherwise leave a TODO comment and rely on users tapping the day square.
- **Empty state / welcome card**: unchanged. The preference only applies once a goal exists.
- **Locale**: Week starts on Sunday across all locales, matching the existing hard-coded `buildMonthGrid` choice (`calendar.ts:19`). Avoid premature locale-aware week start — not asked for, not in scope.
- **Settings on a fresh device**: default is `'month'`, so nothing visibly changes for existing users until they opt in.

### Out of scope (deliberate)

- Backend storage of the preference (cross-device sync). Day-1 it's local-only; revisit if multi-device drift becomes a real complaint.
- A header-bar quick-toggle. Profile is enough; the toggle is a settings-shaped choice, not an in-flow action. Revisit if usage patterns suggest otherwise.
- Variable week start (Monday-start). Would touch `buildMonthGrid`, `WeekdayHeader`, and progress-bar math — separate change.

---

## File-Level Impact

| File | Change |
| --- | --- |
| `frontend/src/lib/view-settings.ts` | **New.** Storage, store, hydration. |
| `frontend/src/lib/calendar.ts` | Add `buildWeekGrid(referenceDate)`. |
| `frontend/src/lib/calendar.test.ts` | Add unit tests for `buildWeekGrid`. |
| `frontend/src/App.svelte` | Replace `currentMonth: string` with `focalDate: Date`. Branch `cells`, header label, prev/next, swipe, keyboard, and `disableNext` on `view`. Subscribe to `viewSettings` store. |
| `frontend/src/lib/components/MonthNav.svelte` | Rename to `PeriodNav.svelte`; accept `label: string` instead of `month`. |
| `frontend/src/lib/components/Header.svelte` | Use `PeriodNav` with computed label prop; mount `ViewToggle`. |
| `frontend/src/lib/components/ViewToggle.svelte` | **New.** Segmented M/W control bound to the `viewSettings` store. |
| `frontend/src/lib/i18n/en.json` & `pt-BR.json` | Add `viewToggle.{month,week,monthShort,weekShort}` keys and any week-range label glue. |

No backend, schema, API, or sync changes.

---

## Testing & Verification

### Unit (Vitest)

- `calendar.test.ts`: add cases for `buildWeekGrid`:
  - Mid-month week: 7 cells, all `isCurrentMonth: true`, Sun→Sat ordering.
  - Week that spans into the prior month (e.g., reference date is `2026-05-01`, a Friday) → leading days have `isCurrentMonth: false`.
  - Week that spans into the next month.
  - Year-boundary week (Dec 28 2025 → Jan 3 2026).
  - Leap year boundary.
- New `view-settings.test.ts` (mirror `notification-settings`-style tests, if they exist): defaults, load/save round-trip, `updateViewSettings` merges.

### Component / integration

- A small Vitest test that mounts `GoalRow` with a 7-cell array and asserts it renders exactly 7 squares.
- Snapshot or DOM assertion that `ProfilePage` shows the toggle and reflects the persisted setting.

### E2E (Playwright)

Add at least two scenarios to `frontend/e2e/`:

1. **Toggle flow:** Sign in (or use the existing test-auth seed), click the header's "W" segment. Assert (a) each goal row has 7 day cells, (b) the period label matches the current week, (c) the next-period button is disabled, (d) the "W" segment shows the active style and the "M" segment does not.
2. **Week navigation:** From the week view, swipe / click prev twice and forward once. Assert the label updates correctly and that completions in those historic weeks remain visible (using seeded data already used by other E2E tests).
3. **Persistence:** After clicking "W", reload the app and confirm the week view is still active and the "W" segment is still highlighted. Click "M", reload, confirm month view restored.

### Manual / device

This is the change's whole point, so it isn't optional:

- Sideload the AAB to the physical Android device via the existing `tinytracker-devcycle` flow.
- With ≥5 active goals, confirm that the Week view fits all rows on a single screen (the explicit motivation), the swipe gesture steps by week, and the Profile toggle persists across app restart.
- Switch device language to Portuguese and confirm the new strings render.

### Regression checks before shipping

- `npm test` and the Playwright suite must be green (per `~/.claude/CLAUDE.md`: full suite, fix anything broken).
- Eyeball the existing month view — there should be **zero** visible difference when the default `'month'` setting is in effect.
- Confirm the progress bars (week/month targets) still render correct numbers in week view — they're driven by `periodCompletions`, not `cells`, so this is mainly a guard against an accidental refactor coupling them.

---

## Open Questions

_None blocking._ Mobile sizing will be eyeballed on device during implementation; if it crowds at 360–400px widths, the deferred follow-up is to hide the toggle entirely under an overflow rule (not in scope for this change).
