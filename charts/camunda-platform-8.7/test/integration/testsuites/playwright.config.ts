import { defineConfig } from "@playwright/test";
import * as dotenv from "dotenv";

dotenv.config();

export default defineConfig({
  testDir: "./tests",
  testMatch: ["**/*.spec.{ts,js}"],
  fullyParallel: true,
  retries: 3,
  timeout: 3 * 60 * 1000, // no test should take more than 3 minutes (failing fast is important so that we can run our tests on each PR)
  workers: process.env.CI ? 1 : "25%",
  use: {
    actionTimeout: 10000,
    screenshot: "only-on-failure",
  },
});
