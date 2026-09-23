import { expect } from "@playwright/test";
import { test } from "@camunda/e2e-test-suite/dist/fixtures/SM-8.10";

type Environment = {
  id: string;
  clusterId: string;
  physicalTenantId?: string;
  targetType: string;
};

const organizationId = "00000000-0000-0000-0000-000000000000";

test("Hub deploys through the selected Physical Tenant environment", async ({
  page,
  navigationPage,
}) => {
  const webModelerURL = process.env.WEBMODELER_BASE_URL;
  const physicalTenantId = process.env.PHYSICAL_TENANT_ID;
  expect(webModelerURL).toBeTruthy();
  expect(physicalTenantId).toBeTruthy();

  const authenticatedRequest = page.waitForRequest(
    (request) =>
      request.url().includes("/api/internal/v2/organizations/") &&
      Boolean(request.headers().authorization),
  );
  await navigationPage.goToModeler();
  const authorization = (await authenticatedRequest).headers().authorization;
  expect(authorization).toBeTruthy();
  const headers = { Authorization: authorization! };

  const environmentsResponse = await page.request.get(
    `${webModelerURL}/api/internal/v2/organizations/${organizationId}/environments`,
    { headers },
  );
  expect(environmentsResponse.ok()).toBeTruthy();
  const environments = (await environmentsResponse.json()) as Environment[];
  const orchestrationEnvironments = environments.filter(
    ({ clusterId }) => clusterId === "orcha",
  );
  expect(orchestrationEnvironments).toHaveLength(3);
  expect(
    orchestrationEnvironments.map(({ physicalTenantId: id, targetType }) => ({
      id,
      targetType,
    })),
  ).toEqual(
    expect.arrayContaining(
      ["default", "tenanta", "tenantb"].map((id) => ({
        id,
        targetType: "PHYSICAL_TENANT_BACKED",
      })),
    ),
  );
  const environment = orchestrationEnvironments.find(
    (candidate) => candidate.physicalTenantId === physicalTenantId,
  );
  expect(environment).toBeDefined();

  const suffix = Date.now().toString(36);
  await page.goto(`${webModelerURL}/workspaces/create`);
  await page.getByLabel("Workspace name").fill(`Physical Tenant ${suffix}`);
  const submit = page.locator('[data-test="workspace-wizard-submit"]');
  await submit.click();
  await expect(
    page.locator('[data-test="step-members"][data-state="active"]'),
  ).toBeVisible();
  await submit.click();
  await expect(
    page.locator('[data-test="step-environments"][data-state="active"]'),
  ).toBeVisible();
  await submit.click();
  await page.waitForURL(/\/workspaces\/[^/]+\/projects$/);
  const workspaceId = new URL(page.url()).pathname.split("/").at(-2)!;

  const assignmentResponse = await page.request.put(
    `${webModelerURL}/api/internal/v2/workspaces/${workspaceId}/environments`,
    { data: { environmentIds: [environment!.id] }, headers },
  );
  expect(assignmentResponse.ok()).toBeTruthy();

  const projectResponse = await page.request.post(
    `${webModelerURL}/api/internal/v2/projects`,
    { data: { workspaceId, name: `Physical Tenant ${suffix}` }, headers },
  );
  expect(projectResponse.ok()).toBeTruthy();
  const projectBody = (await projectResponse.json()) as {
    data?: { id: string };
    id?: string;
  };
  const projectId = projectBody.data?.id ?? projectBody.id;
  expect(projectId).toBeTruthy();

  const processId = `physical_tenant_${physicalTenantId}_${suffix}`;
  const content = `<?xml version="1.0" encoding="UTF-8"?>
<definitions xmlns="http://www.omg.org/spec/BPMN/20100524/MODEL" targetNamespace="http://camunda.io/schema/1.0/bpmn">
  <process id="${processId}" name="${processId}" isExecutable="true">
    <startEvent id="start" />
  </process>
</definitions>`;
  const fileResponse = await page.request.post(
    `${webModelerURL}/api/internal/v2/files`,
    {
      data: {
        name: `${processId}.bpmn`,
        content,
        hubProjectId: projectId,
        folderId: projectId,
        type: "BPMN",
      },
      headers,
    },
  );
  expect(fileResponse.ok()).toBeTruthy();
  const fileBody = (await fileResponse.json()) as {
    data?: { id: string };
    id?: string;
  };
  const fileId = fileBody.data?.id ?? fileBody.id;
  expect(fileId).toBeTruthy();

  const deployResponse = await page.request.post(
    `${webModelerURL}/api/internal/v2/files/${fileId}/deploy`,
    { data: { environmentId: environment!.id }, headers },
  );
  expect(deployResponse.ok()).toBeTruthy();
});
