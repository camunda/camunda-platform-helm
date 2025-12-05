package deploy

import (
	"fmt"
	"os"
	"scripts/camunda-core/pkg/logging"
	"scripts/camunda-deployer/pkg/types"
	"strings"

	"github.com/jwalton/gchalk"
)

// printDryRunSummary outputs what would happen during deployment without executing.
func printDryRunSummary(scenarioCtx *ScenarioContext, opts types.Options, valuesFiles []string) {
	styleKey := func(s string) string { return logging.Emphasize(s, gchalk.Cyan) }
	styleVal := func(s string) string { return logging.Emphasize(s, gchalk.Magenta) }
	styleHead := func(s string) string { return logging.Emphasize(s, gchalk.Bold) }
	styleWarn := func(s string) string { return logging.Emphasize(s, gchalk.Yellow) }

	var out strings.Builder
	out.WriteString(styleWarn("DRY-RUN MODE - No changes will be made"))
	out.WriteString("\n\n")

	// Deployment details
	out.WriteString(styleHead("Deployment Configuration"))
	out.WriteString("\n")
	maxKey := 25
	fmt.Fprintf(&out, "  %s: %s\n", styleKey(fmt.Sprintf("%-*s", maxKey, "Scenario")), styleVal(scenarioCtx.ScenarioName))
	fmt.Fprintf(&out, "  %s: %s\n", styleKey(fmt.Sprintf("%-*s", maxKey, "Namespace")), styleVal(scenarioCtx.Namespace))
	fmt.Fprintf(&out, "  %s: %s\n", styleKey(fmt.Sprintf("%-*s", maxKey, "Release")), styleVal(scenarioCtx.Release))
	if scenarioCtx.IngressHostname != "" {
		fmt.Fprintf(&out, "  %s: %s\n", styleKey(fmt.Sprintf("%-*s", maxKey, "Ingress Hostname")), styleVal(scenarioCtx.IngressHostname))
	}
	fmt.Fprintf(&out, "  %s: %s\n", styleKey(fmt.Sprintf("%-*s", maxKey, "Keycloak Realm")), styleVal(scenarioCtx.KeycloakRealm))
	fmt.Fprintf(&out, "  %s: %s\n", styleKey(fmt.Sprintf("%-*s", maxKey, "Optimize Index Prefix")), styleVal(scenarioCtx.OptimizeIndexPrefix))
	fmt.Fprintf(&out, "  %s: %s\n", styleKey(fmt.Sprintf("%-*s", maxKey, "Orchestration Index Prefix")), styleVal(scenarioCtx.OrchestrationIndexPrefix))

	out.WriteString("\n")
	out.WriteString(styleHead("Helm Options"))
	out.WriteString("\n")
	if opts.ChartPath != "" {
		fmt.Fprintf(&out, "  %s: %s\n", styleKey(fmt.Sprintf("%-*s", maxKey, "Chart Path")), styleVal(opts.ChartPath))
	}
	if opts.Chart != "" {
		fmt.Fprintf(&out, "  %s: %s\n", styleKey(fmt.Sprintf("%-*s", maxKey, "Chart")), styleVal(opts.Chart))
	}
	if opts.Version != "" {
		fmt.Fprintf(&out, "  %s: %s\n", styleKey(fmt.Sprintf("%-*s", maxKey, "Version")), styleVal(opts.Version))
	}
	fmt.Fprintf(&out, "  %s: %s\n", styleKey(fmt.Sprintf("%-*s", maxKey, "Wait")), styleVal(fmt.Sprintf("%t", opts.Wait)))
	fmt.Fprintf(&out, "  %s: %s\n", styleKey(fmt.Sprintf("%-*s", maxKey, "Atomic")), styleVal(fmt.Sprintf("%t", opts.Atomic)))
	fmt.Fprintf(&out, "  %s: %s\n", styleKey(fmt.Sprintf("%-*s", maxKey, "Timeout")), styleVal(opts.Timeout.String()))

	out.WriteString("\n")
	out.WriteString(styleHead("Values Files (in order)"))
	out.WriteString("\n")
	for i, vf := range valuesFiles {
		fmt.Fprintf(&out, "  %d. %s\n", i+1, styleVal(vf))
	}

	out.WriteString("\n")
	out.WriteString(styleHead("Helm Command Preview"))
	out.WriteString("\n")
	helmCmd := buildHelmCommandPreview(opts, valuesFiles)
	fmt.Fprintf(&out, "  %s\n", styleVal(helmCmd))

	out.WriteString("\n")
	out.WriteString(styleWarn("To execute this deployment, remove the --dry-run flag"))

	logging.Logger.Info().Msg(out.String())
}

