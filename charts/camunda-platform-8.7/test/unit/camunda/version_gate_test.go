package camunda

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/gruntwork-io/terratest/modules/helm"
	"github.com/gruntwork-io/terratest/modules/k8s"
	"github.com/gruntwork-io/terratest/modules/logger"
	"github.com/stretchr/testify/require"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
)

func withEnvtest(t *testing.T, fn func(cfg *rest.Config, kubeconfig string, namespace string, absTestChart string)) {
	t.Helper()

	// The envtest control-plane binaries (kube-apiserver, etc.) have to be
	// present on the host where the tests run.  In CI we download them via the
	// `kubernetes-sigs/setup-envtest` GitHub Action which exports the
	// KUBEBUILDER_ASSETS environment variable.  When running the tests
	// locally users can either:
	//   * run `go install sigs.k8s.io/controller-runtime/tools/setup-envtest@latest && \ 
	//         setup-envtest use 1.30.0 --bin-dir $(pwd)/testbin`
	//   * export KUBEBUILDER_ASSETS to point at a directory that contains the
	//     required binaries.
	//
	// We deliberately fail (instead of silently skipping) so that missing
	// assets are obvious to contributors.
	assetsDir := os.Getenv("KUBEBUILDER_ASSETS")
	if assetsDir == "" {
		t.Fatalf(`KUBEBUILDER_ASSETS environment variable is not set. This test requires the Kubernetes envtest binaries.

You can obtain them by installing the setup-envtest tool:

    go install sigs.k8s.io/controller-runtime/tools/setup-envtest@latest
    setup-envtest use 1.30.0 --bin-dir ./testbin

and then export the variable before running tests:

    export KUBEBUILDER_ASSETS="$(pwd)/testbin"

In CI the binaries are provisioned automatically via the "kubernetes-sigs/setup-envtest" action.`)
	}

	testEnv := &envtest.Environment{
		BinaryAssetsDirectory: assetsDir,
	}

	cfg, err := testEnv.Start()
	require.NoError(t, err)
	defer testEnv.Stop()

	// write a kubeconfig that helm can read
	tmp := t.TempDir()
	kcPath := filepath.Join(tmp, "kubeconfig")
	kc := clientcmdapi.Config{
		Clusters: map[string]*clientcmdapi.Cluster{
			"default": {Server: cfg.Host, CertificateAuthorityData: cfg.CAData},
		},
		Contexts: map[string]*clientcmdapi.Context{
			"default": {Cluster: "default", AuthInfo: "default"},
		},
		AuthInfos: map[string]*clientcmdapi.AuthInfo{
			"default": {
				ClientCertificateData: cfg.CertData,
				ClientKeyData:         cfg.KeyData,
			},
		},
		CurrentContext: "default",
	}
	require.NoError(t, clientcmd.WriteToFile(kc, kcPath))

	helmChartPath := "../../../" // this is the root of the camunda-platform-8.7 chart...
	testChartPath := "test-chart" // this is the path to the test-chart directory...

	absChart, err := filepath.Abs(helmChartPath)
	require.NoError(t, err)
	absTestChart, err := filepath.Abs(testChartPath)
	require.NoError(t, err)

	// we need to symlink the version-gate.tpl file from the camunda-platform-8.7 chart to the test-chart directory otherwise
	// the test-chart doesn't know about the version-gate.tpl file and will fail to render the templates.
	templateSrc := filepath.Join(absChart, "templates", "camunda", "_version-gate.tpl")
	templateDst := filepath.Join(absTestChart, "templates", "camunda", "_version-gate.tpl")
	_ = os.MkdirAll(filepath.Dir(templateDst), 0o755)
	_ = os.Remove(templateDst)
	require.NoError(t, os.Symlink(templateSrc, templateDst))
	defer os.Remove(templateDst)

	fn(cfg, kcPath, "default", absTestChart)
}

