import assert from "node:assert/strict";
import test from "node:test";

import { makeShadowConfig } from "./playwright.base.config.ts";

test("SM 8.9 Tasklist v1 project uses its setup, header, and tag filter", () => {
  const config = makeShadowConfig({
    version: "SM-8.9",
    includeSetupProject: true,
    includeTasklistV1Projects: true,
  });
  const project = config.projects.find(({ name }) => name === "full-suite-v1");
  const setup = config.projects.find(
    ({ name }) => name === "full-suite-v1-setup",
  );

  assert.deepEqual(project?.dependencies, ["full-suite-v1-setup"]);
  assert.equal(
    project?.use?.extraHTTPHeaders?.["X-Test-Tasklist-Version"],
    "v1",
  );
  assert.equal(
    setup?.use?.extraHTTPHeaders?.["X-Test-Tasklist-Version"],
    "v1",
  );
  assert.equal(project?.grep?.test("Tasklist test @tasklistV2"), false);
  assert.equal(project?.grep?.test("Tasklist test @tasklistV1"), true);

  const unsupportedConfig = makeShadowConfig({ version: "SM-8.7" });
  assert.equal(
    unsupportedConfig.projects.some(({ name }) => name.includes("full-suite-v1")),
    false,
  );
});
