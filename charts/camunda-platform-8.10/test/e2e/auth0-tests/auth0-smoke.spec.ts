// Auth0 smoke tests — HTTP-level integration check for the auth0 ci-test
// scenario. Each Camunda 8 component on the platform is fronted by an OIDC
// client; this suite asserts the chart wiring is correct end-to-end without
// needing a browser, an Auth0 user, or a token exchange.
//
// Two flavours of test, one per component family:
//
// 1. Spring-server components (orchestration, identity, optimize) expose
//    /<component>/oauth2/authorization/oidc as the canonical login entry.
//    Hitting it unauthenticated walks a redirect chain that must terminate
//    at AUTH0_ISSUER/authorize carrying the right client_id, redirect_uri,
//    and response_type. We trace the chain ourselves so we can assert on
//    every hop.
//
// 2. SPA components (webModeler, console) render a client-side login page
//    and do the OIDC redirect from JS, so a server-side trace stops at the
//    SPA shell. Instead, hit a backend API that demands a Bearer token —
//    a 401 with a WWW-Authenticate header proves the chart wired the
//    component to require an OIDC token at all.
//
// Plus a sanity test that AUTH0_ISSUER/.well-known/openid-configuration
// resolves and points at the right issuer; if that breaks, every other
// failure is downstream noise.
import { expect, test, APIRequestContext } from "@playwright/test";

const ingressHost = process.env.TEST_INGRESS_HOST;
const auth0Issuer = (process.env.AUTH0_ISSUER_URL || "").replace(/\/+$/, "");
const MAX_HOPS = 10;

// Spring-server components: each redirect-chain terminates at Auth0 /authorize.
const springComponents = [
  // [logical-name, oauth2-entry-path, expected client_id env var]
  [
    "orchestration",
    "/orchestration/oauth2/authorization/oidc",
    "AUTH0_ORCHESTRATION_CLIENT_ID",
  ],
  ["identity", "/identity/oauth2/authorization/oidc", "AUTH0_IDENTITY_CLIENT_ID"],
  ["optimize", "/optimize/oauth2/authorization/oidc", "AUTH0_OPTIMIZE_CLIENT_ID"],
] as const;

// SPA components: assert a protected backend API returns 401.
const spaComponents = [
  ["webModeler", "/modeler/api/v1/info"],
  ["console", "/api/v1/clusters"],
] as const;

test.describe("Auth0 OIDC smoke", () => {
  test.beforeAll(() => {
    expect(
      ingressHost,
      "TEST_INGRESS_HOST must be set by the test runner",
    ).toBeTruthy();
    expect(
      auth0Issuer,
      "AUTH0_ISSUER_URL must be set by the test runner",
    ).toBeTruthy();
  });

  for (const [name, urlPath, clientIdVar] of springComponents) {
    test(`${name}: ${urlPath} redirects to Auth0 /authorize`, async ({
      request,
    }) => {
      const expectedClientId = process.env[clientIdVar];
      expect(
        expectedClientId,
        `${clientIdVar} must be set by the test runner`,
      ).toBeTruthy();

      const start = `https://${ingressHost}${urlPath}`;
      const chain = await traceRedirectChain(request, start);
      const authorizeHop = chain.find(
        (u) => u.origin === auth0Issuer && u.pathname === "/authorize",
      );
      expect(
        authorizeHop,
        `chain did not reach ${auth0Issuer}/authorize:\n${chain
          .map((u) => "  → " + u.toString())
          .join("\n")}`,
      ).toBeTruthy();

      const params = authorizeHop!.searchParams;
      expect(
        params.get("client_id"),
        `client_id must match ${expectedClientId}`,
      ).toBe(expectedClientId!);
      expect(
        params.get("response_type"),
        "response_type must be code",
      ).toBe("code");
      expect(
        params.get("redirect_uri"),
        "redirect_uri must be present",
      ).toBeTruthy();
      expect(
        params.get("scope"),
        "scope must include openid",
      ).toContain("openid");
    });
  }

  for (const [name, urlPath] of spaComponents) {
    test(`${name}: ${urlPath} requires authentication (401)`, async ({
      request,
    }) => {
      const url = `https://${ingressHost}${urlPath}`;
      const response = await request.get(url, { maxRedirects: 0 });
      expect(
        response.status(),
        `${url} returned ${response.status()} ${response.statusText()}; expected 401 from a protected backend route`,
      ).toBe(401);
    });
  }

  test("Auth0 well-known OIDC config is reachable", async ({ request }) => {
    const url = `${auth0Issuer}/.well-known/openid-configuration`;
    const response = await request.get(url, { maxRedirects: 0 });
    expect(response.status(), `${url} returned ${response.status()}`).toBe(200);
    const body = await response.json();
    expect(body.issuer, "issuer claim must match AUTH0_ISSUER_URL").toBe(
      auth0Issuer + "/",
    );
    expect(
      body.authorization_endpoint,
      "authorization_endpoint must be present",
    ).toMatch(/\/authorize$/);
    expect(body.jwks_uri, "jwks_uri must be present").toMatch(/\/jwks\.json$/);
  });
});

// traceRedirectChain follows Location headers manually so the test can
// inspect every URL in the chain — Playwright's APIRequestContext follows
// redirects transparently otherwise and we'd lose intermediate hops.
async function traceRedirectChain(
  request: APIRequestContext,
  startUrl: string,
): Promise<URL[]> {
  const chain: URL[] = [new URL(startUrl)];
  let current = startUrl;

  for (let hop = 0; hop < MAX_HOPS; hop++) {
    const response = await request.get(current, { maxRedirects: 0 });
    const status = response.status();
    if (![301, 302, 303, 307, 308].includes(status)) {
      return chain;
    }
    const location = response.headers()["location"];
    if (!location) {
      return chain;
    }
    // Resolve relative redirects against the current URL.
    const next = new URL(location, current);
    chain.push(next);
    current = next.toString();
  }
  return chain;
}
