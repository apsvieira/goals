import { Page, Locator } from '@playwright/test';

export class GoalEditorPage {
  readonly page: Page;
  readonly nameInput: Locator;
  readonly targetCountInput: Locator;
  readonly targetPeriodSelect: Locator;
  readonly saveButton: Locator;
  readonly cancelButton: Locator;
  readonly deleteButton: Locator;

  constructor(page: Page) {
    this.page = page;
    this.nameInput = page.locator('input[placeholder="Goal name"]');
    this.targetCountInput = page.locator('input[type="number"]');
    this.targetPeriodSelect = page.locator('select');
    this.saveButton = page.locator('button:has-text("Save")');
    this.cancelButton = page.locator('button:has-text("Cancel")');
    this.deleteButton = page.locator('button:has-text("Archive")');
  }

  async fillGoalDetails(name: string, targetCount?: number, targetPeriod?: 'week' | 'month') {
    await this.nameInput.fill(name);

    if (targetCount !== undefined) {
      await this.targetCountInput.fill(targetCount.toString());
    }

    if (targetPeriod) {
      await this.targetPeriodSelect.selectOption(targetPeriod);
    }
  }

  async save() {
    await this.saveButton.click();
    // Wait for editor to close
    await this.page.waitForSelector('input[placeholder="Goal name"]', { state: 'hidden', timeout: 5000 });
  }

  async cancel() {
    await this.cancelButton.click();
    // Wait for editor to close
    await this.page.waitForSelector('input[placeholder="Goal name"]', { state: 'hidden', timeout: 5000 });
  }

  async delete() {
    await this.deleteButton.click();
    // Wait for editor to close
    await this.page.waitForSelector('input[placeholder="Goal name"]', { state: 'hidden', timeout: 5000 });
  }

  async isVisible(): Promise<boolean> {
    return await this.nameInput.isVisible();
  }
}