// buildHelmCommandPreview constructs a preview of the helm command that would be executed.
func buildHelmCommandPreview(opts types.Options, valuesFiles []string) string {
	var parts []string
	parts = append(parts, "helm upgrade --install")
	parts = append(parts, opts.ReleaseName)

	if opts.ChartPath != "" {
		parts = append(parts, opts.ChartPath)
	} else if opts.Chart != "" {
		parts = append(parts, opts.Chart)
		if opts.Version != "" {
			parts = append(parts, "--version", opts.Version)
		}
	}

	parts = append(parts, "--namespace", opts.Namespace)
	parts = append(parts, "--create-namespace")

	if opts.Wait {
		parts = append(parts, "--wait")
	}
	if opts.Atomic {
		parts = append(parts, "--atomic")
	}
	if opts.Timeout > 0 {
		parts = append(parts, "--timeout", opts.Timeout.String())
	}

	for _, vf := range valuesFiles {
		parts = append(parts, "-f", vf)
	}

	return strings.Join(parts, " ")
}

// printDeploymentSummary outputs the deployment results.
func printDeploymentSummary(realm, optimizePrefix, orchestrationPrefix string) {
	firstPwd := os.Getenv("DISTRO_QA_E2E_TESTS_IDENTITY_FIRSTUSER_PASSWORD")
	secondPwd := os.Getenv("DISTRO_QA_E2E_TESTS_IDENTITY_SECONDUSER_PASSWORD")
	thirdPwd := os.Getenv("DISTRO_QA_E2E_TESTS_IDENTITY_THIRDUSER_PASSWORD")
	clientSecret := os.Getenv("DISTRO_QA_E2E_TESTS_KEYCLOAK_CLIENTS_SECRET")

	if !logging.IsTerminal(os.Stdout.Fd()) {
		// Plain, machine-friendly output
		var out strings.Builder
		fmt.Fprintf(&out, "deployment: success\n")
		fmt.Fprintf(&out, "realm: %s\n", realm)
		fmt.Fprintf(&out, "optimizeIndexPrefix: %s\n", optimizePrefix)
		fmt.Fprintf(&out, "orchestrationIndexPrefix: %s\n", orchestrationPrefix)
		fmt.Fprintf(&out, "credentials:\n")
		fmt.Fprintf(&out, "  firstUserPassword: %s\n", firstPwd)
		fmt.Fprintf(&out, "  secondUserPassword: %s\n", secondPwd)
		fmt.Fprintf(&out, "  thirdUserPassword: %s\n", thirdPwd)
		fmt.Fprintf(&out, "  keycloakClientsSecret: %s\n", clientSecret)
		logging.Logger.Info().Msg(out.String())
		return
	}

	// Pretty, human-friendly output
	styleKey := func(s string) string { return logging.Emphasize(s, gchalk.Cyan) }
	styleVal := func(s string) string { return logging.Emphasize(s, gchalk.Magenta) }
	styleOk := func(s string) string { return logging.Emphasize(s, gchalk.Green) }
	styleHead := func(s string) string { return logging.Emphasize(s, gchalk.Bold) }

	var out strings.Builder
	out.WriteString(styleOk("Deployment completed successfully"))
	out.WriteString("\n\n")

	// Identifiers
	out.WriteString(styleHead("Identifiers"))
	out.WriteString("\n")
	maxKey := 25
	fmt.Fprintf(&out, "  - %s: %s\n", styleKey(fmt.Sprintf("%-*s", maxKey, "Realm")), styleVal(realm))
	fmt.Fprintf(&out, "  - %s: %s\n", styleKey(fmt.Sprintf("%-*s", maxKey, "Optimize index prefix")), styleVal(optimizePrefix))
	fmt.Fprintf(&out, "  - %s: %s\n", styleKey(fmt.Sprintf("%-*s", maxKey, "Orchestration index prefix")), styleVal(orchestrationPrefix))

	out.WriteString("\n")
	out.WriteString(styleHead("Test credentials"))
	out.WriteString("\n")
	fmt.Fprintf(&out, "  - %s: %s\n", styleKey(fmt.Sprintf("%-*s", maxKey, "First user password")), styleVal(firstPwd))
	fmt.Fprintf(&out, "  - %s: %s\n", styleKey(fmt.Sprintf("%-*s", maxKey, "Second user password")), styleVal(secondPwd))
	fmt.Fprintf(&out, "  - %s: %s\n", styleKey(fmt.Sprintf("%-*s", maxKey, "Third user password")), styleVal(thirdPwd))
	fmt.Fprintf(&out, "  - %s: %s\n", styleKey(fmt.Sprintf("%-*s", maxKey, "Keycloak clients secret")), styleVal(clientSecret))

	out.WriteString("\n")
	out.WriteString("Please keep these credentials safe. If you have any questions, refer to the documentation or reach out for support.")

	logging.Logger.Info().Msg(out.String())
}

