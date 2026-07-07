declare const process: {
  env: Record<string, string | undefined>;
};

type Project = {
  name: string;
  testDir?: string;
  testMatch?: string[];
  testIgnore?: Array<string | RegExp>;
  dependencies?: string[];
  grep?: RegExp;
  use?: {
    extraHTTPHeaders?: Record<string, string>;
  };
};

type ShadowConfig = {
  testDir: string;
  projects: Project[];
  fullyParallel: boolean;
  retries: number;
  timeout: number;
  workers: number | string;
  use: {
    baseURL: string;
    actionTimeout: number;
    screenshot: "only-on-failure";
    video: "retain-on-failure";
    trace: "on-first-retry";
  };
};

type ShadowVersion = "SM-8.7" | "SM-8.9" | "SM-8.10";

export function makeShadowConfig(opts: {
  version: ShadowVersion;
  testDir?: string;
  includeSetupProject?: boolean;
  extraProjects?: Project[];
}): ShadowConfig {
  if (!process.env.CAMUNDA_OPTIMIZE_BASE_URL && process.env.BASE_URL) {
    process.env.CAMUNDA_OPTIMIZE_BASE_URL = `https://${process.env.BASE_URL}/optimize`;
  }

  const includeSetupProject = opts.includeSetupProject ?? true;

  return {
    testDir:
      opts.testDir ??
      `./node_modules/@camunda/e2e-test-suite/dist/tests/${opts.version}`,
    projects: [
      {
        name: "smoke-tests",
        testMatch: ["**/smoke-tests.spec.{ts,js}"],
      },
      ...(
        includeSetupProject
          ? [
              {
                name: "full-suite-setup",
                testMatch: ["**/test-setup.spec.{ts,js}"],
                use: {
                  extraHTTPHeaders: {
                    "X-Test-Tasklist-Version": "v2",
                  },
                },
              },
            ]
          : []
      ),
      {
        name: "full-suite",
        dependencies: includeSetupProject ? ["full-suite-setup"] : undefined,
        testMatch: ["**/*.spec.{ts,js}"],
        testIgnore: [
          "**/cluster-variables.spec.{ts,js}",
          "**/test-setup.spec.{ts,js}",
        ],
        grep: /^(?!.*(@tasklistV1|Connector Secrets User Flow|Custom Tags|Custom Properties)).*$/,
        use: {
          extraHTTPHeaders: {
            "X-Test-Tasklist-Version": "v2",
          },
        },
      },
      ...(opts.extraProjects ?? []),
    ],
    fullyParallel: false,
    retries: 1,
    timeout: 12 * 60 * 1000,
    workers: workerCount(opts.version),
    use: {
      baseURL: getBaseURL(),
      actionTimeout: 10000,
      screenshot: "only-on-failure",
      video: "retain-on-failure",
      trace: "on-first-retry",
    },
  };
}

function workerCount(version: ShadowVersion): number | string {
  if (process.env.CI !== "true") {
    return "100%";
  }

  return version === "SM-8.7" ? 37 : 25;
}

function getBaseURL(): string {
  if (process.env.IS_PROD === "true") {
    return "https://console.camunda.io";
  }

  if (typeof process.env.PLAYWRIGHT_BASE_URL === "string") {
    return process.env.PLAYWRIGHT_BASE_URL;
  }

  if (process.env.MINOR_VERSION?.includes("SM")) {
    return "https://gke-" + process.env.BASE_URL + ".ci.distro.ultrawombat.com";
  }

  if (process.env.MINOR_VERSION?.includes("Run")) {
    return "http://localhost:8080";
  }

  return "https://console.cloud.ultrawombat.com";
}
