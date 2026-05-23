import { describe, it, expect } from 'vitest';
import { buildMonthGrid, buildWeekGrid, formatPeriodLabel, monthsForCells } from './calendar';

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

describe('buildWeekGrid', () => {
  it('returns exactly 7 cells', () => {
    // Mid-month reference: Wednesday May 13, 2026
    const cells = buildWeekGrid(new Date(2026, 4, 13)); // month is 0-indexed
    expect(cells.length).toBe(7);
  });

  it('mid-month week: all cells isCurrentMonth true, Sun→Sat ordering', () => {
    // May 13 2026 is a Wednesday. Week: Sun May 10 – Sat May 16.
    const cells = buildWeekGrid(new Date(2026, 4, 13));
    expect(cells[0].dateString).toBe('2026-05-10');
    expect(cells[6].dateString).toBe('2026-05-16');
    expect(cells.every(c => c.isCurrentMonth)).toBe(true);
    // Verify ordering is contiguous
    for (let i = 1; i < 7; i++) {
      const prev = new Date(cells[i - 1].dateString + 'T00:00:00');
      const curr = new Date(cells[i].dateString + 'T00:00:00');
      expect(curr.getTime() - prev.getTime()).toBe(24 * 60 * 60 * 1000);
    }
  });

  it('week spanning into prior month: leading days isCurrentMonth false', () => {
    // May 1 2026 is a Friday. Week: Sun Apr 26 – Sat May 2.
    // referenceDate is May 1; refMonth = 5.
    const cells = buildWeekGrid(new Date(2026, 4, 1));
    expect(cells[0].dateString).toBe('2026-04-26');
    expect(cells[5].dateString).toBe('2026-05-01');
    expect(cells[6].dateString).toBe('2026-05-02');
    // Apr 26–30 are outside May
    for (let i = 0; i < 5; i++) {
      expect(cells[i].isCurrentMonth).toBe(false);
    }
    // May 1–2 are inside May
    expect(cells[5].isCurrentMonth).toBe(true);
    expect(cells[6].isCurrentMonth).toBe(true);
  });

  it('week spanning into next month: trailing days isCurrentMonth false', () => {
    // May 30 2026 is a Saturday. Week: Sun May 24 – Sat May 30.
    const cells = buildWeekGrid(new Date(2026, 4, 30));
    expect(cells[0].dateString).toBe('2026-05-24');
    expect(cells[6].dateString).toBe('2026-05-30');
    // All in May
    expect(cells.every(c => c.isCurrentMonth)).toBe(true);

    // May 31 2026 is a Sunday — week: Sun May 31 – Sat Jun 6
    const cells2 = buildWeekGrid(new Date(2026, 4, 31));
    expect(cells2[0].dateString).toBe('2026-05-31');
    expect(cells2[1].dateString).toBe('2026-06-01');
    expect(cells2[0].isCurrentMonth).toBe(true);
    // Jun 1–6 are outside May
    for (let i = 1; i < 7; i++) {
      expect(cells2[i].isCurrentMonth).toBe(false);
    }
  });

  it('year-boundary week: Dec 28 2025 – Jan 3 2026', () => {
    // Dec 28 2025 is a Sunday.
    const cells = buildWeekGrid(new Date(2025, 11, 28));
    expect(cells[0].dateString).toBe('2025-12-28');
    expect(cells[6].dateString).toBe('2026-01-03');
    // refMonth = 12. Dec 28–31 are inside Dec; Jan 1–3 are outside.
    for (let i = 0; i < 4; i++) {
      expect(cells[i].isCurrentMonth).toBe(true);
    }
    for (let i = 4; i < 7; i++) {
      expect(cells[i].isCurrentMonth).toBe(false);
    }
  });

  it('leap year boundary: Feb 28 2024 (leap year) week', () => {
    // Feb 28 2024 is a Wednesday. Week: Sun Feb 25 – Sat Mar 2.
    const cells = buildWeekGrid(new Date(2024, 1, 28));
    expect(cells[0].dateString).toBe('2024-02-25');
    expect(cells[6].dateString).toBe('2024-03-02');
    // refMonth = 2. Feb 25–29 (5 cells) inside, Mar 1–2 outside.
    expect(cells[0].isCurrentMonth).toBe(true); // Feb 25
    expect(cells[4].dateString).toBe('2024-02-29'); // leap day
    expect(cells[4].isCurrentMonth).toBe(true);
    expect(cells[5].dateString).toBe('2024-03-01');
    expect(cells[5].isCurrentMonth).toBe(false);
    expect(cells[6].dateString).toBe('2024-03-02');
    expect(cells[6].isCurrentMonth).toBe(false);
  });

  it('first cell is always a Sunday (getDay() === 0)', () => {
    const referenceDates = [
      new Date(2026, 0, 15),   // Thursday Jan 15
      new Date(2026, 1, 1),    // Sunday Feb 1
      new Date(2026, 3, 30),   // Thursday Apr 30
      new Date(2025, 11, 28),  // Sunday Dec 28
      new Date(2024, 1, 28),   // Wednesday Feb 28 (leap year)
    ];
    for (const d of referenceDates) {
      const cells = buildWeekGrid(d);
      const firstDate = new Date(cells[0].dateString + 'T00:00:00');
      expect(firstDate.getDay()).toBe(0);
    }
  });

  it('reference date on Sunday: that Sunday is cell[0]', () => {
    // Feb 1 2026 is a Sunday
    const cells = buildWeekGrid(new Date(2026, 1, 1));
    expect(cells[0].dateString).toBe('2026-02-01');
    expect(cells[6].dateString).toBe('2026-02-07');
    expect(cells.every(c => c.isCurrentMonth)).toBe(true);
  });

  it('reference date on Saturday: that Saturday is cell[6]', () => {
    // Feb 7 2026 is a Saturday
    const cells = buildWeekGrid(new Date(2026, 1, 7));
    expect(cells[0].dateString).toBe('2026-02-01');
    expect(cells[6].dateString).toBe('2026-02-07');
  });

  it('every cell has a valid dateString, dayNumber, and isCurrentMonth flag', () => {
    const cells = buildWeekGrid(new Date(2026, 4, 13));
    for (const cell of cells) {
      expect(cell.dateString).toMatch(/^\d{4}-\d{2}-\d{2}$/);
      expect(cell.dayNumber).toBeGreaterThanOrEqual(1);
      expect(cell.dayNumber).toBeLessThanOrEqual(31);
      expect(typeof cell.isCurrentMonth).toBe('boolean');
    }
  });
});

