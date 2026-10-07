import { defineConfig } from '@playwright/test';
import { BASE, PORT } from './support/env.mjs';

const ci = !!process.env.CI;

export default defineConfig({
  testDir: './tests',
  // One worker, in file order: the suite shares one database and one clinic.
  workers: 1,
  fullyParallel: false,
  retries: ci ? 1 : 0,
  timeout: 60_000,
  expect: { timeout: 10_000 },
  reporter: ci ? [['list'], ['html', { open: 'never' }]] : [['list']],
  outputDir: 'test-results',
  globalSetup: './support/global-setup.ts',
  use: {
    baseURL: BASE,
    storageState: '.tmp/state.json',
    viewport: { width: 1280, height: 800 },
    locale: 'es-MX',
    timezoneId: 'America/Mexico_City',
    trace: 'retain-on-failure',
    screenshot: 'only-on-failure',
    launchOptions: {
      args: ['--no-sandbox'],
      // Locally you can point to an already installed Chromium (E2E_CHROMIUM_PATH); CI uses Playwright's own.
      executablePath: process.env.E2E_CHROMIUM_PATH || undefined
    }
  },
  webServer: {
    command: 'node support/stack.mjs',
    url: `http://localhost:${PORT}/api/health`,
    timeout: 240_000,
    reuseExistingServer: false,
    stdout: 'pipe',
    stderr: 'pipe'
  }
});