// printMultiScenarioSummary outputs the deployment results for multiple scenarios.
func printMultiScenarioSummary(results []*ScenarioResult) {
	successCount := 0
	failureCount := 0
	for _, r := range results {
		if r.Error == nil {
			successCount++
		} else {
			failureCount++
		}
	}

	if !logging.IsTerminal(os.Stdout.Fd()) {
		// Plain, machine-friendly output
		var out strings.Builder
		fmt.Fprintf(&out, "parallel deployment: completed\n")
		fmt.Fprintf(&out, "total scenarios: %d\n", len(results))
		fmt.Fprintf(&out, "successful: %d\n", successCount)
		fmt.Fprintf(&out, "failed: %d\n", failureCount)
		fmt.Fprintf(&out, "\nscenarios:\n")
		for _, r := range results {
			fmt.Fprintf(&out, "- scenario: %s\n", r.Scenario)
			fmt.Fprintf(&out, "  namespace: %s\n", r.Namespace)
			fmt.Fprintf(&out, "  release: %s\n", r.Release)
			if r.Error != nil {
				fmt.Fprintf(&out, "  status: failed\n")
				fmt.Fprintf(&out, "  error: %v\n", r.Error)
			} else {
				fmt.Fprintf(&out, "  status: success\n")
				fmt.Fprintf(&out, "  realm: %s\n", r.KeycloakRealm)
				fmt.Fprintf(&out, "  optimizeIndexPrefix: %s\n", r.OptimizeIndexPrefix)
				fmt.Fprintf(&out, "  orchestrationIndexPrefix: %s\n", r.OrchestrationIndexPrefix)
			if r.IngressHostname != "" {
				fmt.Fprintf(&out, "  ingressHostname: %s\n", r.IngressHostname)
			}
				fmt.Fprintf(&out, "  credentials:\n")
				fmt.Fprintf(&out, "    firstUserPassword: %s\n", r.FirstUserPassword)
				fmt.Fprintf(&out, "    secondUserPassword: %s\n", r.SecondUserPassword)
				fmt.Fprintf(&out, "    thirdUserPassword: %s\n", r.ThirdUserPassword)
				fmt.Fprintf(&out, "    keycloakClientsSecret: %s\n", r.KeycloakClientsSecret)
			}
		}
		logging.Logger.Info().Msg(out.String())
		return
	}

	// Pretty, human-friendly output
	styleKey := func(s string) string { return logging.Emphasize(s, gchalk.Cyan) }
	styleVal := func(s string) string { return logging.Emphasize(s, gchalk.Magenta) }
	styleOk := func(s string) string { return logging.Emphasize(s, gchalk.Green) }
	styleErr := func(s string) string { return logging.Emphasize(s, gchalk.Red) }
	styleHead := func(s string) string { return logging.Emphasize(s, gchalk.Bold) }
	styleWarn := func(s string) string { return logging.Emphasize(s, gchalk.Yellow) }

	var out strings.Builder
	if failureCount == 0 {
		out.WriteString(styleOk("All scenarios deployed successfully!"))
	} else if successCount == 0 {
		out.WriteString(styleErr("All scenarios failed to deploy"))
	} else {
		out.WriteString(styleWarn(fmt.Sprintf("Partial success: %d/%d scenarios deployed", successCount, len(results))))
	}
	out.WriteString("\n\n")

	// Summary
	out.WriteString(styleHead("Deployment Summary"))
	out.WriteString("\n")
	fmt.Fprintf(&out, "  Total scenarios: %s\n", styleVal(fmt.Sprintf("%d", len(results))))
	fmt.Fprintf(&out, "  Successful: %s\n", styleOk(fmt.Sprintf("%d", successCount)))
	if failureCount > 0 {
		fmt.Fprintf(&out, "  Failed: %s\n", styleErr(fmt.Sprintf("%d", failureCount)))
	}
	out.WriteString("\n")

	// Details per scenario
	maxKey := 30
	for i, r := range results {
		if i > 0 {
			out.WriteString("\n")
		}

		if r.Error != nil {
			out.WriteString(styleErr(fmt.Sprintf("Scenario %d: %s [FAILED]", i+1, r.Scenario)))
			out.WriteString("\n")
			fmt.Fprintf(&out, "  %s: %s\n", styleKey(fmt.Sprintf("%-*s", maxKey, "Namespace")), styleVal(r.Namespace))
			fmt.Fprintf(&out, "  %s: %s\n", styleKey(fmt.Sprintf("%-*s", maxKey, "Error")), styleErr(r.Error.Error()))
		} else {
			out.WriteString(styleOk(fmt.Sprintf("Scenario %d: %s [SUCCESS]", i+1, r.Scenario)))
			out.WriteString("\n")
			fmt.Fprintf(&out, "  %s: %s\n", styleKey(fmt.Sprintf("%-*s", maxKey, "Namespace")), styleVal(r.Namespace))
			fmt.Fprintf(&out, "  %s: %s\n", styleKey(fmt.Sprintf("%-*s", maxKey, "Release")), styleVal(r.Release))
		if r.IngressHostname != "" {
			fmt.Fprintf(&out, "  %s: %s\n", styleKey(fmt.Sprintf("%-*s", maxKey, "Ingress Hostname")), styleVal(r.IngressHostname))
		}
			fmt.Fprintf(&out, "  %s: %s\n", styleKey(fmt.Sprintf("%-*s", maxKey, "Keycloak Realm")), styleVal(r.KeycloakRealm))
			fmt.Fprintf(&out, "  %s: %s\n", styleKey(fmt.Sprintf("%-*s", maxKey, "Optimize Index Prefix")), styleVal(r.OptimizeIndexPrefix))
			fmt.Fprintf(&out, "  %s: %s\n", styleKey(fmt.Sprintf("%-*s", maxKey, "Orchestration Index Prefix")), styleVal(r.OrchestrationIndexPrefix))
			out.WriteString(styleHead("  Credentials:"))
			out.WriteString("\n")
			fmt.Fprintf(&out, "    %s: %s\n", styleKey(fmt.Sprintf("%-*s", maxKey-2, "First user password")), styleVal(r.FirstUserPassword))
			fmt.Fprintf(&out, "    %s: %s\n", styleKey(fmt.Sprintf("%-*s", maxKey-2, "Second user password")), styleVal(r.SecondUserPassword))
			fmt.Fprintf(&out, "    %s: %s\n", styleKey(fmt.Sprintf("%-*s", maxKey-2, "Third user password")), styleVal(r.ThirdUserPassword))
			fmt.Fprintf(&out, "    %s: %s\n", styleKey(fmt.Sprintf("%-*s", maxKey-2, "Keycloak clients secret")), styleVal(r.KeycloakClientsSecret))
		}
	}

	out.WriteString("\n")
	if failureCount == 0 {
		out.WriteString("Please keep these credentials safe. All deployments are ready to use!")
	}

	logging.Logger.Info().Msg(out.String())
}

