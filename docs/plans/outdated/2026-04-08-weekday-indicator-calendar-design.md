# Weekday Indicator & Aligned Calendar Grid — Design Plan

> **Status:** Implemented and shipped in **v1.0.13** on 2026-04-08.
> - PR: apsvieira/goals#2 (merge commit `5d0fa91`)
> - Release CI: Android Build run `24163932435` — signed AAB uploaded to Play Store internal track
> - Key commits on the feature branch (all in `main` after merge):
>   - `1478d11` feat(calendar): add buildMonthGrid helper
>   - `89a5050` feat(calendar): add WeekdayHeader component
>   - `468baf4` refactor(calendar): rewrite DayGrid as 7x6 weekday-aligned grid
>   - `d57bd6d` feat(calendar): render sticky weekday header and load adjacent-month completions
>   - `743e00c` fix(a11y): include completion state in DaySquare aria-label
>   - `0dcbded` refactor(ui): extract goal-info width to CSS custom property
>   - `30daa68` fix(calendar): normalize completion date keys to YYYY-MM-DD (regression caught by E2E)
> - Incidental improvements bundled in the same PR:
>   - `a60184e` ci(android): allow workflow_dispatch triggers for feature-branch builds
>   - `867487e` ci(android): derive debug APK version from latest git tag (fixed pre-existing drift: debug APKs had reported the stale `1.0.2/3` defaults across the prior ~10 releases)
>   - `8fbda83` test: remove two pre-existing broken e2e tests unrelated to this feature
> - Verification: 69/69 Vitest passing; all meaningful Chromium Playwright tests passing; release APK sideloaded on a physical device and verified to report `versionName=1.0.13 versionCode=10013` with the weekday header and adjacent-month cells rendering.

**Goal:** Add a weekday header (Sun–Sat) above the calendar and align each goal's day grid to a proper 7-column weekly layout, so users can see which day of the week each date falls on.

**Problem:** The current calendar renders days sequentially: desktop shows a flat 31-column horizontal strip per goal, mobile wraps into an unaligned 7-column grid. In neither case do days line up under their correct weekday. Without weekday context, users cannot easily tell which day was "last Saturday" or "this Monday" when tracking habit streaks.

**Approach:** Replace both layouts with a single weekday-aligned `7 × 6` grid. Render a sticky shared weekday header once at the top of the goals list. Adjacent-month cells (leading days from M-1, trailing days from M+1) are shown in their correct columns for a seamless week view. Leading M-1 cells are interactive (subject to the existing 7-day lockout); trailing M+1 cells are shown but never interactive because they fall in the future.

**Tech Stack:** Svelte 5, TypeScript, svelte-i18n, Vitest, Playwright (no backend changes).

---

## Design Decisions

### Layout

- Single `grid-template-columns: repeat(7, 1fr)` layout for desktop and mobile. Drop the desktop 31-column strip and the mobile media-query breakpoint.
- Always render **42 cells** (6 rows × 7 columns). Months that don't need the last row render trailing next-month cells instead of leaving empty space. Fixed height eliminates layout shift when navigating months.
- Week starts on **Sunday**, hard-coded. Works for both current locales (`en`, `pt-BR`). No per-locale or user-configurable week start — YAGNI.

### Adjacent-Month Cells

- **Leading cells (from M-1):** Shown in a muted style, fully interactive. Completion state is loaded from the backend (see Data Loading). Subject to the existing 7-day lockout via full-date comparison.
- **Trailing cells (from M+1):** Shown in a muted style, **never interactive** (all future dates, already disabled by existing logic). Completion data for M+1 is **not** fetched — there's nothing to display because future days cannot have completions.
- **Edge-case trade-off:** Leading cells from M-1 that fall outside the last-7-days load window (e.g., when viewing a past month) display as empty/uncompleted even if they were actually completed. Accepted cost to avoid loading a full extra month of data for non-interactive cells.

### Data Loading

Two parallel fetches per month navigation, no backend changes:

