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
	"bytes"
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"math/big"
	"os"
	"sort"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"scripts/camunda-core/pkg/kube"
)

const (
	credentialAlphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789"
	credentialLength   = 40
)

type externalSecretDoc struct {
	Kind string `yaml:"kind"`
	Spec struct {
		Data []struct {
			RemoteRef struct {
				Key      string `yaml:"key"`
				Property string `yaml:"property"`
			} `yaml:"remoteRef"`
		} `yaml:"data"`
	} `yaml:"spec"`
}

// credentialSource is the single source Secret an ExternalSecret manifest reads
// from, and every property it reads.
type credentialSource struct {
	Name       string
	Properties []string
}

// parseCredentialSource reads every ExternalSecret document in manifest and
// requires them to read from exactly one source Secret, so ensure-credentials
// has a single object to own. Properties are de-duplicated and sorted.
func parseCredentialSource(manifest []byte) (credentialSource, error) {
	dec := yaml.NewDecoder(bytes.NewReader(manifest))
	names := map[string]bool{}
	props := map[string]bool{}
	for {
		var doc externalSecretDoc
		err := dec.Decode(&doc)
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return credentialSource{}, fmt.Errorf("decode manifest: %w", err)
		}
		if doc.Kind != "ExternalSecret" {
			continue
		}
		for _, d := range doc.Spec.Data {
			if d.RemoteRef.Key == "" || d.RemoteRef.Property == "" {
				return credentialSource{}, errors.New("every spec.data entry needs remoteRef.key and remoteRef.property")
			}
			names[d.RemoteRef.Key] = true
			props[d.RemoteRef.Property] = true
		}
	}
	if len(names) != 1 {
		found := make([]string, 0, len(names))
		for n := range names {
			found = append(found, n)
		}
		sort.Strings(found)
		return credentialSource{}, fmt.Errorf("manifest must read from exactly one source secret, found %d: %v", len(names), found)
	}
	src := credentialSource{}
	for n := range names {
		src.Name = n
	}
	for p := range props {
		src.Properties = append(src.Properties, p)
	}
	sort.Strings(src.Properties)
	return src, nil
}

// planCredentials returns the full data set to store: every existing non-empty
// value unchanged, plus a generated value for each required property that is
// missing. It never replaces a value: rotating a credential is a deliberate,
// out-of-band operation because databases and Keycloak only read theirs once.
func planCredentials(existing map[string]string, required []string, generate func() (string, error)) (map[string]string, []string, error) {
	merged := make(map[string]string, len(existing)+len(required))
	for k, v := range existing {
		merged[k] = v
	}
	var created []string
	for _, key := range required {
		if merged[key] != "" {
			continue
		}
		v, err := generate()
		if err != nil {
			return nil, nil, err
		}
		merged[key] = v
		created = append(created, key)
	}
	return merged, created, nil
}

func generateCredential() (string, error) {
	out := make([]byte, credentialLength)
	max := big.NewInt(int64(len(credentialAlphabet)))
	for i := range out {
		n, err := rand.Int(rand.Reader, max)
		if err != nil {
			return "", fmt.Errorf("generate credential: %w", err)
		}
		out[i] = credentialAlphabet[n.Int64()]
	}
	return string(out), nil
}

func ensureCredentials(
	ctx context.Context,
	out io.Writer,
	manifest []byte,
	namespace string,
	get func(context.Context, string, string) (map[string]string, error),
	put func(context.Context, string, string, map[string]string) error,
) error {
	src, err := parseCredentialSource(manifest)
	if err != nil {
		return err
	}
	existing, err := get(ctx, namespace, src.Name)
	if err != nil {
		return err
	}
	merged, created, err := planCredentials(existing, src.Properties, generateCredential)
	if err != nil {
		return err
	}
	if len(created) > 0 {
		if err := put(ctx, namespace, src.Name, merged); err != nil {
			return err
		}
	}
	fmt.Fprintf(out, "%s/%s: %d generated, %d kept\n", namespace, src.Name, len(created), len(src.Properties)-len(created))
	for _, k := range created {
		fmt.Fprintf(out, "  generated %s\n", k)
	}
	return nil
}

func newTopologyEnsureCredentialsCommand() *cobra.Command {
	var (
		manifestPath string
		namespace    string
		kubeContext  string
	)

	cmd := &cobra.Command{
		Use:   "ensure-credentials",
		Short: "Create the source Secret a credentials manifest reads from, generating any missing value",
		Long: `Reads an ExternalSecret manifest (a topology's credentials-manifest), and makes
sure the one source Secret it reads from exists in the ClusterSecretStore's
namespace with a random value for every property it references.

Existing values are never changed, and no value is ever printed.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			manifest, err := os.ReadFile(manifestPath)
			if err != nil {
				return fmt.Errorf("read manifest: %w", err)
			}
			client, err := kube.NewClient("", kubeContext)
			if err != nil {
				return err
			}
			return ensureCredentials(cmd.Context(), cmd.OutOrStdout(), manifest, namespace, client.GetSecretData, client.EnsureOpaqueSecret)
		},
	}

	cmd.Flags().StringVar(&manifestPath, "manifest", "", "ExternalSecret manifest whose source secret to ensure")
	cmd.Flags().StringVar(&namespace, "secret-namespace", "distribution-team", "namespace the ClusterSecretStore reads source secrets from")
	cmd.Flags().StringVar(&kubeContext, "kube-context", "", "kubectl context (defaults to current)")
	_ = cmd.MarkFlagRequired("manifest")
	return cmd
}