// Integration-style test that runs the full set of scenarios against a
// live control-plane provided by envtest so that the Helm `lookup` function
// can see real Kubernetes objects.
func TestVersionGate_LookupScenarios(t *testing.T) {
	// Re-use the same declarative test table from the unit test section.
	type scenario struct {
		name          string
		setValues     map[string]any // values passed with --set
		currentTag    string         // empty -> no deployment exists (install)
		expectedError string
	}

	scenarios := []scenario{
		{
			name: "valid upgrade path",
			setValues: map[string]any{
				"global": map[string]any{
					"image": map[string]any{
						"tag": "8.2.0",
					},
				},
				"currentdeployment": map[string]any{
					"image": map[string]any{
						"tag": "8.2.0",
					},
				},
			},
			currentTag:    "8.1.0",
			expectedError: "",
		},
		{
			name: "downgrade attempt should fail",
			setValues: map[string]any{
				"global": map[string]any{
					"image": map[string]any{
						"tag": "8.1.0",
					},
				},
				"currentdeployment": map[string]any{
					"image": map[string]any{
						"tag": "8.1.0",
					},
				},
			},
			currentTag:    "8.7.0",
			expectedError: "downgrade detected: 8.7.0 -> 8.1.0",
		},
		{
			name: "latest tag should pass",
			setValues: map[string]any{
				"global": map[string]any{
					"image": map[string]any{
						"tag": "latest",
					},
				},
				"currentdeployment": map[string]any{
					"image": map[string]any{
						"tag": "8.1.0",
					},
				},
			},
			currentTag:    "8.1.0",
			expectedError: "",
		},
		{
			name: "invalid semver should fail",
			setValues: map[string]any{
				"global": map[string]any{
					"image": map[string]any{
						"tag": "invalid-version",
					},
				},
				"currentdeployment": map[string]any{
					"image": map[string]any{
						"tag": "8.2.0",
					},
				},
			},
			currentTag:    "8.1.0",
			expectedError: "new tag \"invalid-version\" is not a valid semver",
		},
		{
			name:          "missing required parameter should fail",
			setValues:     map[string]any{},
			currentTag:    "8.1.0",
			expectedError: "global parameter must be provided",
		},
		{
			name: "v-prefixed versions should work",
			setValues: map[string]any{
				"global": map[string]any{
					"image": map[string]any{
						"tag": "v8.2.0",
					},
				},
				"currentdeployment": map[string]any{
					"image": map[string]any{
						"tag": "v8.1.0",
					},
				},
			},
			currentTag:    "v8.1.0",
			expectedError: "",
		},
	}

	withEnvtest(t, func(cfg *rest.Config, kubeconfig, ns string, absTestChart string) {
		for _, sc := range scenarios {
			t.Run(sc.name, func(t *testing.T) {
				ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
				defer cancel()

				fmt.Printf("sc: %+v\n", sc)

				cl, err := client.New(cfg, client.Options{})
				require.NoError(t, err)

				// First, uninstall any previous Helm release to ensure a clean slate.
				opts := &helm.Options{
					KubectlOptions: &k8s.KubectlOptions{
						ConfigPath: kubeconfig,
						Namespace:  ns,
					},
					Logger: logger.Discard,
				}
				_, _ = helm.RunHelmCommandAndGetStdOutE(t, opts,
					"uninstall", "test-release", "-n", ns,
				)

				// Remove any leftover deployment that may still exist (best-effort).
				_ = cl.Delete(ctx, &appsv1.Deployment{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-release-currentdeployment",
						Namespace: ns,
					},
				})

				// If the scenario specifies a current tag, install a release with
				// that tag so Helm treats the subsequent command as an upgrade.
				if sc.currentTag != "" {
					initialVals := map[string]any{
						"currentdeployment": map[string]any{
							"image": map[string]any{
								"tag": sc.currentTag,
							},
						},
					}

					stdout, err := helm.RunHelmCommandAndGetStdOutE(
						t, opts,
						"install", "test-release", absTestChart,
						"-n", ns,
						"--set-json", convertMapToJsonString(initialVals),
						"--debug",
					)

					fmt.Printf("stdout: %s\n", stdout)
					fmt.Printf("err: %v\n", err)
					require.NoError(t, err)
				}

				stdout, err := helm.RunHelmCommandAndGetStdOutE(
					t, opts,
					"upgrade", "--dry-run=server",
					"test-release", absTestChart,
					"-n", ns,
					"--set-json", convertMapToJsonString(sc.setValues),
					"--debug",
				)

				fmt.Printf("stdout: %s\n", stdout)
				fmt.Printf("err: %v\n", err)

				if sc.expectedError != "" {
					if err == nil || !strings.Contains(err.Error(), sc.expectedError) {
						printDebugOnFailure(t, sc.name, stdout, err)
					}
					require.Error(t, err)
					require.Contains(t, err.Error(), sc.expectedError)
				} else {
					if err != nil {
						printDebugOnFailure(t, sc.name, stdout, err)
					}
					require.NoError(t, err)
				}
			})
		}
	})
}

func convertMapToJsonString(in any) string {
	m, ok := in.(map[string]any)
	if !ok || len(m) == 0 {
		return ""
	}

	var result []string
	for k, v := range m {
		j, err := json.Marshal(v)
		if err != nil {
			panic(err)
		}
		result = append(result, fmt.Sprintf("%s=%s", k, string(j)))
	}

	return strings.Join(result, ",")
}

func printDebugOnFailure(t *testing.T, name, yaml string, err error) {
	t.Helper()
	fmt.Printf("YAML output for failed test '%s':\n%s\n", name, yaml)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
	}
}
