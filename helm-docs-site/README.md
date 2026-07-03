# helm-docs-site

Docusaurus-powered documentation site for the [Camunda Helm Charts](https://github.com/camunda/camunda-platform-helm) repository.

**Published at:** https://helm.camunda.io/camunda-platform-helm/

## Overview

This directory contains the Docusaurus project configuration. Markdown content lives in [`../docs/`](../docs/) (repo root), keeping documentation alongside the code it describes.

```
camunda-platform-helm/
├── helm-docs-site/     ← Docusaurus project (you are here)
└── docs/               ← Markdown content
```

## Local Development

### Prerequisites

- Node.js 20 LTS — install via `asdf install nodejs 20` (`.tool-versions` is updated)
- npm (bundled with Node)

### Run the site locally

```bash
# from this directory
npm install
npm start
```

The site opens at http://localhost:3000/camunda-platform-helm/

### Build

```bash
npm run build
```

Output is written to `helm-docs-site/build/`. This is what gets deployed to GitHub Pages.

### Serve the production build locally

```bash
npm run build && npm run serve
```

## Adding or Editing Documentation

1. Edit or create Markdown files in `../docs/` (relative to this directory).
2. If adding a **new page**, also add its `id` to [`sidebars.js`](./sidebars.js).
3. Run `npm start` to preview locally before opening a PR.

### Sidebar conventions

- Sidebar order is explicit (defined in `sidebars.js`) — do **not** use `sidebar_position` frontmatter.
- Page IDs are derived from the filename without the `.md` extension (e.g., `docs/release-process.md` → id `release-process`).
- The landing page (`docs/index.md`) must keep id `index` and is listed first in the sidebar.

### Frontmatter

Minimal frontmatter is enough:

```markdown
---
id: my-page
title: My Page Title
---
```

`id` and `title` are the only required fields.

## Pre-commit hook (package-lock.json sync)

A pre-commit hook ensures `package-lock.json` stays in sync whenever `package.json` is modified. The hook runs `npm install` automatically and stages the updated lock file. No manual action is needed.

## PR Previews

When a PR touches `docs/**` or `helm-docs-site/**`, the CI workflow builds the site with a PR-scoped `BASE_URL` and deploys it to:

```
https://helm.camunda.io/camunda-platform-helm/pr-preview/pr-<number>/
```

The PR preview URL is posted as a comment on the pull request automatically.
