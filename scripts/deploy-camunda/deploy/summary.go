package deploy

import (
	"fmt"
	"os"
	"strings"

	"scripts/camunda-core/pkg/logging"
	"scripts/camunda-deployer/pkg/types"
	"scripts/deploy-camunda/config"

	"github.com/jwalton/gchalk"
)

// Style helpers for terminal output.
var (
	styleKey  = func(s string) string { return logging.Emphasize(s, gchalk.Cyan) }
	styleVal  = func(s string) string { return logging.Emphasize(s, gchalk.Magenta) }
	styleOk   = func(s string) string { return logging.Emphasize(s, gchalk.Green) }
	styleErr  = func(s string) string { return logging.Emphasize(s, gchalk.Red) }
	styleHead = func(s string) string { return logging.Emphasize(s, gchalk.Bold) }
	styleWarn = func(s string) string { return logging.Emphasize(s, gchalk.Yellow) }
)

// redactDeployOpts returns a copy of deploy options with sensitive fields redacted for logging.
func redactDeployOpts(opts types.Options) map[string]interface{} {
	redacted := "[REDACTED]"
	return map[string]interface{}{
		"chart":                  opts.Chart,
		"chartPath":              opts.ChartPath,
		"version":                opts.Version,
		"releaseName":            opts.ReleaseName,
		"namespace":              opts.Namespace,
		"kubeContext":            opts.KubeContext,
		"timeout":                opts.Timeout.String(),
		"wait":                   opts.Wait,
		"atomic":                 opts.Atomic,
		"ingressHost":            opts.IngressHost,
		"valuesFiles":            opts.ValuesFiles,
		"identifier":             opts.Identifier,
		"ttl":                    opts.TTL,
		"ensureDockerRegistry":   opts.EnsureDockerRegistry,
		"dockerRegistryUsername": opts.DockerRegistryUsername,
		"dockerRegistryPassword": func() string {
			if opts.DockerRegistryPassword != "" {
				return redacted
			}
			return ""
		}(),
		"ensureDockerHub":   opts.EnsureDockerHub,
		"dockerHubUsername": opts.DockerHubUsername,
		"dockerHubPassword": func() string {
			if opts.DockerHubPassword != "" {
				return redacted
			}
			return ""
		}(),
		"skipDockerLogin":        opts.SkipDockerLogin,
		"skipDependencyUpdate":   opts.SkipDependencyUpdate,
		"applyIntegrationCreds":  opts.ApplyIntegrationCreds,
		"externalSecretsEnabled": opts.ExternalSecretsEnabled,
		"platform":               opts.Platform,
		"repoRoot":               opts.RepoRoot,
		"loadKeycloakRealm":      opts.LoadKeycloakRealm,
		"keycloakRealmName":      opts.KeycloakRealmName,
		"vaultSecretPath":        opts.VaultSecretPath,
		"renderTemplates":        opts.RenderTemplates,
		"renderOutputDir":        opts.RenderOutputDir,
		"includeCRDs":            opts.IncludeCRDs,
		"ciMetadata":             opts.CIMetadata,
	}
}

