// Copyright 2026 Camunda Services GmbH
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package cmd

import (
	"scripts/deploy-camunda/acceptance"
	"scripts/deploy-camunda/deploy"

	"github.com/spf13/cobra"
)

// newE2EEnvCommand groups e2e .env generation subcommands.
func newE2EEnvCommand() *cobra.Command {
	c := &cobra.Command{
		Use:   "e2e-env",
		Short: "Generate e2e .env files",
	}
	c.AddCommand(newE2EEnvMergeCommand())
	c.AddCommand(newE2EEnvAPISuiteCommand())
	return c
}

// newE2EEnvMergeCommand produces a single merged .env for a multi-namespace
// topology deploy: endpoints come from the orchestration namespace (via
// render-e2e-env.sh), while the auth/host vars and credentials are overridden
// to the Hub namespace where Identity/Keycloak run.
func newE2EEnvMergeCommand() *cobra.Command {
	opts := deploy.E2EEnvMergeOptions{VenomClientSecretKey: acceptance.VenomClientSecretKey}

	cmd := &cobra.Command{
		Use:   "merge",
		Short: "Merge orchestration endpoints with Hub auth into one .env",
		RunE: func(cmd *cobra.Command, args []string) error {
			return deploy.MergeE2EEnv(opts)
		},
	}

	cmd.Flags().StringVar(&opts.OrchestrationNamespace, "orchestration-namespace", "", "orchestration release namespace")
	cmd.Flags().StringVar(&opts.HubNamespace, "hub-namespace", "", "Hub release namespace")
	cmd.Flags().StringVar(&opts.OptimizeNamespace, "optimize-namespace", "", "namespace of the Optimize-only release serving this leg; render-e2e-env.sh derives Optimize from the orchestration namespace, which is not where Optimize runs in a topology using the optimize release role")
	cmd.Flags().StringVar(&opts.OptimizeContextPath, "optimize-context-path", "", "ingress path the Optimize-only release is served on, e.g. /optimize-orcha")
	cmd.Flags().StringVar(&opts.ChartPath, "absolute-chart-path", "", "absolute chart path")
	cmd.Flags().StringVar(&opts.Output, "output", ".env", "output .env path")
	cmd.Flags().StringVar(&opts.RenderScript, "render-script", "scripts/render-e2e-env.sh", "path to render-e2e-env.sh")
	cmd.Flags().StringVar(&opts.KubeContext, "kube-context", "", "kube context (optional)")
	cmd.Flags().BoolVar(&opts.CI, "ci", false, "set CI=true in the merged .env (matches render-e2e-env.sh's default; pass when running in an actual CI job)")
	cmd.Flags().BoolVar(&opts.RunSmokeTests, "run-smoke-tests", true, "pass --run-smoke-tests to render-e2e-env.sh (sets IS_SMOKE=true)")
	_ = cmd.MarkFlagRequired("orchestration-namespace")
	_ = cmd.MarkFlagRequired("hub-namespace")
	_ = cmd.MarkFlagRequired("absolute-chart-path")

	return cmd
}

func newE2EEnvAPISuiteCommand() *cobra.Command {
	var opts deploy.APISuiteEnvOptions

	cmd := &cobra.Command{
		Use:   "api-suite",
		Short: "Write the REST v2 API suite's variables for a deployment, for bash to source",
		RunE: func(cmd *cobra.Command, args []string) error {
			return deploy.WriteAPISuiteEnv(cmd.OutOrStdout(), opts)
		},
	}

	cmd.Flags().StringVar(&opts.EnvFile, "env-file", "", "the deployment's rendered e2e .env")
	cmd.Flags().StringVar(&opts.Namespace, "namespace", "", "namespace of the deployment")
	cmd.Flags().StringVar(&opts.KubeContext, "kube-context", "", "kube context (optional)")
	cmd.Flags().StringVar(&opts.Auth, "auth", "", `the scenario's auth type; "basic" runs the suite with basic auth`)
	cmd.Flags().BoolVar(&opts.MultiTenancy, "mt", false, "the deployment has multi-tenancy enabled")
	cmd.Flags().StringVar(&opts.Output, "output", "", "file to write the variables to")
	cmd.Flags().BoolVar(&opts.CI, "ci", false, "print ::add-mask:: lines for the secrets")
	_ = cmd.MarkFlagRequired("env-file")
	_ = cmd.MarkFlagRequired("namespace")
	_ = cmd.MarkFlagRequired("output")
	return cmd
}
