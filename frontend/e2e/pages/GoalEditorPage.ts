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
    // Button text is "Add Goal" when creating, "Save" when editing
    this.saveButton = page.locator('button:has-text("Add Goal")').or(page.locator('button:has-text("Save")'));
    this.cancelButton = page.locator('button:has-text("Cancel")');
    this.deleteButton = page.locator('button:has-text("Delete")');
  }

  async fillGoalDetails(name: string, targetCount?: number, targetPeriod?: 'week' | 'month') {
    await this.nameInput.fill(name);

    // Select target type first (this makes the number input visible)
    if (targetPeriod) {
      const targetLabel = targetPeriod === 'week' ? 'Weekly target' : 'Monthly target';
      await this.page.locator(`label:has-text("${targetLabel}")`).click();

      // Wait for number input to appear
      await this.targetCountInput.waitFor({ state: 'visible', timeout: 5000 });

      // Fill the target count
      if (targetCount !== undefined) {
        await this.targetCountInput.fill(targetCount.toString());
      }
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
    // First click shows confirmation dialog
    await this.deleteButton.click();

    // Wait for confirmation dialog and click confirm button
    const confirmButton = this.page.locator('.confirm-dialog button:has-text("Delete")');
    await confirmButton.waitFor({ state: 'visible', timeout: 5000 });
    await confirmButton.click();

    // Wait for editor to close
    await this.page.waitForSelector('input[placeholder="Goal name"]', { state: 'hidden', timeout: 5000 });
  }

  async isVisible(): Promise<boolean> {
    return await this.nameInput.isVisible();
  }
}
