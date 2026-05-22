export interface CalendarCell {
  dateString: string; // "YYYY-MM-DD"
  dayNumber: number; // 1-31
  isCurrentMonth: boolean;
}

/**
 * Format a Date as "YYYY-MM-DD" without timezone conversion.
 */
function toDateString(d: Date): string {
  const y = d.getFullYear();
  const m = d.getMonth() + 1;
  const day = d.getDate();
  return `${y.toString().padStart(4, '0')}-${m.toString().padStart(2, '0')}-${day.toString().padStart(2, '0')}`;
}

/**
 * Derive the unique YYYY-MM month strings covered by `cells`.
 * Used to determine which months the calendar API needs to fetch
 * when the visible period straddles a month boundary.
 */
export function monthsForCells(cells: CalendarCell[]): string[] {
  const seen = new Set<string>();
  for (const c of cells) {
    seen.add(c.dateString.slice(0, 7));
  }
  return [...seen];
}

/**
 * Format the header label for the currently visible period.
 * Pure helper for unit testing. `locale` should be the app locale string
 * (e.g. "en", "pt-BR") so the label matches the rest of the UI.
 */
export function formatPeriodLabel(
  focalDate: Date,
  view: 'month' | 'week',
  locale: string,
): string {
  if (view === 'month') {
    return new Intl.DateTimeFormat(locale, { month: 'long' }).format(focalDate);
  }
  // Week mode: "Apr 27 – May 3" (or locale equivalent)
  const dow = focalDate.getDay();
  const sunday = new Date(focalDate);
  sunday.setDate(focalDate.getDate() - dow);
  sunday.setHours(0, 0, 0, 0);
  const saturday = new Date(sunday);
  saturday.setDate(sunday.getDate() + 6);

  const yearCross = sunday.getFullYear() !== saturday.getFullYear();
  const fmt = (d: Date, includeYear: boolean) =>
    new Intl.DateTimeFormat(locale, {
      month: 'short',
      day: 'numeric',
      ...(includeYear ? { year: 'numeric' } : {}),
    }).format(d);

  if (yearCross) {
    return `${fmt(sunday, true)} – ${fmt(saturday, true)}`;
  }
  return `${fmt(sunday, false)} – ${fmt(saturday, false)}`;
}

/**
 * Build a 7-cell week grid for the week containing `referenceDate`.
 * The week always starts on Sunday (matching the weekday header).
 * `isCurrentMonth` is true when the cell falls in the same calendar
 * month as `referenceDate`; false for cells that spill into an adjacent month.
 */
export function buildWeekGrid(referenceDate: Date): CalendarCell[] {
  const refYear = referenceDate.getFullYear();
  const refMonth = referenceDate.getMonth() + 1; // 1-indexed

  // Find the Sunday that begins this week
  const dayOfWeek = referenceDate.getDay(); // 0 (Sun) – 6 (Sat)
  const sunday = new Date(referenceDate);
  sunday.setDate(referenceDate.getDate() - dayOfWeek);
  sunday.setHours(0, 0, 0, 0);

  const cells: CalendarCell[] = [];
  const cursor = new Date(sunday);
  for (let i = 0; i < 7; i++) {
    const y = cursor.getFullYear();
    const m = cursor.getMonth() + 1;
    const d = cursor.getDate();
    cells.push({
      dateString: toDateString(cursor),
      dayNumber: d,
      isCurrentMonth: y === refYear && m === refMonth,
    });
    cursor.setDate(cursor.getDate() + 1);
  }

  return cells;
}

/**
 * Build a weekday-aligned month grid for the given (year, month).
 * `month` is 1-indexed (1 = January).
 * Returns either 35 cells (5 rows) or 42 cells (6 rows) depending on
 * whether the month fits in 5 rows: leading days from the previous
 * month, all current-month days, and trailing days from the next month.
 * A 6th row is only included when `leadingCount + daysInMonth > 35`.
 * The first cell is always a Sunday so the grid aligns to a standard
 * 7-column (Sun-Sat) calendar layout.
 */
export function buildMonthGrid(year: number, month: number): CalendarCell[] {
  const firstOfMonth = new Date(year, month - 1, 1);
  const leadingCount = firstOfMonth.getDay(); // 0 (Sun) - 6 (Sat)
  const daysInMonth = new Date(year, month, 0).getDate();
  const totalCells = leadingCount + daysInMonth <= 35 ? 35 : 42;

  // Start the grid `leadingCount` days before the first of the month.
  const gridStart = new Date(year, month - 1, 1 - leadingCount);

  const cells: CalendarCell[] = [];
  const cursor = new Date(gridStart);
  for (let i = 0; i < totalCells; i++) {
    const y = cursor.getFullYear();
    const m = cursor.getMonth() + 1;
    const d = cursor.getDate();
    const dateString = `${y.toString().padStart(4, '0')}-${m
      .toString()
      .padStart(2, '0')}-${d.toString().padStart(2, '0')}`;
    cells.push({
      dateString,
      dayNumber: d,
      isCurrentMonth: m === month && y === year,
    });
    cursor.setDate(cursor.getDate() + 1);
  }

  return cells;
}