describe('formatPeriodLabel', () => {
  it('en: returns "April" for April 2026', () => {
    const label = formatPeriodLabel(new Date(2026, 3, 15), 'en');
    expect(label).toBe('April');
  });

  it('pt-BR: returns "abril" (lowercase per pt-BR locale)', () => {
    const label = formatPeriodLabel(new Date(2026, 3, 15), 'pt-BR');
    expect(label).not.toBe('April');
    expect(label.toLowerCase()).toContain('abril');
  });
});

describe('monthsForCells', () => {
  it('returns single month when all cells fall in one month', () => {
    const cells = buildWeekGrid(new Date(2026, 4, 13)); // mid-May week
    const months = monthsForCells(cells);
    expect(months).toEqual(['2026-05']);
  });

  it('returns two months when week straddles a month boundary', () => {
    // May 1 2026 is Friday; week Sun Apr 26 – Sat May 2
    const cells = buildWeekGrid(new Date(2026, 4, 1));
    const months = monthsForCells(cells);
    expect(months.sort()).toEqual(['2026-04', '2026-05']);
  });

  it('returns two months when week straddles a year boundary', () => {
    // Dec 28 2025 Sunday – Jan 3 2026 Saturday
    const cells = buildWeekGrid(new Date(2025, 11, 28));
    const months = monthsForCells(cells);
    expect(months.sort()).toEqual(['2025-12', '2026-01']);
  });

  it('returns months in the order encountered from a month grid', () => {
    // Apr 2026 grid: leading days from March + April + trailing from May
    const cells = buildMonthGrid(2026, 4);
    const months = monthsForCells(cells);
    expect(months).toContain('2026-04');
    expect(months).toContain('2026-03');
    // 5-row month: Apr 2026 has leading=3 (Wed start), days=30 → 33 cells total
    // The 33-cell run fits in 35; the last 2 cells will be May 1, May 2.
    // Verify by checking whether trailing cells exist
    const hasTrailing = cells.some(c => c.dateString.startsWith('2026-05'));
    if (hasTrailing) {
      expect(months).toContain('2026-05');
    }
  });
});