1. **`getCalendar(M)`** — existing call, returns goals + current-month completions.
2. **`/completions?from=<M-1 last 7 days start>&to=<M-1 last day>`** — small range fetch via the existing `/completions?from=&to=` endpoint (already used by `getCurrentPeriodCompletions` at `frontend/src/lib/api.ts:388`). Returns completions for the leading adjacent cells.

Results from both are merged into a single flat completion list and passed to the goals list.

### Data Model Change

`App.svelte:135-140` currently keys completions by day number:

```ts
// Before
completionsByGoal: Record<string, Map<number, string>>  // day → completion_id
```

Change to key by full date string:

```ts
// After
completionsByGoal: Record<string, Map<string, string>>  // "YYYY-MM-DD" → completion_id
```

The reduce drops `parseInt(c.date.split('-')[2])` and uses `c.date` directly as the map key.

### Toggle Signature Change

```ts
// Before — DayGrid.svelte:7
onToggle: (day: number) => void

// After
onToggle: (dateString: string) => void
```

Each cell already carries its full date string, so DayGrid passes it directly. App.svelte's toggle handler already constructs full dates internally — just remove the day-number → date conversion step. The underlying `createCompletion(goalId, date)` API (`frontend/src/lib/api.ts:243`) is unchanged.

### 7-Day Lockout

`DayGrid.svelte:23-36` already operates on full Date objects. No change needed — it correctly handles leading M-1 cells that fall within the 7-day window (e.g., March 31 viewed from April on April 2).

---

## Components

### New: `frontend/src/lib/calendar.ts`

Pure helper module. ~30 lines. No Svelte imports, easy to unit-test.

```ts
export interface CalendarCell {
  dateString: string;       // "YYYY-MM-DD"
  dayNumber: number;        // 1-31
  isCurrentMonth: boolean;
}

/**
 * Build a 42-cell month grid for the given (year, month).
 * `month` is 1-indexed (1 = January).
 * Always returns exactly 42 cells: leading days from prev month,
 * all current month days, trailing days from next month.
 * First cell is always a Sunday.
 */
export function buildMonthGrid(year: number, month: number): CalendarCell[];
```

### New: `frontend/src/lib/components/WeekdayHeader.svelte`

~20 lines. Renders a single CSS Grid row with 7 columns matching `DayGrid`'s template.

- Cells contain localized short labels from `$_('weekday.sun')` … `$_('weekday.sat')`.
- `aria-hidden="true"` — decorative; day cells carry semantic date labels.
- `position: sticky; top: 0; z-index: 2;` with solid `var(--bg-primary)` background so it overlays goal rows on scroll.
- Thin bottom border / soft shadow for visual separation when sticky.

### Modified: `frontend/src/lib/components/DayGrid.svelte`

Rewrite the body; same approximate line count.

- **Props change:**
  - Remove `daysInMonth: number` → add `cells: CalendarCell[]` (precomputed in App.svelte, shared across all goal rows).
  - Remove `completedDays: Set<number>` → add `completedDates: Map<string, string>` (date → completion id).
  - Change `onToggle: (day: number)` → `onToggle: (dateString: string)`.
- **Render:** `{#each cells as cell}` → one `DaySquare` per cell, passing `outsideMonth={!cell.isCurrentMonth}` and `filled={completedDates.has(cell.dateString)}`.
- **CSS:** `grid-template-columns: repeat(7, 1fr)` at all widths. Drop `@media (max-width: 500px)` block. Grid auto-flows into 6 rows.

### Modified: `frontend/src/lib/components/DaySquare.svelte`

Small extension.

- Add `export let outsideMonth: boolean = false;`
- When `outsideMonth` is true, apply a muted style class (lower opacity / grayed text). Click handler still fires; interactivity is gated by the existing `disabled` prop, which is driven by the lockout logic on the full date.
- Update `aria-label` to include the full localized date (e.g., "Sunday, March 29, completed") instead of just `"Day {day}"`, so screen readers aren't confused by leading cells in April showing "Day 31".

### Modified: `frontend/src/App.svelte`

