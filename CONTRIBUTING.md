# Contributing to the Camunda Helm chart

We welcome new contributions. We take pride in maintaining and encouraging a friendly, welcoming, and collaborative community.

Anyone is welcome to contribute to Camunda! The best way to get started is to choose an existing [issue](#starting-on-an-issue).

For community-maintained Camunda projects, please visit the [Camunda Community Hub](https://github.com/camunda-community-hub). For connectors and process blueprints, please visit [Camunda Marketplace](https://marketplace.camunda.com/en-US/home) instead.

## Prerequisites

<!-- ### Contributor License Agreement -->
<!---->
<!-- You will be asked to sign our [Contributor License Agreement](https://cla-assistant.io/camunda-community-hub/community) when you open a Pull Request. We are not asking you to assign copyright to us but to give us the right to distribute your code without restriction. We ask this of all contributors to assure our users of the origin and continuing existence of the code. -->
<!---->
<!-- > [!NOTE] -->
<!-- > In most cases, you will only need to sign the CLA once. -->

### Code of Conduct

This project adheres to the [Camunda Code of Conduct](https://camunda.com/events/code-conduct/). By participating, you are expected to uphold this code. Please [report](https://camunda.com/events/code-conduct/reporting-violations/) unacceptable behavior as soon as possible.

## GitHub issue guidelines

If you want to report a bug or request a new feature, feel free to open a new issue on [GitHub][issues].

If you report a bug, please help speed up problem diagnosis by providing as much information as possible.

> [!NOTE]
> If you have a general usage question, please ask on the [forum][forum].

Every issue should have a meaningful name and a description that either describes:

- A new feature with details about the use case the feature would solve or
  improve
- A problem, how we can reproduce it, and what the expected behavior would be
- A change and the intention of how this would improve the system

<!-- ### Severity and Likelihood (bugs): -->
<!---->
<!-- To help us prioritize, please also determine the severity and likelihood of the bug. To help you with this, here are the definitions for the options: -->
<!---->
<!-- Severity: -->
<!-- - *Low:* Having little to no noticeable impact on usage for the user (e.g. log noise) -->
<!-- - *Mid:* Having a noticeable impact on production usage, which does not lead to data loss, or for which there is a known configuration workaround. -->
<!-- - *High:* Having a noticeable impact on production usage, which does not lead to data loss, but for which there is no known workaround, or the workaround is very complex. Examples include issues which lead to regular crashes and break the availability SLA. -->
<!-- - *Critical:* Stop-the-world issue with a high impact that can lead to data loss (e.g. corruption, deletion, inconsistency, etc.), unauthorized privileged actions (e.g. remote code execution, data exposure, etc.), and for which there is no existing configuration workaround. -->
<!-- - *Unknown:* If it's not possible to determine the severity of a bug without in-depth investigation, you can select unknown. This should be treated as high until we have enough information to triage it properly. -->
<!---->
<!-- Likelihood: -->
<!-- - *Low:* rarely observed issue/ rather unlikely edge-case -->
<!-- - *Mid:* occasionally observed -->
<!-- - *High:* recurring issue -->

<!-- #### Determining the severity of an issue -->
<!---->
<!-- Whenever possible, please try to determine the severity of an issue to the best of your knowledge. -->
<!-- Only select `Unknown` if it's really difficult to tell without spending a non-negligible amount of time (e.g. >1h) to -->
<!-- figure it out. -->

The `main` branch contains the current in-development state of the project. To work on an issue, follow these steps:

1. Check that a [GitHub issue][issues] exists for the task you want to work on.
   If one does not, create one. Refer to the [issue guidelines](#github-issue-guidelines).
2. Check that no one is already working on the issue, and make sure the team would accept a pull request for this topic. Some topics are complex and may touch multiple of [Camunda's Components](https://docs.camunda.io/docs/components/), requiring internal coordination.
3. Checkout the `main` branch and pull the latest changes.

   ```
   git checkout main
   git pull
   ```

4. Create a new branch with the naming scheme `issueId-description`.

   ```
   git checkout -b 123-adding-bpel-support
   ```

5. Implement the required changes on your branch and regularly push your
   changes to the origin so that the CI can run. Code formatting, style, and
   license header are fixed automatically by running Maven. Checkstyle
   violations have to be fixed manually.

   ```
   git commit -am 'feat: add BPEL execution support'
   git push -u origin 123-adding-bpel-support
   ```

6. If you think you finished the issue, please prepare the branch for review. Please consider our [pull requests and code reviews](https://github.com/camunda/camunda/wiki/Pull-Requests-and-Code-Reviews) guide, before requesting a review. In general, the commits should be squashed into meaningful commits with a helpful message. This means cleanup/fix etc. commits should be squashed into the related commit. If you made refactorings it would be best if they are split up into another commit. Think about how a reviewer can best understand your changes. Please follow the [commit message guidelines](#commit-message-guidelines).
