// Multitenancy smoke test that deploys BPMN models with different tenant IDs
// and verifies they are properly isolated

/// <reference types="node" />

import { config as dotenv } from "dotenv";
dotenv(); // ← loads .env before anything else

import { test, expect, APIRequestContext } from "@playwright/test";
import { execFileSync } from "child_process";
import { authHeader, fetchToken, requireEnv } from "./helper";

// ---------- config & helpers ----------

// Grouped config for base URLs  
const config = {
  authURL: requireEnv("AUTH_URL"),
  authType: requireEnv("TEST_AUTH_TYPE"),
  testBasePath: requireEnv("TEST_BASE_PATH"),
  base: {
    console: requireEnv("CONSOLE_BASE_URL"),
    keycloak: requireEnv("KEYCLOAK_BASE_URL"),
    identity: requireEnv("IDENTITY_BASE_URL"),
    coreTasklist: requireEnv("CORE_TASKLIST_BASE_URL"),
    coreOperate: requireEnv("CORE_OPERATE_BASE_URL"),
    optimize: requireEnv("OPTIMIZE_BASE_URL"),
    webModeler: requireEnv("WEBMODELER_BASE_URL"),
    connectors: requireEnv("CONNECTORS_BASE_URL"),
    zeebeGRPC: requireEnv("ZEEBE_GATEWAY_GRPC"),
    zeebeREST: requireEnv("ZEEBE_GATEWAY_REST"),
  },
  venomID: process.env.TEST_CLIENT_ID ?? "venom",
  venomSec: requireEnv("PLAYWRIGHT_VAR_CORE_CLIENT_SECRET"),
};

// Tenant configurations
const tenants = [
  { id: "tenant-a", processId: "tenant-a-process", file: "tenant-a-process.bpmn" },
  { id: "tenant-b", processId: "tenant-b-process", file: "tenant-b-process.bpmn" },
];

let api: APIRequestContext;

test.beforeAll(async ({ playwright }) => {
  api = await playwright.request.newContext({
    ignoreHTTPSErrors: true,
  });
});

