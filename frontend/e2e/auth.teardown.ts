import { test as teardown } from '@playwright/test';

teardown('cleanup', async ({}) => {
  // Optional: Add cleanup logic if needed
  // For now, we keep the auth file for reuse
  console.log('Test session complete');
});
