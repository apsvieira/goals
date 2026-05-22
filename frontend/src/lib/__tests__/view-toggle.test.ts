import { describe, it, expect, beforeAll, beforeEach, vi } from 'vitest';
import { render, fireEvent } from '@testing-library/svelte';
import '@testing-library/jest-dom/vitest';

// Mock @capacitor/preferences with an in-memory store BEFORE importing the component
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

import ViewToggle from '../components/ViewToggle.svelte';
import { viewSettings } from '../view-settings';
import { setupI18n } from '../i18n';

describe('ViewToggle component', () => {
  beforeAll(() => {
    setupI18n();
  });

  beforeEach(async () => {
    prefStore.clear();
    vi.clearAllMocks();
    // Reset the store to month (the default) before each test
    viewSettings.set({ calendarView: 'month' });
    // Let i18n settle
    await new Promise(resolve => setTimeout(resolve, 20));
  });

  it('renders both M and W segments', () => {
    const { getByLabelText } = render(ViewToggle);
    // Buttons should be findable by aria-label (full word)
    expect(getByLabelText(/Month/i)).toBeInTheDocument();
    expect(getByLabelText(/Week/i)).toBeInTheDocument();
  });

  it('marks the current selection as active', () => {
    const { getByLabelText } = render(ViewToggle);
    const monthBtn = getByLabelText(/Month/i);
    const weekBtn = getByLabelText(/Week/i);

    // Default is month
    expect(monthBtn.className).toContain('active');
    expect(weekBtn.className).not.toContain('active');
  });

  it('clicking W updates the store to week and applies the active class to W', async () => {
    const { getByLabelText } = render(ViewToggle);
    const weekBtn = getByLabelText(/Week/i);

    await fireEvent.click(weekBtn);

    // Allow the store update to settle (updateViewSettings is async).
    await new Promise(resolve => setTimeout(resolve, 20));

    // Store should now hold 'week'
    let current = 'month';
    const unsub = viewSettings.subscribe(s => { current = s.calendarView; });
    unsub();
    expect(current).toBe('week');
  });

  it('clicking W then M toggles back', async () => {
    const { getByLabelText } = render(ViewToggle);
    const monthBtn = getByLabelText(/Month/i);
    const weekBtn = getByLabelText(/Week/i);

    await fireEvent.click(weekBtn);
    await new Promise(resolve => setTimeout(resolve, 0));
    await fireEvent.click(monthBtn);
    await new Promise(resolve => setTimeout(resolve, 0));

    let current = 'week';
    const unsub = viewSettings.subscribe(s => { current = s.calendarView; });
    unsub();
    expect(current).toBe('month');
  });

  it('renders aria-pressed reflecting selection', () => {
    const { getByLabelText } = render(ViewToggle);
    const monthBtn = getByLabelText(/Month/i);
    const weekBtn = getByLabelText(/Week/i);

    expect(monthBtn.getAttribute('aria-pressed')).toBe('true');
    expect(weekBtn.getAttribute('aria-pressed')).toBe('false');
  });
});