// printDeploymentSummary outputs the deployment results.
func printDeploymentSummary(result *ScenarioResult, flags *config.RuntimeFlags) {
	realm := result.KeycloakRealm
	optimizePrefix := result.OptimizeIndexPrefix
	orchestrationPrefix := result.OrchestrationIndexPrefix
	namespace := result.Namespace
	release := result.Release
	testEnvFile := result.TestEnvFile
	layeredFiles := result.LayeredFiles
	firstPwd := result.FirstUserPassword
	secondPwd := result.SecondUserPassword
	thirdPwd := result.ThirdUserPassword
	clientSecret := result.KeycloakClientsSecret

	if !logging.IsTerminal(os.Stdout.Fd()) {
		// Plain, machine-friendly output
		var out strings.Builder
		fmt.Fprintf(&out, "deployment: success\n")
		fmt.Fprintf(&out, "namespace: %s\n", namespace)
		fmt.Fprintf(&out, "realm: %s\n", realm)
		fmt.Fprintf(&out, "optimizeIndexPrefix: %s\n", optimizePrefix)
		fmt.Fprintf(&out, "orchestrationIndexPrefix: %s\n", orchestrationPrefix)
		if len(layeredFiles) > 0 {
			fmt.Fprintf(&out, "layeredFiles:\n")
			for _, f := range layeredFiles {
				fmt.Fprintf(&out, "  - %s\n", f)
			}
		}
		fmt.Fprintf(&out, "credentials:\n")
		fmt.Fprintf(&out, "  firstUserPassword: %s\n", firstPwd)
		fmt.Fprintf(&out, "  secondUserPassword: %s\n", secondPwd)
		fmt.Fprintf(&out, "  thirdUserPassword: %s\n", thirdPwd)
		fmt.Fprintf(&out, "  keycloakClientsSecret: %s\n", clientSecret)
		// Add test env file path if generated
		if testEnvFile != "" {
			fmt.Fprintf(&out, "testEnvFile: %s\n", testEnvFile)
		}
		// Add debug port-forward instructions
		if len(flags.Debug.DebugComponents) > 0 {
			fmt.Fprintf(&out, "debug:\n")
			for component, debugCfg := range flags.Debug.DebugComponents {
				if component == "orchestration" {
					fmt.Fprintf(&out, "  orchestration:\n")
					fmt.Fprintf(&out, "    port: %d\n", debugCfg.Port)
					fmt.Fprintf(&out, "    portForwardCommand: kubectl port-forward -n %s svc/%s-zeebe %d:%d\n", namespace, release, debugCfg.Port, debugCfg.Port)
				} else if component == "connectors" {
					fmt.Fprintf(&out, "  connectors:\n")
					fmt.Fprintf(&out, "    port: %d\n", debugCfg.Port)
					fmt.Fprintf(&out, "    portForwardCommand: kubectl port-forward -n %s deploy/%s-connectors %d:%d\n", namespace, release, debugCfg.Port, debugCfg.Port)
				}
			}
		}
		logging.Logger.Info().Msg(out.String())
		return
	}

	// Pretty, human-friendly output
	var out strings.Builder
	out.WriteString(styleOk("🎉 Deployment completed successfully"))
	out.WriteString("\n\n")

	// Identifiers
	out.WriteString(styleHead("Identifiers"))
	out.WriteString("\n")
	maxKey := 25
	fmt.Fprintf(&out, "  - %s: %s\n", styleKey(fmt.Sprintf("%-*s", maxKey, "Namespace")), styleVal(namespace))
	fmt.Fprintf(&out, "  - %s: %s\n", styleKey(fmt.Sprintf("%-*s", maxKey, "Realm")), styleVal(realm))
	fmt.Fprintf(&out, "  - %s: %s\n", styleKey(fmt.Sprintf("%-*s", maxKey, "Optimize index prefix")), styleVal(optimizePrefix))
	fmt.Fprintf(&out, "  - %s: %s\n", styleKey(fmt.Sprintf("%-*s", maxKey, "Orchestration index prefix")), styleVal(orchestrationPrefix))

	// Layered values files
	if len(layeredFiles) > 0 {
		out.WriteString("\n")
		out.WriteString(styleHead("Layered values files"))
		out.WriteString("\n")
		for i, f := range layeredFiles {
			fmt.Fprintf(&out, "  %s %s\n", styleKey(fmt.Sprintf("%d.", i+1)), styleVal(f))
		}
	}

	out.WriteString("\n")
	out.WriteString(styleHead("Test credentials"))
	out.WriteString("\n")
	fmt.Fprintf(&out, "  - %s: %s\n", styleKey(fmt.Sprintf("%-*s", maxKey, "First user password")), styleVal(firstPwd))
	fmt.Fprintf(&out, "  - %s: %s\n", styleKey(fmt.Sprintf("%-*s", maxKey, "Second user password")), styleVal(secondPwd))
	fmt.Fprintf(&out, "  - %s: %s\n", styleKey(fmt.Sprintf("%-*s", maxKey, "Third user password")), styleVal(thirdPwd))
	fmt.Fprintf(&out, "  - %s: %s\n", styleKey(fmt.Sprintf("%-*s", maxKey, "Keycloak clients secret")), styleVal(clientSecret))

	// Add test env file path if generated
	if testEnvFile != "" {
		out.WriteString("\n")
		out.WriteString(styleHead("Test Environment"))
		out.WriteString("\n")
		fmt.Fprintf(&out, "  - %s: %s\n", styleKey(fmt.Sprintf("%-*s", maxKey, "E2E env file")), styleVal(testEnvFile))
	}

	// Add debug port-forward instructions if debug mode is enabled
	if len(flags.Debug.DebugComponents) > 0 {
		out.WriteString("\n")
		out.WriteString(styleHead("Debug mode"))
		out.WriteString("\n")
		for component, debugCfg := range flags.Debug.DebugComponents {
			fmt.Fprintf(&out, "  - %s: %s\n", styleKey(fmt.Sprintf("%-*s", maxKey, "Component")), styleVal(component))
			fmt.Fprintf(&out, "  - %s: %s\n", styleKey(fmt.Sprintf("%-*s", maxKey, "Debug port")), styleVal(fmt.Sprintf("%d", debugCfg.Port)))
			fmt.Fprintf(&out, "  - %s: %s\n", styleKey(fmt.Sprintf("%-*s", maxKey, "Suspend on start")), styleVal(fmt.Sprintf("%t", flags.Debug.DebugSuspend)))
			out.WriteString("\n")
			out.WriteString(styleHead("  To connect your debugger, run:"))
			out.WriteString("\n")
			var portForwardCmd string
			if component == "orchestration" {
				portForwardCmd = fmt.Sprintf("kubectl port-forward -n %s svc/%s-zeebe %d:%d", namespace, release, debugCfg.Port, debugCfg.Port)
			} else if component == "connectors" {
				portForwardCmd = fmt.Sprintf("kubectl port-forward -n %s deploy/%s-connectors %d:%d", namespace, release, debugCfg.Port, debugCfg.Port)
			}
			fmt.Fprintf(&out, "    %s\n", styleVal(portForwardCmd))
			out.WriteString("\n")
			out.WriteString(fmt.Sprintf("  Then connect your IDE debugger to %s\n", styleVal(fmt.Sprintf("localhost:%d", debugCfg.Port))))
		}
	}

	out.WriteString("\n")
	out.WriteString("Please keep these credentials safe. If you have any questions, refer to the documentation or reach out for support. 🚀")

	logging.Logger.Info().Msg(out.String())
}