test.describe("Multitenancy Smoke Tests", () => {
  
  // Skip if multitenancy scenario is not enabled
  test.beforeAll(async () => {
    const testScenario = process.env.TEST_SCENARIO;
    
    if (testScenario !== "multitenancy") {
      test.skip(true, `Multitenancy tests skipped - running scenario: ${testScenario || "unknown"}`);
    }
  });
  
  // Deploy models to different tenants
  for (const tenant of tenants) {
    test(`Deploy BPMN model to ${tenant.id}`, async () => {
      const extra =
        process.env.ZBCTL_EXTRA_ARGS?.trim().split(/\s+/).filter(Boolean) ?? [];
      
      // Deploy model with tenant ID
      execFileSync(
        "zbctl",
        [
          "deploy",
          `${config.testBasePath}/${tenant.file}`,
          "--clientCache",
          "/tmp/zeebe",
          "--clientId",
          config.venomID,
          "--clientSecret",
          config.venomSec,
          "--authzUrl",
          config.authURL,
          "--address",
          config.base.zeebeGRPC,
          "--tenantId",
          tenant.id,
          ...extra,
        ],
        { stdio: "inherit" },
      );
      
      // Wait for deployment to propagate
      await new Promise((resolve) => setTimeout(resolve, 15000));
    });
  }

  // Verify tenant isolation - each tenant should only see their own processes
  for (const tenant of tenants) {
    test(`Verify process visibility for ${tenant.id}`, async () => {
      // Get authentication token
      const token = await fetchToken(config.venomID, config.venomSec, api, config);
      
      // Search for process definitions in this tenant
      const r = await api.post(
        `${config.base.coreOperate}/v2/process-definitions/search`,
        {
          data: JSON.stringify({
            filter: {
              tenantId: tenant.id
            }
          }),
          headers: {
            Authorization: `Bearer ${token}`,
            "Content-Type": "application/json",
          },
        },
      );
      
      expect(
        r.ok(),
        `Process search failed for ${tenant.id}: ${r.status()}`,
      ).toBeTruthy();
      
      const data = await r.json();
      const processes = data.items as Array<{ 
        processDefinitionId: string, 
        tenantId: string 
      }>;
      
      // Verify our tenant's process is present
      const tenantProcesses = processes.filter(p => p.processDefinitionId === tenant.processId);
      expect(
        tenantProcesses.length, 
        `Process ${tenant.processId} not found for tenant ${tenant.id}`
      ).toBeGreaterThan(0);
      
      // Verify all processes belong to this tenant
      for (const process of processes) {
        expect(
          process.tenantId,
          `Process ${process.processDefinitionId} has wrong tenant ID`
        ).toBe(tenant.id);
      }
      
      // Verify other tenant's processes are NOT visible
      const otherTenants = tenants.filter(t => t.id !== tenant.id);
      for (const otherTenant of otherTenants) {
        const otherTenantProcesses = processes.filter(p => 
          p.processDefinitionId === otherTenant.processId
        );
        expect(
          otherTenantProcesses.length,
          `Process ${otherTenant.processId} should not be visible to tenant ${tenant.id}`
        ).toBe(0);
      }
    });
  }

  // Start process instances in different tenants
  for (const tenant of tenants) {
    test(`Start process instance in ${tenant.id}`, async () => {
      const extra =
        process.env.ZBCTL_EXTRA_ARGS?.trim().split(/\s+/).filter(Boolean) ?? [];
      
      // Start a process instance
      const output = execFileSync(
        "zbctl",
        [
          "create", 
          "instance",
          tenant.processId,
          "--clientCache",
          "/tmp/zeebe",
          "--clientId",
          config.venomID,
          "--clientSecret",
          config.venomSec,
          "--authzUrl",
          config.authURL,
          "--address",
          config.base.zeebeGRPC,
          "--tenantId",
          tenant.id,
          ...extra,
        ],
        { encoding: "utf8" },
      );
      
      // Verify instance was created successfully
      expect(output).toContain("Process instance created");
      
      // Wait for instance to propagate
      await new Promise((resolve) => setTimeout(resolve, 10000));
    });
  }

  // Verify process instances are isolated by tenant
  for (const tenant of tenants) {
    test(`Verify process instance isolation for ${tenant.id}`, async () => {
      // Get authentication token
      const token = await fetchToken(config.venomID, config.venomSec, api, config);
      
      // Search for process instances in this tenant
      const r = await api.post(
        `${config.base.coreOperate}/v2/process-instances/search`,
        {
          data: JSON.stringify({
            filter: {
              tenantId: tenant.id,
              processDefinitionId: tenant.processId
            }
          }),
          headers: {
            Authorization: `Bearer ${token}`,
            "Content-Type": "application/json",
          },
        },
      );
      
      expect(
        r.ok(),
        `Process instance search failed for ${tenant.id}: ${r.status()}`,
      ).toBeTruthy();
      
      const data = await r.json();
      const instances = data.items as Array<{ 
        processInstanceKey: string,
        processDefinitionId: string,
        tenantId: string 
      }>;
      
      // Verify we have instances for this tenant
      expect(
        instances.length,
        `No process instances found for tenant ${tenant.id}`
      ).toBeGreaterThan(0);
      
      // Verify all instances belong to this tenant and process
      for (const instance of instances) {
        expect(
          instance.tenantId,
          `Instance ${instance.processInstanceKey} has wrong tenant ID`
        ).toBe(tenant.id);
        expect(
          instance.processDefinitionId,
          `Instance ${instance.processInstanceKey} has wrong process ID`
        ).toBe(tenant.processId);
      }
    });
  }

  test.afterAll(async ({}, testInfo) => {
    // If the test outcome is different from what was expected (i.e. the test failed),
    // dump the resolved configuration so that it is visible in the Playwright output.
    if (testInfo.status !== testInfo.expectedStatus) {
      console.error(
        "\n===== CONFIG DUMP (test failed) =====\n" +
          JSON.stringify(config, null, 2),
      );
    }
  });
});
