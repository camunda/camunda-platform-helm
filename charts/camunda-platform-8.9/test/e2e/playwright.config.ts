/// <reference types="node" />
/// <reference lib="esnext" />

import { defineConfig } from "@playwright/test";
import * as dotenv from "dotenv";

import { makeShadowConfig } from "../../../../test/e2e/playwright.base.config";

dotenv.config();

export default defineConfig(
  makeShadowConfig({
    version: "SM-8.9",
    extraTestIgnore: ["**/optimize-api-tests.spec.{ts,js}"],
  }),
);
