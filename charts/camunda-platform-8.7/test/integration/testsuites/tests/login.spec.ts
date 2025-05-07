// NOTE: If you get the message connection error: desc = "error reading server preface: http2: frame too large"
// this is likely due to --insecure on the zbctl call while the endpoint is TLS enabled.

// camunda.spec.ts
import { config as dotenv } from 'dotenv';
dotenv();                                 // ← loads .env before anything else

import { test, expect, request } from '@playwright/test';
import { execFileSync } from 'child_process';

// ---------- config & helpers ----------

const authURL = process.env.AUTH_URL!;
const base = {
  console:      process.env.CONSOLE_BASE_URL!,
  keycloak:     process.env.KEYCLOAK_BASE_URL!,
  identity:     process.env.IDENTITY_BASE_URL!,
  operate:      process.env.OPERATE_BASE_URL!,
  optimize:     process.env.OPTIMIZE_BASE_URL!,
  tasklist:     process.env.TASKLIST_BASE_URL!,
  webModeler:   process.env.WEBMODELER_BASE_URL!,
  connectors:   process.env.CONNECTORS_BASE_URL!,
  zeebeGRPC:    process.env.ZEEBE_GATEWAY_GRPC!,
  zeebeREST:    process.env.ZEEBE_GATEWAY_REST!,
};

const loginPath = {
  Console:      process.env.CONSOLE_LOGIN_PATH!,
  Keycloak:     process.env.KEYCLOAK_LOGIN_PATH!,
  Identity:     process.env.IDENTITY_LOGIN_PATH!,
  Operate:      process.env.OPERATE_LOGIN_PATH!,
  Optimize:     process.env.OPTIMIZE_LOGIN_PATH!,
  Tasklist:     process.env.TASKLIST_LOGIN_PATH!,
  WebModeler:   process.env.WEBMODELER_LOGIN_PATH!,
  connectors:   process.env.CONNECTORS_LOGIN_PATH!,
  zeebeGRPC:    process.env.ZEEBE_GATEWAY_GRPC!,
  zeebeREST:    process.env.ZEEBE_GATEWAY_REST!,
};
const secrets = {
  connectors: process.env.CONNECTORS_CLIENT_SECRET!,
  tasklist:   process.env.TASKLIST_CLIENT_SECRET!,
  operate:    process.env.OPERATE_CLIENT_SECRET!,
  optimize:   process.env.OPTIMIZE_CLIENT_SECRET!,
  zeebe:      process.env.ZEEBE_CLIENT_SECRET!,
};

const venomID  = process.env.TEST_CLIENT_ID  ?? 'venom';
const venomSec = process.env.TEST_CLIENT_SECRET!;

async function token(id: string, sec: string, api: request.APIRequestContext) {
  const r = await api.post(authURL, {
    form: { client_id: id, client_secret: sec, grant_type: 'client_credentials' },
  });
  expect(r.ok()).toBeTruthy();
  return (await r.json()).access_token as string;
}

// ---------- tests ----------
test.describe('Camunda core', () => {
  let api: request.APIRequestContext;
  let venomJWT: string;

  test.beforeAll(async ({ playwright }) => {
    api = await playwright.request.newContext();
    venomJWT = await token(venomID, venomSec, api);
  });

  test('M2M tokens', async () => {
    for (const [id, sec] of Object.entries(secrets)) {
      // ensure each call resolves and yields a non-empty JWT:
      await expect(token(id, sec, api)).resolves.toMatch(/^[\w-]+\.[\w-]+\.[\w-]+$/);
    }
  });

  for (const [name, url] of Object.entries({
    Console:    base.console,
    Keycloak:   base.keycloak,
    Identity:   base.identity,
    Operate:    base.operate,
    Optimize:   base.optimize,
    Tasklist:   base.tasklist,
    WebModeler: base.webModeler,
  })) {
    test(`Login page: ${name}`, async () => {
      const r = await api.get(`${url}${loginPath[name]}`, { timeout: 45_000 });
      expect(r.ok()).toBeTruthy();
      expect(await r.text()).not.toMatch(/error/i);
    });
  }

  test('Connectors inbound page', async () => {
    expect((await api.get(base.connectors, { timeout: 45_000 })).ok()).toBeTruthy();
  });

  for (const [label, url, method, body] of [
    ['Console clusters', `${base.console}/api/clusters`, 'GET', ''],
    ['Identity users', `${base.identity}api/users`, 'GET', ''],
    ['Operate defs', `${base.operate}/v1/process-definitions/search`, 'POST', '{}'],
    ['Tasklist tasks', `${base.tasklist}/graphql`, 'POST', '{"query":"{tasks(query:{}){id name}}"}'],
  ] as const) {
    test(`API: ${label}`, async () => {
      const r = await api.fetch(url, {
        method,
        data: body || undefined,
        headers: { Authorization: `Bearer ${venomJWT}`, 'Content-Type': 'application/json' },
      });
      expect(r.ok()).toBeTruthy();
    });
  }

  test('WebModeler login page', async () => {
    const r = await api.get(base.webModeler, { timeout: 45_000 });
    expect(r.ok()).toBeTruthy();
    expect(await r.text()).not.toMatch(/error/i);
  });

  test('Zeebe status (gRPC)', async () => {
    const extra = process.env.ZBCTL_EXTRA_ARGS?.trim().split(/\s+/).filter(Boolean) ?? [];

    const out = execFileSync('zbctl', ['status',
      '--clientCache','/tmp/zeebe','--clientId',venomID,'--clientSecret',venomSec,
      '--authzUrl',authURL,'--address',base.zeebeGRPC,
      ...extra,
    ],{ encoding:'utf-8' });
    expect(out).toMatch(/Leader, Healthy/);
    expect(out).not.toMatch(/Unhealthy/);
  });

  test('Zeebe topology (REST)', async () => {
    const r = await api.get(`${base.zeebeREST}/v1/topology`, {
      headers:{ Authorization:`Bearer ${venomJWT}` },
    });

    expect(r.ok()).toBeTruthy();
    expect(await r.json()).toHaveProperty('brokers');
  });

  for (const [name,file] of [['Basic','test-process.bpmn'],['Inbound','test-inbound-process.bpmn']] as const) {
    const extra = process.env.ZBCTL_EXTRA_ARGS?.trim().split(/\s+/).filter(Boolean) ?? [];
    test(`Deploy BPMN: ${name}`, async () => {
      execFileSync('zbctl', ['deploy',`../../../../../test/integration/testsuites/core/files/${file}`,
        '--clientCache','/tmp/zeebe','--clientId',venomID,'--clientSecret',venomSec,
        '--authzUrl',authURL,'--address',base.zeebeGRPC,
        ...extra,
      ],{ stdio:'inherit' });
    });
  }

  for (const [bpmnId,label] of [['it-test-process','Basic'],['test-inbound-process','Inbound']] as const) {
    test(`Process visible: ${label}`, async () => {
      const r = await api.post(`${base.operate}/v1/process-definitions/search`, {
        data:'{}',
        headers:{ Authorization:`Bearer ${venomJWT}`,'Content-Type':'application/json' },
      });
      expect(r.ok()).toBeTruthy();
      const data = await r.json();
      const ids = data.items.map((i: any) => i.bpmnProcessId);
      expect(ids).toContain(bpmnId);
    });
  }
});