// printMultiScenarioSummary outputs the deployment results for multiple scenarios.
func printMultiScenarioSummary(results []*ScenarioResult, flags *config.RuntimeFlags) {
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
				if r.IngressHost != "" {
					fmt.Fprintf(&out, "  ingressHost: %s\n", r.IngressHost)
				}
				if len(r.LayeredFiles) > 0 {
					fmt.Fprintf(&out, "  layeredFiles:\n")
					for _, f := range r.LayeredFiles {
						fmt.Fprintf(&out, "    - %s\n", f)
					}
				}
				if r.TestEnvFile != "" {
					fmt.Fprintf(&out, "  testEnvFile: %s\n", r.TestEnvFile)
				}
				fmt.Fprintf(&out, "  credentials:\n")
				fmt.Fprintf(&out, "    firstUserPassword: %s\n", r.FirstUserPassword)
				fmt.Fprintf(&out, "    secondUserPassword: %s\n", r.SecondUserPassword)
				fmt.Fprintf(&out, "    thirdUserPassword: %s\n", r.ThirdUserPassword)
				fmt.Fprintf(&out, "    keycloakClientsSecret: %s\n", r.KeycloakClientsSecret)
			}
		}
		// Add debug port-forward instructions for machine-friendly output
		if len(flags.Debug.DebugComponents) > 0 && successCount > 0 {
			fmt.Fprintf(&out, "debug:\n")
			for component, debugCfg := range flags.Debug.DebugComponents {
				fmt.Fprintf(&out, "  - component: %s\n", component)
				fmt.Fprintf(&out, "    port: %d\n", debugCfg.Port)
				fmt.Fprintf(&out, "    portForwardCommands:\n")
				for _, r := range results {
					if r.Error == nil {
						fmt.Fprintf(&out, "      - scenario: %s\n", r.Scenario)
						if component == "orchestration" {
							fmt.Fprintf(&out, "        command: kubectl port-forward -n %s svc/%s-zeebe %d:%d\n", r.Namespace, r.Release, debugCfg.Port, debugCfg.Port)
						} else if component == "connectors" {
							fmt.Fprintf(&out, "        command: kubectl port-forward -n %s deploy/%s-connectors %d:%d\n", r.Namespace, r.Release, debugCfg.Port, debugCfg.Port)
						}
					}
				}
			}
		}
		logging.Logger.Info().Msg(out.String())
		return
	}

	// Pretty, human-friendly output
	var out strings.Builder
	if failureCount == 0 {
		out.WriteString(styleOk("🎉 All scenarios deployed successfully!"))
	} else if successCount == 0 {
		out.WriteString(styleErr("❌ All scenarios failed to deploy"))
	} else {
		out.WriteString(styleWarn(fmt.Sprintf("⚠️  Partial success: %d/%d scenarios deployed", successCount, len(results))))
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
			if r.IngressHost != "" {
				fmt.Fprintf(&out, "  %s: %s\n", styleKey(fmt.Sprintf("%-*s", maxKey, "Ingress Host")), styleVal(r.IngressHost))
			}
			fmt.Fprintf(&out, "  %s: %s\n", styleKey(fmt.Sprintf("%-*s", maxKey, "Keycloak Realm")), styleVal(r.KeycloakRealm))
			fmt.Fprintf(&out, "  %s: %s\n", styleKey(fmt.Sprintf("%-*s", maxKey, "Optimize Index Prefix")), styleVal(r.OptimizeIndexPrefix))
			fmt.Fprintf(&out, "  %s: %s\n", styleKey(fmt.Sprintf("%-*s", maxKey, "Orchestration Index Prefix")), styleVal(r.OrchestrationIndexPrefix))
			if len(r.LayeredFiles) > 0 {
				out.WriteString(styleHead("  Layered values files:"))
				out.WriteString("\n")
				for j, f := range r.LayeredFiles {
					fmt.Fprintf(&out, "    %s %s\n", styleKey(fmt.Sprintf("%d.", j+1)), styleVal(f))
				}
			}
			if r.TestEnvFile != "" {
				fmt.Fprintf(&out, "  %s: %s\n", styleKey(fmt.Sprintf("%-*s", maxKey, "E2E env file")), styleVal(r.TestEnvFile))
			}
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
		out.WriteString("Please keep these credentials safe. All deployments are ready to use! 🚀")
	}

	// Add debug port-forward instructions if debug mode is enabled
	if len(flags.Debug.DebugComponents) > 0 && successCount > 0 {
		out.WriteString("\n\n")
		out.WriteString(styleHead("Debug mode"))
		out.WriteString("\n")
		for component, debugCfg := range flags.Debug.DebugComponents {
			fmt.Fprintf(&out, "  - %s: %s\n", styleKey(fmt.Sprintf("%-*s", maxKey, "Component")), styleVal(component))
			fmt.Fprintf(&out, "  - %s: %s\n", styleKey(fmt.Sprintf("%-*s", maxKey, "Debug port")), styleVal(fmt.Sprintf("%d", debugCfg.Port)))
			fmt.Fprintf(&out, "  - %s: %s\n", styleKey(fmt.Sprintf("%-*s", maxKey, "Suspend on start")), styleVal(fmt.Sprintf("%t", flags.Debug.DebugSuspend)))
			out.WriteString("\n")
			out.WriteString(styleHead("  To connect your debugger, run one of the following:"))
			out.WriteString("\n")
			for _, r := range results {
				if r.Error == nil {
					var portForwardCmd string
					if component == "orchestration" {
						portForwardCmd = fmt.Sprintf("kubectl port-forward -n %s svc/%s-zeebe %d:%d", r.Namespace, r.Release, debugCfg.Port, debugCfg.Port)
					} else if component == "connectors" {
						portForwardCmd = fmt.Sprintf("kubectl port-forward -n %s deploy/%s-connectors %d:%d", r.Namespace, r.Release, debugCfg.Port, debugCfg.Port)
					}
					fmt.Fprintf(&out, "    [%s] %s\n", styleVal(r.Scenario), portForwardCmd)
				}
			}
			out.WriteString("\n")
			out.WriteString(fmt.Sprintf("  Then connect your IDE debugger to %s\n", styleVal(fmt.Sprintf("localhost:%d", debugCfg.Port))))
		}
	}

	logging.Logger.Info().Msg(out.String())
}
