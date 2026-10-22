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
	"os"
	"scripts/deploy-camunda/acceptance"

	"github.com/spf13/cobra"
)

func newAcceptanceCommand() *cobra.Command {
	parent := &cobra.Command{Use: "acceptance", Short: "Run repository acceptance checks"}
	var opts acceptance.PhysicalTenantAcceptanceOptions
	cmd := &cobra.Command{
		Use:   "physical-tenant-exporters",
		Short: "Validate Physical Tenant exporters and Optimize isolation",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return acceptance.RunPhysicalTenant(cmd.Context(), opts)
		},
	}
	f := cmd.Flags()
	f.StringVar(&opts.Namespace, "namespace", os.Getenv("TEST_NAMESPACE"), "Camunda namespace")
	f.StringVar(&opts.Release, "release", envDefault("RELEASE_NAME", "integration"), "Camunda release")
	f.StringVar(&opts.HubNamespace, "hub-namespace", os.Getenv("HUB_NAMESPACE"), "Hub namespace")
	f.StringVar(&opts.HubHost, "hub-host", os.Getenv("HUB_HOST"), "Hub hostname")
	f.StringVar(&opts.OrchHost, "orchestration-host", os.Getenv("ORCH_HOST"), "Orchestration hostname")
	f.StringVar(&opts.KubeContext, "kube-context", os.Getenv("KUBE_CONTEXT"), "kubectl context")
	paths := map[string]*string{}
	for _, tenant := range []string{"default", "tenanta", "tenantb"} {
		paths[tenant] = new(string)
	}
	f.StringVar(paths["default"], "default-optimize-path", os.Getenv("OPTDEFAULT_OPTIMIZE_CONTEXT_PATH"), "Default Optimize context path")
	f.StringVar(paths["tenanta"], "tenanta-optimize-path", os.Getenv("OPTTA_OPTIMIZE_CONTEXT_PATH"), "Tenant A Optimize context path")
	f.StringVar(paths["tenantb"], "tenantb-optimize-path", os.Getenv("OPTTB_OPTIMIZE_CONTEXT_PATH"), "Tenant B Optimize context path")
	cmd.PreRunE = func(*cobra.Command, []string) error {
		opts.OptimizePaths = map[string]string{}
		for tenant, value := range paths {
			opts.OptimizePaths[tenant] = *value
		}
		return acceptance.ValidatePhysicalTenantOptions(opts)
	}
	parent.AddCommand(cmd, newDocumentStoreAcceptanceCommand())
	return parent
}

func newDocumentStoreAcceptanceCommand() *cobra.Command {
	var opts acceptance.DocumentStoreAcceptanceOptions
	command := &cobra.Command{
		Use:   "documentstore-s3-compatible",
		Short: "Verify document operations against the S3-compatible CI fixture",
		Args:  cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			return acceptance.RunDocumentStore(command.Context(), opts)
		},
	}
	command.Flags().StringVar(&opts.Namespace, "namespace", os.Getenv("TEST_NAMESPACE"), "Camunda namespace")
	command.Flags().StringVar(&opts.Release, "release", envDefault("RELEASE_NAME", "integration"), "Camunda release")
	command.Flags().StringVar(&opts.KubeContext, "kube-context", os.Getenv("KUBE_CONTEXT"), "kubectl context")
	return command
}

func envDefault(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}
