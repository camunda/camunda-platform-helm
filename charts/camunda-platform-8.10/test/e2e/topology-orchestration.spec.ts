import { expect, test } from "@playwright/test";

test("Orchestration topology is reachable", async ({ request }) => {
  const baseURL = process.env.BASE_URL;
  const keycloakURL = process.env.KEYCLOAK_BASE_URL;
  const clientSecret = process.env.DISTRO_QA_E2E_TESTS_KEYCLOAK_CLIENTS_SECRET;
  expect(baseURL).toBeTruthy();
  expect(keycloakURL).toBeTruthy();
  expect(clientSecret).toBeTruthy();

  const tokenResponse = await request.post(
    `${keycloakURL}/realms/camunda-platform/protocol/openid-connect/token`,
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
  const topologyResponse = await request.get(
    `${baseURL}/orchestration/v2/topology`,
    { headers: { Authorization: `Bearer ${accessToken}` } },
  );
  expect(topologyResponse.ok()).toBeTruthy();
});