- Compute `cells = buildMonthGrid(year, month)` reactively alongside existing month-derivation code (around lines 112-117). Pass `cells` to every `DayGrid` instance.
- Replace the `completionsByGoal` reduce (lines 135-140) to key by `c.date` instead of day number.
- Trigger the secondary `/completions?from=&to=` fetch for M-1's last 7 days in parallel with `getCalendar(M)` and merge both lists into `completions` before the reduce runs.
- Render `<WeekdayHeader />` once at the top of the goals list, inside the same scroll container so sticky positioning works.

### i18n Additions

Extend `frontend/src/lib/i18n/en.json` and `pt-BR.json`:

```json
"weekday": {
  "sun": "S", "mon": "M", "tue": "T",
  "wed": "W", "thu": "T", "fri": "F", "sat": "S",
  "sun_long": "Sunday", "mon_long": "Monday", "tue_long": "Tuesday",
  "wed_long": "Wednesday", "thu_long": "Thursday", "fri_long": "Friday",
  "sat_long": "Saturday"
}
```

pt-BR short forms: `D S T Q Q S S` (Brazilian convention; ambiguous duplicates are expected). Long forms: `Domingo`, `Segunda-feira`, etc.

Short forms used in the `WeekdayHeader`; long forms used in the per-cell aria-labels on `DaySquare`.

---

## Testing

### Unit Tests (Vitest) — new `frontend/src/lib/calendar.test.ts`

- `buildMonthGrid` returns exactly 42 cells for any input.
- First cell is always a Sunday (JS `.getDay() === 0`).
- `isCurrentMonth` correctly flags the three segments (leading prev, current, trailing next).
- Leading-cell count equals `new Date(year, month-1, 1).getDay()`.
- Edge cases:
  - Month starting on Sunday (0 leading cells).
  - Month starting on Saturday (6 leading cells, trailing cells fill row 6).
  - Leap February (Feb 2024, 29 days).
  - 28-day February (Feb 2026).
  - Year boundary forward (December → January leading cells from Dec into the next year).
  - Year boundary backward (January's leading cells come from December of the previous year).

### E2E Tests (Playwright) — new `frontend/e2e/calendar-grid.spec.ts`

- Weekday header is visible and shows 7 localized labels.
- Header remains visible when scrolling the goals list (sticky positioning).
- Day cells align under correct weekday columns (spot-check a known date's grid column via computed styles or a data attribute).
- Toggling a leading-adjacent M-1 cell that falls within the 7-day window persists across reload.
- Clicking a leading-adjacent M-1 cell outside the 7-day window does nothing (still disabled).
- Trailing M+1 cells render but do not respond to clicks.

### Ripple Cost on Existing Tests

- Existing specs that query `aria-label="Day {N}"` (e.g., `frontend/e2e/bug-fixes.spec.ts`) need updating to match the new localized aria-label format.
- Existing month-navigation spec (`frontend/e2e/month-navigation.spec.ts`) should continue working without changes but needs to be re-run.

### Accessibility

- Header is `aria-hidden="true"` (decorative).
- Day cells carry a full localized aria-label including weekday, date, and completion state.
- Keyboard navigation: day buttons remain focusable. Arrow-key grid traversal is **out of scope** — potential follow-up.

---

## Performance

- Each goal renders 42 `DaySquare` components instead of 31 (~35% increase in DOM nodes). For 10 goals, that's 420 cells vs 310. Negligible; no virtualization needed.
- Two parallel HTTP calls on month navigation instead of one. The second call returns at most ~7 completions × goal count — trivial payload.

---

## Rollout

- Pure frontend change. No backend changes, no schema migration, no data migration.
- Ship without a feature flag — the old strip layout is replaced directly.
- Manual smoke test after deploy: verify the weekday header shows, adjacent cells render correctly, and toggling a recent past-month day (from the leading cells of the current month) persists.

---

## Out of Scope (Explicit Non-Goals)

- Arrow-key grid navigation between day cells.
- Touch-drag month navigation.
- Locale-driven week start (Monday-start for hypothetical future locales).
- Adjacent M+1 cell interactivity (all future, permanently disabled).
- Showing completion state for leading M-1 cells outside the loaded 7-day window.
- Horizontal scroll / zoom behavior on very narrow viewports — defer until we see real issues.
