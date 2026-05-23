# Calendar Viewport Fit & Dynamic Row Height — Design

**Status:** Design approved 2026-04-08. Implementation pending.

**Goal:** Three follow-up refinements to the weekday-indicator calendar shipped in v1.0.13:

1. Remove the visual separator under the weekday header.
2. Render months with 5 grid rows when they fit; fall back to 6 rows only when needed.
3. Lock the app to the viewport so the root document no longer scrolls — only the goals list scrolls internally when it overflows.

**Tech stack:** Svelte 5, TypeScript, svelte-i18n, Vitest, Playwright. No backend changes.

---

## 1. Remove the weekday-header separator

`frontend/src/lib/components/WeekdayHeader.svelte` currently draws a `border-bottom: 1px solid var(--border)` under `.weekday-header`. Drop that declaration.

The header's solid `var(--bg-primary)` background and sticky positioning already give it enough visual distinction from the first goal row. `padding: 0.25rem 0` provides breathing room. No change to layout math, component props, or tests — just remove the border rule.

---

## 2. Dynamic 5/6-row grid

### Helper change

`frontend/src/lib/calendar.ts` — `buildMonthGrid` becomes variable-length instead of always returning 42 cells:

```ts
export function buildMonthGrid(year: number, month: number): CalendarCell[] {
  const firstOfMonth = new Date(year, month - 1, 1);
  const leadingCount = firstOfMonth.getDay();
  const daysInMonth = new Date(year, month, 0).getDate();
  const totalCells = leadingCount + daysInMonth <= 35 ? 35 : 42;

  const gridStart = new Date(year, month - 1, 1 - leadingCount);
  const cells: CalendarCell[] = [];
  const cursor = new Date(gridStart);
  for (let i = 0; i < totalCells; i++) {
    // ...existing body unchanged
  }
  return cells;
}
```

A month needs the 6th row only when `leadingCount + daysInMonth > 35`. Examples:

| Month      | Leading + Days | Cells |
| ---------- | -------------- | ----- |
| Feb 2026   | 0 + 28 = 28    | 35    |
| Feb 2024   | 4 + 29 = 33    | 35    |
| Apr 2026   | 3 + 30 = 33    | 35    |
| Sep 2024   | 0 + 30 = 30    | 35    |
| May 2026   | 5 + 31 = 36    | 42    |
| Aug 2026   | 6 + 31 = 37    | 42    |

### Grid/render changes

`frontend/src/lib/components/DayGrid.svelte` **needs no changes**. `grid-template-columns: repeat(7, 1fr)` with square `aspect-ratio: 1` cells auto-flows into 5 or 6 rows depending on cell count. Each cell stays the same size; the grid is just shorter for 35-cell months.

### Trade-off

