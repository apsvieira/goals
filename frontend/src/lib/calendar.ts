export interface CalendarCell {
  dateString: string; // "YYYY-MM-DD"
  dayNumber: number; // 1-31
  isCurrentMonth: boolean;
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
