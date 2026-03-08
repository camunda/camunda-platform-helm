import { APIRequestContext, expect } from "@playwright/test";

const sleep = (ms: number) => new Promise((resolve) => setTimeout(resolve, ms));

const shouldRetryStatus = (status: number): boolean =>
  status === 404 || status === 429 || status >= 500;

// Helper to fetch a token
export async function fetchToken(
  id: string,
  sec: string,
  api: APIRequestContext,
  config: any,
) {
  if (config.authType !== "basic") {
    const maxAttempts = Number.parseInt(
      process.env.PLAYWRIGHT_TOKEN_RETRY_MAX_ATTEMPTS || "8",
      10,
    );
    const retryDelayMs = Number.parseInt(
      process.env.PLAYWRIGHT_TOKEN_RETRY_DELAY_MS || "5000",
      10,
    );

    const form: Record<string, string> = {
      client_id: id,
      client_secret: sec,
      grant_type: "client_credentials",
    };
    // Entra v2.0 requires a scope for client_credentials grants.
    if (config.tokenScope) {
      form.scope = config.tokenScope;
    }

    let lastError = "";
    for (let attempt = 1; attempt <= maxAttempts; attempt += 1) {
      try {
        const r = await api.post(config.authURL, { form, timeout: 15_000 });
        if (r.ok()) {
          return (await r.json()).access_token as string;
        }

        const status = r.status();
        const body = await r.text();
        lastError = `status=${status}${body ? ` body=${body}` : ""}`;

        if (attempt < maxAttempts && shouldRetryStatus(status)) {
          await sleep(retryDelayMs);
          continue;
        }

        expect(
          r.ok(),
          `Failed to get token for client_id=${id}: ${status}`,
        ).toBeTruthy();
      } catch (err) {
        lastError = String(err);
        if (attempt < maxAttempts) {
          await sleep(retryDelayMs);
          continue;
        }
      }
    }

    throw new Error(
      `Failed to get token for client_id=${id} after ${maxAttempts} attempts: ${lastError || "unknown error"}`,
    );
  } else {
    return "";
  }
}

export const authHeader = async (
  api: APIRequestContext,
  config: any,
): Promise<string> => {
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