Month navigation will cause a small vertical layout shift when moving between a 5-row and a 6-row month (roughly one cell's height, ~48px). This is intentional — the alternative (padding short months with empty space) is exactly what we're removing. Since the app will be locked to `100dvh` (see Section 3), the shift is absorbed inside the scroll container; nothing outside the goals list jumps.

### Unit-test updates — `frontend/src/lib/calendar.test.ts`

- Replace "returns exactly 42 cells for any input" with a test covering **both** shapes:
  - 35-cell cases: Feb 2026, Feb 2024 (leap), Apr 2026, Sep 2024 Sun-start, Jun 2025.
  - 42-cell cases: Aug 2026 Sat-start, May 2026.
  - Boundary: a month where `leading + days === 35` gets 35 cells; `=== 36` gets 42 cells.
- Update "trailing cells fill row 6" (currently Aug 2026) — already a 6-row month, assertions stay valid.
- "First cell is always a Sunday", "contiguous dates", "leading-cell count", "isCurrentMonth flags three segments", year-boundary tests — all stay valid since they don't depend on total length.

---

## 3. Viewport fit (no root-document scroll)

### Root cause of current overflow

`.app-container` uses `min-height: 100vh` with `grid-template-rows: auto 1fr auto`. The `1fr` row can grow beyond the viewport because `min-height` lets the container expand. With the weekday header row added, total content on shorter mobile viewports exceeds `100vh`, so the document body scrolls.

### Layout lock

Change `.app-container`:

```css
.app-container {
  display: grid;
  grid-template-rows: auto 1fr auto;
  height: 100dvh;          /* was min-height: 100vh */
  overflow: hidden;        /* prevent root scroll */
  padding-top: env(safe-area-inset-top);
  padding-bottom: env(safe-area-inset-bottom);
  box-sizing: border-box;
}
```

**`100dvh`** (dynamic viewport height) avoids the mobile-Safari/Chrome pitfall where `100vh` includes space under the browser chrome and pushes the footer off-screen.

Change `main`:

```css
main {
  padding: var(--space-md) 0;   /* was var(--space-lg) 0 — tightens ~16px */
  width: 100%;
  box-sizing: border-box;
  min-height: 0;                /* critical: lets the 1fr row actually shrink */
  display: flex;
  flex-direction: column;
  overflow: hidden;
}
```

`min-height: 0` is the grid-flex escape hatch. Without it, the `1fr` track refuses to shrink below its content's intrinsic size, and the app grows past the viewport — the actual bug we're fixing.

### Goals list as the scroll container

```css
.goals {
  width: 100%;
  flex: 1;
  min-height: 0;
  overflow-y: auto;
  overscroll-behavior: contain;
}
```

The weekday header row stays **inside `.goals`** at the top with its existing `position: sticky; top: 0`. Because `.goals` is now the scroll container, sticky anchors the weekday row to the top of the scrolling region — exactly the behavior we want when scrolling through many goals.

### Height tightening (common case)

The 5/6-row change from Section 2 already saves ~48px for most months — the biggest single win. A few more small tweaks to help the typical case fit without any internal scroll:

- `main` padding: `var(--space-lg) 0` → `var(--space-md) 0` (~16px).
- `.goal-row` padding: `0.375rem 0` → `0.25rem 0` (~4px × N goals).
- `.weekday-header` padding: `0.25rem 0` → `0.125rem 0` (~4px).

These are small, reversible adjustments. The dominant factor is still the 5-row / 6-row switch.

### What this does NOT change

- No changes to Header, Footer, MonthNav, GoalEditor, ProfilePage, or AuthPage.
- No virtualization. With internal scroll, rendering all goal rows is fine.
- No change to the `/completions` 7-day lockout fetch. Leading adjacent-month cells are still loaded the same way.
- No change to the weekday-start day, i18n keys, `DaySquare` props, or `GoalRow` props.
- No change to the current sticky positioning mechanism — only its containing block changes.

---

## Testing

### Unit tests (Vitest)

Covered in Section 2. Rewrites `calendar.test.ts` to assert variable-length output.

### E2E tests (Playwright) — `frontend/e2e/calendar-grid.spec.ts`

New assertions:

- Navigate to a known 5-row month (e.g. April 2026 or February 2026) and assert the grid has exactly 35 `[data-date]` cells per goal.
- Navigate to a known 6-row month (e.g. August 2026 or May 2026) and assert 42 cells per goal.
- On a standard desktop viewport with a typical goal count, assert `document.documentElement.scrollHeight === document.documentElement.clientHeight` (no root-document scroll).
- With many seeded goals (e.g. 15), assert the goals list has an internal scrollbar and the weekday header stays visible at the top of the scroll region after scrolling down.

Existing assertions that depend on the weekday-header bottom border need updating — the border is gone.

### Ripple cost

- Any test that asserts "42 cells" directly needs updating. Spot-check with `grep -rn "42" frontend/e2e frontend/src` during implementation.
- Existing month-navigation and bug-fix specs should continue working without changes but need a re-run.

---

## Rollout

- Pure frontend change. No backend touches, no schema/data migration, no feature flag.
- Single PR, single merge to `main`.
- Manual smoke test on a real device after the debug APK build completes in CI (no local Android SDK per project conventions).

## Out of scope

- Changing the week start day.
- Virtualizing the goals list.
- Changing how `/completions` fetches leading-cell data.
- Restructuring Header/Footer/MonthNav to reclaim more vertical space.
- Animating the 5↔6 row height change on month navigation.
- Keyboard arrow-key navigation across the grid (still a potential follow-up from the weekday-header design doc).
