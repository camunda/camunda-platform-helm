// @ts-check

/** @type {import('@docusaurus/plugin-content-docs').SidebarsConfig} */
const sidebars = {
  docs: [
    {
      type: 'doc',
      id: 'index',
      label: 'Overview',
    },

    // ── For all contributors ──────────────────────────────────────────────────
    {
      type: 'category',
      label: 'For all contributors',
      collapsible: false,
      items: [
        {
          type: 'doc',
          id: 'contribution-and-collaboration',
          label: 'Contribution & Collaboration',
        },
        {
          type: 'category',
          label: 'Policies',
          collapsed: true,
          items: [
            {
              type: 'doc',
              id: 'policies/breaking-changes',
              label: 'Breaking Changes & Deprecation',
            },
            {
              type: 'doc',
              id: 'policies/backporting',
              label: 'Backporting',
            },
            {
              type: 'doc',
              id: 'policies/values-yaml-policy',
              label: 'Values YAML Policy',
            },
            {
              type: 'doc',
              id: 'policies/ticket-and-label',
              label: 'Ticket & Label',
            },
          ],
        },
      ],
    },

    // ── For maintainers & HC owners ───────────────────────────────────────────
    {
      type: 'category',
      label: 'For maintainers & HC owners',
      collapsible: false,
      items: [
        {
          type: 'doc',
          id: 'maintainer-guide',
          label: 'Maintainer Guide',
        },
        {
          type: 'doc',
          id: 'reference/release-process',
          label: 'Release Process',
        },
      ],
    },

    // ── Reference ─────────────────────────────────────────────────────────────
    {
      type: 'category',
      label: 'Reference',
      collapsible: false,
      items: [
        {
          type: 'doc',
          id: 'reference/code-style',
          label: 'Code Style',
        },
        {
          type: 'doc',
          id: 'reference/testing',
          label: 'Testing',
        },
        {
          type: 'doc',
          id: 'reference/github-actions-workflows',
          label: 'GitHub Actions Workflows',
        },
      ],
    },

    // ── Skills & Runbooks ─────────────────────────────────────────────────────
    {
      type: 'category',
      label: 'Skills & Runbooks',
      collapsible: false,
      items: [
        {
          type: 'doc',
          id: 'skills/reproducing-ci-e2e-failures',
          label: 'Reproducing CI E2E Failures',
        },
        {
          type: 'doc',
          id: 'skills/integration-test-scenario-resolution',
          label: 'Integration Test Scenario Resolution',
        },
      ],
    },
  ],
};

module.exports = sidebars;
