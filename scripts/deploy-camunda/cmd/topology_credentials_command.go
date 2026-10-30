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
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"scripts/deploy-camunda/credentials"
	"scripts/deploy-camunda/deploy"
	"scripts/deploy-camunda/matrix"
)

func newTopologyEnsureCredentialsCommand() *cobra.Command {
	var opts credentials.EnsureOptions

	cmd := &cobra.Command{
		Use:   "ensure-credentials",
		Short: "Create the source Secret a credentials manifest reads from, generating any missing value",
		Long: `Reads an ExternalSecret manifest (a topology's credentials-manifest), and makes
sure the one source Secret it reads from exists in the ClusterSecretStore's
namespace with a random value for every property it references.

Existing values are never changed, and no value is ever printed. A deployed
environment picks up a generated value through reconcile-credentials.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return credentials.Ensure(cmd.Context(), cmd.OutOrStdout(), opts)
		},
	}

	cmd.Flags().StringVar(&opts.ManifestPath, "manifest", "", "ExternalSecret manifest whose source secret to ensure")
	cmd.Flags().StringVar(&opts.Base, "base", "", "topology base namespace, substituted for "+matrix.CredentialsManifestBaseToken+" in the manifest")
	cmd.Flags().StringVar(&opts.SecretNamespace, "secret-namespace", "distribution-team", "namespace the ClusterSecretStore reads source secrets from")
	cmd.Flags().StringVar(&opts.KubeContext, "kube-context", "", "kubectl context (defaults to current)")
	cmd.Flags().StringSlice("guard-namespace", nil, "ignored")
	_ = cmd.Flags().MarkDeprecated("guard-namespace", "it has no effect; reconcile-credentials now brings existing stores to the source")
	_ = cmd.MarkFlagRequired("manifest")
	return cmd
}

type credentialsCommandFlags struct {
	topologyLifecycleFlags
	secretNamespace string
}

func (f *credentialsCommandFlags) bind(cmd *cobra.Command) {
	f.topologyLifecycleFlags.bind(cmd)
	cmd.Flags().StringVar(&f.secretNamespace, "secret-namespace", "distribution-team", "namespace the ClusterSecretStore reads source secrets from")
}

func (f *credentialsCommandFlags) load() (*matrix.Topology, error) {
	topology, err := loadScenarioTopology(f.repoRoot, f.version, f.scenario)
	if err != nil {
		return nil, err
	}
	if topology.CredentialsManifest == "" {
		return nil, fmt.Errorf("scenario %q declares no credentials-manifest", f.scenario)
	}
	return topology, nil
}

func newTopologyReconcileCredentialsCommand() *cobra.Command {
	var (
		f            credentialsCommandFlags
		timeout      time.Duration
		checksumFile string
	)
	cmd := &cobra.Command{
		Use:   "reconcile-credentials",
		Short: "Set a deployed topology's credential stores to the credentials-manifest's current values",
		Long: `Applies the topology's credentials-manifest to every existing release namespace
and waits for its ExternalSecrets to sync from the source Secret. Then, in the
namespace named by the topology's credential-stores, it sets every store that
keeps the password it was initialised with to the synced value:

- the Keycloak master-realm admin: logs in with the synced value, else with the
  namespace's previous value, else creates a temporary admin with
  kc.sh bootstrap-admin in a separate pod and deletes it afterwards;
- each listed Keycloak realm user that exists;
- the Elasticsearch native user, through the security API with its previous
  value;
- each PostgreSQL role, through ALTER ROLE over the pod's local socket.

Stores already current are left alone and no value is ever printed. Run it
before the deploy, and annotate the --checksum-file value onto the consumers
so the deploy restarts them.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			topology, err := f.load()
			if err != nil {
				return err
			}
			opts := credentials.ReconcileOptions{
				RepoRoot:        f.repoRoot,
				Scenario:        f.scenario,
				Topology:        topology,
				Base:            f.base,
				SecretNamespace: f.secretNamespace,
				KubeContext:     f.kubeContext,
				Timeout:         timeout,
				ChecksumFile:    checksumFile,
			}
			if topology.CredentialStores != nil {
				releases, err := f.releases()
				if err != nil {
					return err
				}
				for _, rel := range releases {
					opts.Namespaces = append(opts.Namespaces, rel.Namespace)
				}
				if opts.HubNamespace, err = deploy.DeriveReleaseNamespace(f.base, topology.CredentialStores.NamespaceSuffix); err != nil {
					return err
				}
			}
			return credentials.Reconcile(cmd.Context(), cmd.OutOrStdout(), opts)
		},
	}
	f.bind(cmd)
	cmd.Flags().DurationVar(&timeout, "timeout", 5*time.Minute, "how long to wait for an ExternalSecret sync or the bootstrap-admin pod")
	cmd.Flags().StringVar(&checksumFile, "checksum-file", "", "after a successful run, write the credentials checksum of the source values it reconciled against to this file")
	return cmd
}
