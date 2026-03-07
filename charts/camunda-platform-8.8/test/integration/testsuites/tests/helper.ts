import { APIRequestContext, expect } from "@playwright/test";

// Helper to fetch a token
export async function fetchToken(
  id: string,
  sec: string,
  api: APIRequestContext,
  config: any,
) {
  if (config.authType !== "basic") {
    const form: Record<string, string> = {
      client_id: id,
      client_secret: sec,
      grant_type: "client_credentials",
    };
    // Entra v2.0 requires a scope for client_credentials grants.
    if (config.tokenScope) {
      form.scope = config.tokenScope;
    }
    const r = await api.post(config.authURL, { form });
    expect(
      r.ok(),
      `Failed to get token for client_id=${id}: ${r.status()}`,
    ).toBeTruthy();
    return (await r.json()).access_token as string;
  } else {
    return ""
  }
}

export const authHeader = async (api: APIRequestContext, config: any): Promise<string> => {
  if (config.authType === "basic") {
    return `Basic ${Buffer.from(
      `${config.demoUser}:${config.demoPass}`,
    ).toString("base64")}`;
  } else if (config.authType === "keycloak" || config.authType === "oidc") {
    return `Bearer ${await fetchToken(config.venomID, config.venomSec, api, config)}`;
  }
};

export function requireEnv(name: string): string {
  const value = process.env[name];
  if (!value) throw new Error(`Missing required env var: ${name}`);
  return value;
}
