import { expect, test } from "@playwright/test";

type Environment = {
  clusterId: string;
  physicalTenantId?: string;
  targetType: string;
};

test("Hub discovers the selected Physical Tenant", async ({ request }) => {
  const baseURL = process.env.BASE_URL;
  const clientSecret = process.env.DISTRO_QA_E2E_TESTS_KEYCLOAK_CLIENTS_SECRET;
  const physicalTenantId = process.env.PHYSICAL_TENANT_ID;
  expect(baseURL).toBeTruthy();
  expect(clientSecret).toBeTruthy();
  expect(physicalTenantId).toBeTruthy();

  const tokenResponse = await request.post(
    `${baseURL}/auth/realms/camunda-platform/protocol/openid-connect/token`,
    {
      form: {
        client_id: "venom",
        client_secret: clientSecret!,
        grant_type: "client_credentials",
      },
    },
  );
  expect(tokenResponse.ok()).toBeTruthy();
  const { access_token: accessToken } = (await tokenResponse.json()) as {
    access_token: string;
  };

  const response = await request.get(
    `${baseURL}/modeler/api/internal/v2/organizations/00000000-0000-0000-0000-000000000000/environments`,
    { headers: { Authorization: `Bearer ${accessToken}` } },
  );
  expect(response.ok()).toBeTruthy();
  const environments = (await response.json()) as Environment[];
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
  expect(
    orchestrationEnvironments.some(
      (environment) => environment.physicalTenantId === physicalTenantId,
    ),
  ).toBeTruthy();
});
