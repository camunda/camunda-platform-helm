import { defineConfig } from "@playwright/test";
import * as dotenv from 'dotenv';

dotenv.config();

console.log(process.env);

export default defineConfig({
  testDir: "./node_modules/playwright-automation/dist/tests/SM-8.7",
  testMatch: ["**/smoke-tests.spec.{ts,js}"],
  fullyParallel: true,
  retries: 3,
  timeout: 3 * 60 * 1000, // no test should take more than 3 minutes (failing fast is important so that we can run our tests on each PR)
  workers: process.env.CI ? 1: '25%', 
  use: {
    baseURL: getBaseURL(),
    actionTimeout: 10000,
  },
});

function getBaseURL(): string {
  if (process.env.IS_PROD === 'true') {
    return 'https://console.camunda.io';
  }
  
  if (typeof process.env.PLAYWRIGHT_BASE_URL === 'string') {
    return process.env.PLAYWRIGHT_BASE_URL;
  }
  
  if (process.env.MINOR_VERSION?.includes('SM')) {
    return 'https://gke-' + process.env.BASE_URL + '.ci.distro.ultrawombat.com';
  }
  
  if (process.env.MINOR_VERSION?.includes('Run')) {
    return 'http://localhost:8080';
  }
  
  return 'https://console.cloud.ultrawombat.com';
}