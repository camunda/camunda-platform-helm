// @ts-check

/** @type {import('@docusaurus/types').Config} */
const config = {
  title: 'Camunda Helm Charts',
  tagline: 'Internal developer documentation for the Camunda 8 Helm charts repository',
  favicon: 'img/camunda-logo.svg',

  // GitHub Pages hosting
  url: 'https://helm.camunda.io',
  // BASE_URL env var is overridden in PR preview builds
  baseUrl: process.env.BASE_URL || '/camunda-platform-helm/',

  organizationName: 'camunda',
  projectName: 'camunda-platform-helm',

  onBrokenLinks: 'throw',
  onBrokenMarkdownLinks: 'warn',

  i18n: {
    defaultLocale: 'en',
    locales: ['en'],
  },

  markdown: {
    mermaid: true,
  },

  themes: [
    '@docusaurus/theme-mermaid',
    'docusaurus-theme-github-codeblock',
  ],

  presets: [
    [
      'classic',
      /** @type {import('@docusaurus/preset-classic').Options} */
      ({
        docs: {
          // Markdown content lives in docs/ at repo root (sibling of helm-docs-site/)
          path: '../docs',
          routeBasePath: '/',
          sidebarPath: require.resolve('./sidebars.js'),
          editUrl:
            'https://github.com/camunda/camunda-platform-helm/edit/main/',
        },
        blog: false,
        theme: {
          customCss: require.resolve('./src/css/custom.css'),
        },
      }),
    ],
  ],

  plugins: [
    [
      require.resolve('docusaurus-lunr-search'),
      {
        languages: ['en'],
      },
    ],
  ],

  themeConfig:
    /** @type {import('@docusaurus/preset-classic').ThemeConfig} */
    ({
      image: 'img/camunda-logo.svg',
      navbar: {
        title: 'Camunda Helm Charts',
        logo: {
          alt: 'Camunda Logo',
          src: 'img/camunda-logo.svg',
        },
        items: [
          {
            type: 'docSidebar',
            sidebarId: 'docs',
            position: 'left',
            label: 'Docs',
          },
          {
            href: 'https://github.com/camunda/camunda-platform-helm',
            label: 'GitHub',
            position: 'right',
          },
        ],
      },
      footer: {
        style: 'dark',
        links: [
          {
            title: 'Docs',
            items: [
              {
                label: 'Release Process',
                to: '/release-process',
              },
              {
                label: 'GitHub Actions Workflows',
                to: '/github-actions-workflows',
              },
              {
                label: 'Contribution & Collaboration',
                to: '/contribution-and-collaboration',
              },
            ],
          },
          {
            title: 'Community',
            items: [
              {
                label: 'GitHub Issues',
                href: 'https://github.com/camunda/camunda-platform-helm/issues',
              },
              {
                label: 'Camunda Docs',
                href: 'https://docs.camunda.io',
              },
            ],
          },
          {
            title: 'More',
            items: [
              {
                label: 'GitHub',
                href: 'https://github.com/camunda/camunda-platform-helm',
              },
              {
                label: 'Artifact Hub',
                href: 'https://artifacthub.io/packages/helm/camunda/camunda-platform',
              },
            ],
          },
        ],
        copyright: `Copyright © ${new Date().getFullYear()} Camunda Services GmbH. Built with Docusaurus.`,
      },
      prism: {
        // Additional languages beyond the default set
        additionalLanguages: ['bash', 'yaml', 'json', 'go'],
      },
      mermaid: {
        theme: { light: 'default', dark: 'dark' },
      },
      // docusaurus-theme-github-codeblock
      codeblock: {
        showGithubLink: true,
        githubLinkLabel: 'View on GitHub',
        showRunmeLink: false,
      },
    }),
};

module.exports = config;
