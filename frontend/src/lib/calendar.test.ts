import { describe, it, expect } from 'vitest';
import { buildMonthGrid } from './calendar';

describe('buildMonthGrid', () => {
  it('returns 35 cells when the month fits in 5 rows', () => {
    // Each month here has leadingCount + daysInMonth <= 35.
    //   Feb 2026: 0 + 28 = 28
    //   Feb 2024 (leap): 4 + 29 = 33
    //   Apr 2026: 3 + 30 = 33
    //   Sep 2024 (Sun-start): 0 + 30 = 30
    //   Jun 2025: 0 + 30 = 30
    const months: Array<[number, number]> = [
      [2026, 2],
      [2024, 2],
      [2026, 4],
      [2024, 9],
      [2025, 6],
    ];
    for (const [year, month] of months) {
      const cells = buildMonthGrid(year, month);
      expect(cells.length).toBe(35);
    }
  });

  it('returns 42 cells when the month needs 6 rows', () => {
    // Each month here has leadingCount + daysInMonth > 35.
    //   Aug 2026 (Sat-start): 6 + 31 = 37
    //   May 2026: 5 + 31 = 36
    const months: Array<[number, number]> = [
      [2026, 8],
      [2026, 5],
    ];
    for (const [year, month] of months) {
      const cells = buildMonthGrid(year, month);
      expect(cells.length).toBe(42);
    }
  });

  it('boundary: leading + days === 35 yields 35 cells, === 36 yields 42', () => {
    // Oct 2020: leading=4 (Thu-start), days=31 → sum 35, fits in 5 rows.
    const fiveRow = buildMonthGrid(2020, 10);
    expect(fiveRow.length).toBe(35);
    // Jan 2021: leading=5 (Fri-start), days=31 → sum 36, needs 6 rows.
    const sixRow = buildMonthGrid(2021, 1);
    expect(sixRow.length).toBe(42);
  });

  it('first cell is always a Sunday (JS getDay() === 0)', () => {
    const months: Array<[number, number]> = [
      [2024, 1],
      [2024, 2],
      [2024, 3],
      [2024, 4],
      [2024, 5],
      [2025, 2],
      [2026, 2],
      [2026, 4],
      [2026, 11],
      [2027, 1],
    ];
    for (const [year, month] of months) {
      const cells = buildMonthGrid(year, month);
      const firstDate = new Date(cells[0].dateString + 'T00:00:00');
      expect(firstDate.getDay()).toBe(0);
    }
  });

  it('isCurrentMonth correctly flags three segments', () => {
    // April 2026: April 1 is a Wednesday, so 3 leading cells from March.
    const cells = buildMonthGrid(2026, 4);
    const leading = cells.filter((_, i) => i < 3);
    const current = cells.filter((c) => c.isCurrentMonth);
    const trailing = cells.filter((_, i) => i >= 3 + 30);

    expect(leading.every((c) => !c.isCurrentMonth)).toBe(true);
    expect(current.length).toBe(30);
    expect(trailing.every((c) => !c.isCurrentMonth)).toBe(true);
  });

  it('leading-cell count equals new Date(year, month-1, 1).getDay()', () => {
    const months: Array<[number, number]> = [
      [2024, 1],
      [2024, 2],
      [2024, 3],
      [2024, 9], // Sept 1 2024 is a Sunday
      [2025, 2],
      [2026, 2],
      [2026, 4],
      [2026, 8], // Aug 1 2026 is a Saturday
      [2027, 1],
    ];
    for (const [year, month] of months) {
      const cells = buildMonthGrid(year, month);
      const expectedLeading = new Date(year, month - 1, 1).getDay();
      const actualLeading = cells.findIndex((c) => c.isCurrentMonth);
      expect(actualLeading).toBe(expectedLeading);
    }
  });

  it('month starting on Sunday has 0 leading cells', () => {
    // September 2024: Sept 1 is a Sunday
    const cells = buildMonthGrid(2024, 9);
    expect(cells[0].isCurrentMonth).toBe(true);
    expect(cells[0].dayNumber).toBe(1);
    expect(cells[0].dateString).toBe('2024-09-01');
  });

  it('month starting on Saturday has 6 leading cells and trailing cells fill row 6', () => {
    // August 2026: Aug 1 is a Saturday
    const cells = buildMonthGrid(2026, 8);
    // Leading: Sun..Fri (6 cells from July)
    for (let i = 0; i < 6; i++) {
      expect(cells[i].isCurrentMonth).toBe(false);
    }
    expect(cells[6].isCurrentMonth).toBe(true);
    expect(cells[6].dayNumber).toBe(1);
    // August has 31 days; 6 + 31 = 37 current/leading cells
    // Last 5 cells should be trailing (September)
    for (let i = 37; i < 42; i++) {
      expect(cells[i].isCurrentMonth).toBe(false);
    }
    expect(cells.length).toBe(42);
  });

  it('handles leap February (Feb 2024, 29 days)', () => {
    const cells = buildMonthGrid(2024, 2);
    const currentCells = cells.filter((c) => c.isCurrentMonth);
    expect(currentCells.length).toBe(29);
    expect(currentCells[28].dateString).toBe('2024-02-29');
    // Feb 1 2024 is a Thursday
    const firstCurrentIndex = cells.findIndex((c) => c.isCurrentMonth);
    expect(firstCurrentIndex).toBe(4);
  });

  it('handles 28-day February (Feb 2026)', () => {
    const cells = buildMonthGrid(2026, 2);
    const currentCells = cells.filter((c) => c.isCurrentMonth);
    expect(currentCells.length).toBe(28);
    expect(currentCells[0].dateString).toBe('2026-02-01');
    expect(currentCells[27].dateString).toBe('2026-02-28');
  });

  it('handles year boundary forward (January leading cells come from December of previous year)', () => {
    // January 2026: Jan 1 is a Thursday, so 4 leading cells from Dec 2025
    const cells = buildMonthGrid(2026, 1);
    expect(cells[0].dateString).toBe('2025-12-28');
    expect(cells[1].dateString).toBe('2025-12-29');
    expect(cells[2].dateString).toBe('2025-12-30');
    expect(cells[3].dateString).toBe('2025-12-31');
    expect(cells[4].dateString).toBe('2026-01-01');
    expect(cells[4].isCurrentMonth).toBe(true);
  });

  it('handles year boundary backward (December trailing cells go into January of next year)', () => {
    // December 2025: Dec 1 is a Monday, so 1 leading cell (Nov 30)
    const cells = buildMonthGrid(2025, 12);
    expect(cells[0].dateString).toBe('2025-11-30');
    expect(cells[1].dateString).toBe('2025-12-01');
    expect(cells[1].isCurrentMonth).toBe(true);
    // Last cell should be in January 2026 (Dec 2025 is a 5-row month).
    const lastCell = cells[cells.length - 1];
    expect(lastCell.isCurrentMonth).toBe(false);
    expect(lastCell.dateString.startsWith('2026-01')).toBe(true);
  });

  it('every cell has a valid dateString, dayNumber and isCurrentMonth flag', () => {
    const cells = buildMonthGrid(2026, 4);
    for (const cell of cells) {
      expect(cell.dateString).toMatch(/^\d{4}-\d{2}-\d{2}$/);
      expect(cell.dayNumber).toBeGreaterThanOrEqual(1);
      expect(cell.dayNumber).toBeLessThanOrEqual(31);
      expect(typeof cell.isCurrentMonth).toBe('boolean');
    }
  });

  it('cells are contiguous (each date is exactly one day after the previous)', () => {
    const cells = buildMonthGrid(2026, 4);
    for (let i = 1; i < cells.length; i++) {
      const prev = new Date(cells[i - 1].dateString + 'T00:00:00');
      const curr = new Date(cells[i].dateString + 'T00:00:00');
      const diffMs = curr.getTime() - prev.getTime();
      expect(diffMs).toBe(24 * 60 * 60 * 1000);
    }
  });
});
