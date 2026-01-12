import { Page, Locator } from '@playwright/test';

export class HomePage {
  readonly page: Page;
  readonly header: Locator;
  readonly newGoalButton: Locator;
  readonly goalsList: Locator;
  readonly monthNavigator: Locator;
  readonly prevMonthButton: Locator;
  readonly nextMonthButton: Locator;
  readonly profileButton: Locator;
  readonly offlineBanner: Locator;
  readonly syncBanner: Locator;

  constructor(page: Page) {
    this.page = page;
    this.header = page.locator('header');
    this.newGoalButton = page.locator('button:has-text("New Goal")');
    this.goalsList = page.locator('.goals');
    this.monthNavigator = page.locator('header h1');
    this.prevMonthButton = page.locator('button[aria-label="Previous month"]').or(page.locator('button').filter({ hasText: '←' }).first());
    this.nextMonthButton = page.locator('button[aria-label="Next month"]').or(page.locator('button').filter({ hasText: '→' }).last());
    this.profileButton = page.locator('button').filter({ hasText: /Profile|User/ });
    this.offlineBanner = page.locator('.offline-banner');
    this.syncBanner = page.locator('.sync-banner');
  }

  async goto() {
    await this.page.goto('/');
    await this.header.waitFor({ state: 'visible', timeout: 10000 });
  }

  async createGoal(name: string, targetCount?: number, targetPeriod?: 'week' | 'month') {
    await this.newGoalButton.click();
    await this.page.fill('input[placeholder="Goal name"]', name);

    if (targetCount && targetPeriod) {
      await this.page.fill('input[type="number"]', targetCount.toString());
      await this.page.selectOption('select', targetPeriod);
    }

    await this.page.click('button:has-text("Add Goal")');

    // Wait for editor to close (input should disappear)
    await this.page.waitForSelector('input[placeholder="Goal name"]', { state: 'hidden', timeout: 5000 });

    // Wait for goal to appear in goal row on main page
    await this.page.waitForSelector(`.goal-row:has-text("${name}")`, { timeout: 5000 });
  }

  async toggleCompletion(goalName: string, day: number) {
    // Find the goal row
    const goalRow = this.page.locator('.goal-row').filter({ hasText: goalName });

    // Find the day button using aria-label for exact matching
    const dayButton = goalRow.locator(`button[aria-label="Day ${day}"]`);
    await dayButton.click();
  }

  async navigateToMonth(direction: 'prev' | 'next') {
    if (direction === 'prev') {
      await this.prevMonthButton.click();
    } else {
      await this.nextMonthButton.click();
    }
    // Wait for calendar to update
    await this.page.waitForTimeout(500);
  }

  async getGoalRow(goalName: string): Promise<Locator> {
    return this.page.locator('.goal-row').filter({ hasText: goalName });
  }

  async isOffline(): Promise<boolean> {
    return await this.offlineBanner.isVisible();
  }

  async isSyncing(): Promise<boolean> {
    return await this.syncBanner.isVisible();
  }
}
