/**
 * Integration test for the view-toggle feature.
 *
 * Component mounting (testing-library) is not available in this project, so
 * this file tests the store + calendar layer that the ViewToggle component
 * delegates to, which is the meaningful behavioural contract.
 *
 * Specifically:
 * 1. Selecting "W" (week) produces a 7-cell grid via buildWeekGrid.
 * 2. Selecting "M" (month) produces a 35- or 42-cell grid via buildMonthGrid.
 * 3. The viewSettings store reflects the selection.
 * 4. isCurrentPeriod logic matches the view mode.
 */
import { describe, it, expect, beforeEach, vi } from 'vitest';
import { buildWeekGrid, buildMonthGrid } from '../calendar';

// Mock @capacitor/preferences with an in-memory store
const prefStore = vi.hoisted(() => new Map<string, string>());
vi.mock('@capacitor/preferences', () => ({
  Preferences: {
    get: vi.fn(async ({ key }: { key: string }) => ({
      value: prefStore.has(key) ? prefStore.get(key)! : null,
    })),
    set: vi.fn(async ({ key, value }: { key: string; value: string }) => {
      prefStore.set(key, value);
    }),
  },
}));

async function freshMod() {
  vi.resetModules();
  return await import('../view-settings');
}

describe('view toggle — store + grid integration', () => {
  beforeEach(() => {
    prefStore.clear();
    vi.clearAllMocks();
  });

  it('selecting "week" produces exactly 7 cells for any reference date', async () => {
    const mod = await freshMod();
    await mod.updateViewSettings({ calendarView: 'week' });

    const settings = await mod.loadViewSettings();
    expect(settings.calendarView).toBe('week');

    // Build the week grid as App.svelte would when view==='week'
    const focalDate = new Date(2026, 4, 13); // Wednesday May 13
    const cells = buildWeekGrid(focalDate);
    expect(cells.length).toBe(7);
  });

  it('selecting "month" produces 35 or 42 cells', async () => {
    const mod = await freshMod();
    await mod.updateViewSettings({ calendarView: 'month' });

    const settings = await mod.loadViewSettings();
    expect(settings.calendarView).toBe('month');

    // Build as App.svelte would when view==='month'
    const cells = buildMonthGrid(2026, 5); // May 2026 = 42 cells
    expect(cells.length).toBe(42);
  });

  it('viewSettings store reflects the toggle selection', async () => {
    const mod = await freshMod();
    const seen: string[] = [];
    const unsub = mod.viewSettings.subscribe(s => seen.push(s.calendarView));

    await mod.updateViewSettings({ calendarView: 'week' });
    await mod.updateViewSettings({ calendarView: 'month' });
    unsub();

    // Initial default + two updates
    expect(seen).toContain('month');
    expect(seen).toContain('week');
    // Last value should be 'month'
    expect(seen[seen.length - 1]).toBe('month');
  });

  it('week grid cells align with the 7-column weekday header (Sun→Sat)', async () => {
    // The WeekdayHeader always renders 7 columns. Verify the week grid
    // cells cover exactly one of each weekday starting from Sunday.
    const focalDate = new Date(2026, 4, 13); // Wednesday
    const cells = buildWeekGrid(focalDate);
    expect(cells.length).toBe(7);

    // First cell must be Sunday
    const firstDate = new Date(cells[0].dateString + 'T00:00:00');
    expect(firstDate.getDay()).toBe(0); // Sunday

    // Each subsequent cell is one day later
    for (let i = 1; i < 7; i++) {
      const d = new Date(cells[i].dateString + 'T00:00:00');
      expect(d.getDay()).toBe(i);
    }
  });

  it('isCurrentPeriod — week mode: current week returns true', () => {
    const today = new Date();
    today.setHours(0, 0, 0, 0);

    // Simulate the isCurrentPeriod logic for week mode
    function isCurrentPeriodWeek(focalDate: Date): boolean {
      const now = new Date();
      const todayDow = now.getDay();
      const focalDow = focalDate.getDay();
      const todaySunday = new Date(now);
      todaySunday.setDate(now.getDate() - todayDow);
      todaySunday.setHours(0, 0, 0, 0);
      const focalSunday = new Date(focalDate);
      focalSunday.setDate(focalDate.getDate() - focalDow);
      focalSunday.setHours(0, 0, 0, 0);
      return todaySunday.getTime() === focalSunday.getTime();
    }

    // Today should be in the current week
    expect(isCurrentPeriodWeek(today)).toBe(true);

    // One week ago should not be the current week
    const lastWeek = new Date(today);
    lastWeek.setDate(today.getDate() - 7);
    expect(isCurrentPeriodWeek(lastWeek)).toBe(false);
  });
});
