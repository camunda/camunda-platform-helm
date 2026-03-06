import { defineConfig } from "@playwright/test";

const parseEnvInt = (name: string, fallback: number): number => {
  const raw = process.env[name];
  if (!raw) return fallback;
  const parsed = Number.parseInt(raw, 10);
  return Number.isFinite(parsed) ? parsed : fallback;
};

export default defineConfig({
  testDir: "./tests",
  projects: [
    {
      name: "full-suite",
      testMatch: ["**/*.spec.{ts,js}"],
    },
  ],
  fullyParallel: true,
  retries: parseEnvInt("PLAYWRIGHT_RETRIES", 3),
  timeout: parseEnvInt("PLAYWRIGHT_TEST_TIMEOUT_MS", 120000),
  workers: process.env.CI === "true" ? 1 : "25%",
  reporter: [
    ["html", { open: "never" }],
    ["list"],
    ["junit", { outputFile: "test-results/results.xml" }],
  ],
  use: { baseURL: "https://camunda.local", trace: "on-first-retry" },
});
