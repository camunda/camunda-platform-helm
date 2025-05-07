import { defineConfig } from '@playwright/test';

export default defineConfig({
  testDir: './tests',
  defaultTimeout: 30000,
  fullyParallel: true,
  retries: 2,
  reporter: [['html', { open: 'never' }], ['list'], ['junit', { outputFile: 'test-results/results.xml' }]],
  use: { baseURL: 'https://camunda.local', trace: 'on-first-retry' },
})
