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
	"context"
	"encoding/base64"
	"errors"
	"io"
	"net/http"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPartitionsHealthy(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		body string
		want bool
	}{
		{"healthy", `{"default":[{"exporterPhase":"EXPORTING","exportedPosition":1}],"tenanta":[{"exporterPhase":"EXPORTING","exportedPosition":2}],"tenantb":[{"exporterPhase":"EXPORTING","exportedPosition":3}]}`, true},
		{"missing tenant", `{"default":[{"exporterPhase":"EXPORTING","exportedPosition":1}],"tenanta":[{"exporterPhase":"EXPORTING","exportedPosition":2}]}`, false},
		{"empty tenant", `{"default":[{"exporterPhase":"EXPORTING","exportedPosition":1}],"tenanta":[],"tenantb":[{"exporterPhase":"EXPORTING","exportedPosition":3}]}`, false},
		{"blocked", `{"default":[{"exporterPhase":"EXPORTING","exportedPosition":1}],"tenanta":[{"exporterPhase":"EXPORTING","exportedPosition":2}],"tenantb":[{"exporterPhase":"EXPORTING","exportedPosition":-1}]}`, false},
		{"extra tenant", `{"default":[{"exporterPhase":"EXPORTING","exportedPosition":1}],"tenanta":[{"exporterPhase":"EXPORTING","exportedPosition":2}],"tenantb":[{"exporterPhase":"EXPORTING","exportedPosition":3}],"other":[]}`, false},
		{"invalid JSON", `{`, false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, test.want, partitionsHealthy([]byte(test.body)))
		})
	}
}

func TestValidateTokenAudience(t *testing.T) {
	t.Parallel()

	token := testJWT(`{"aud":["own-api","identity"]}`)
	require.NoError(t, validateTokenAudience(token, "own-api", []string{"sibling-api"}))
	assert.ErrorContains(t, validateTokenAudience(token, "missing-api", nil), "missing")
	assert.ErrorContains(t, validateTokenAudience(testJWT(`{"aud":["own-api","sibling-api"]}`), "own-api", []string{"sibling-api"}), "forbidden")
	require.NoError(t, validateTokenAudience(testJWT(`{"aud":"own-api"}`), "own-api", nil))
	assert.Error(t, validateTokenAudience("not-a-jwt", "own-api", nil))
}

func TestTenantAPIPathAndDefinitionMatching(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "/orchestration/v2", tenantAPIPath("default"))
	assert.Equal(t, "/orchestration/physical-tenants/tenanta/v2", tenantAPIPath("tenanta"))
	body := []byte(`[{"key":"key-match","name":"name-match"}]`)
	assert.True(t, definitionsContain(body, "key-match"))
	assert.True(t, definitionsContain(body, "name-match"))
	assert.False(t, definitionsContain(body, "sibling"))
	assert.False(t, definitionsContain([]byte(`{`), "key-match"))
}

func TestWaitForOptimizeImportRetriesTransportFailureThenSucceeds(t *testing.T) {
	t.Parallel()

	calls, sleeps := 0, 0
	deps := pollingDependencies(func(*http.Request) (*http.Response, error) {
		calls++
		if calls == 1 {
			return nil, errors.New("connection reset")
		}
		return testResponse(http.StatusOK, `[{"key":"own-process"}]`), nil
	}, &sleeps)

	err := waitForOptimizeImport(context.Background(), deps, "https://optimize", "tenanta", "own-process", "token", 2)
	require.NoError(t, err)
	assert.Equal(t, 2, calls)
	assert.Equal(t, 1, sleeps)
}

func TestWaitForOptimizeImportPreservesLastDiagnostic(t *testing.T) {
	t.Parallel()

	calls, sleeps := 0, 0
	deps := pollingDependencies(func(*http.Request) (*http.Response, error) {
		calls++
		if calls == 1 {
			return testResponse(http.StatusServiceUnavailable, "warming up"), nil
		}
		return nil, errors.New("connection refused")
	}, &sleeps)

	err := waitForOptimizeImport(context.Background(), deps, "https://optimize", "tenanta", "own-process", "token", 2)
	require.ErrorContains(t, err, "last response: transport error: connection refused")
	assert.Equal(t, 1, sleeps)
}

func TestAssertOptimizeExcludesSiblingsRetriesTransportFailureThenSucceeds(t *testing.T) {
	t.Parallel()

	calls, sleeps := 0, 0
	deps := pollingDependencies(func(*http.Request) (*http.Response, error) {
		calls++
		if calls == 1 {
			return nil, errors.New("unexpected EOF")
		}
		return testResponse(http.StatusOK, `[{"key":"own-process"}]`), nil
	}, &sleeps)

	err := assertOptimizeExcludesSiblings(context.Background(), deps, "https://optimize", "tenanta", "token", []string{"sibling-process"}, 2)
	require.NoError(t, err)
	assert.Equal(t, 2, calls)
	assert.Equal(t, 1, sleeps)
}

func TestAssertOptimizeExcludesSiblingsFailures(t *testing.T) {
	t.Parallel()

	t.Run("preserves final transport diagnostic", func(t *testing.T) {
		calls, sleeps := 0, 0
		deps := pollingDependencies(func(*http.Request) (*http.Response, error) {
			calls++
			return nil, errors.New("dial failure " + string(rune('0'+calls)))
		}, &sleeps)
		err := assertOptimizeExcludesSiblings(context.Background(), deps, "https://optimize", "tenanta", "token", nil, 2)
		require.ErrorContains(t, err, "dial failure 2")
	})

	t.Run("does not retry HTTP failure", func(t *testing.T) {
		calls, sleeps := 0, 0
		deps := pollingDependencies(func(*http.Request) (*http.Response, error) {
			calls++
			return testResponse(http.StatusForbidden, "denied"), nil
		}, &sleeps)
		err := assertOptimizeExcludesSiblings(context.Background(), deps, "https://optimize", "tenanta", "token", nil, 2)
		require.ErrorContains(t, err, "HTTP 403")
		assert.Equal(t, 1, calls)
		assert.Zero(t, sleeps)
	})

	t.Run("detects sibling", func(t *testing.T) {
		sleeps := 0
		deps := pollingDependencies(func(*http.Request) (*http.Response, error) {
			return testResponse(http.StatusOK, `[{"name":"sibling-process"}]`), nil
		}, &sleeps)
		err := assertOptimizeExcludesSiblings(context.Background(), deps, "https://optimize", "tenanta", "token", []string{"sibling-process"}, 2)
		require.ErrorContains(t, err, "imported sibling process")
	})
}

func TestClientToken(t *testing.T) {
	t.Parallel()

	var requestBody string
	token, err := clientToken(context.Background(), func(req *http.Request) (*http.Response, error) {
		body, readErr := io.ReadAll(req.Body)
		require.NoError(t, readErr)
		requestBody = string(body)
		assert.Equal(t, http.MethodPost, req.Method)
		return testResponse(http.StatusOK, `{"access_token":"jwt"}`), nil
	}, "https://hub", "client", "secret value")
	require.NoError(t, err)
	assert.Equal(t, "jwt", token)
	assert.Contains(t, requestBody, "client_secret=secret+value")

	_, err = clientToken(context.Background(), func(*http.Request) (*http.Response, error) {
		return testResponse(http.StatusUnauthorized, "bad credentials"), nil
	}, "https://hub", "client", "secret")
	require.ErrorContains(t, err, "HTTP 401: bad credentials")
}

func TestDeployAndStartAcceptanceProcess(t *testing.T) {
	t.Parallel()

	requests := 0
	doHTTP := func(req *http.Request) (*http.Response, error) {
		requests++
		assert.Equal(t, "Bearer venom-token", req.Header.Get("Authorization"))
		if requests == 1 {
			assert.Equal(t, "/orchestration/physical-tenants/tenanta/v2/deployments", req.URL.Path)
			body, err := io.ReadAll(req.Body)
			require.NoError(t, err)
			assert.Contains(t, string(body), "pt-accept-tenanta")
			return testResponse(http.StatusOK, `{"deployments":[{"processDefinition":{"processDefinitionKey":"123"}}]}`), nil
		}
		assert.Equal(t, "/orchestration/physical-tenants/tenanta/v2/process-instances", req.URL.Path)
		body, err := io.ReadAll(req.Body)
		require.NoError(t, err)
		assert.JSONEq(t, `{"processDefinitionKey":"123","variables":{"physicalTenantAcceptanceMarker":"marker"}}`, string(body))
		return testResponse(http.StatusNoContent, ""), nil
	}

	key, err := deployAcceptanceProcess(context.Background(), doHTTP, "https://orch", "tenanta", "pt-accept-tenanta", "venom-token")
	require.NoError(t, err)
	assert.Equal(t, "123", key)
	require.NoError(t, startAcceptanceProcess(context.Background(), doHTTP, "https://orch", "tenanta", key, "marker", "venom-token"))
}

func TestReadSecretUsesContextAndDecodesValue(t *testing.T) {
	t.Parallel()

	deps := acceptanceDependencies{runCommand: func(_ context.Context, name string, args ...string) ([]byte, error) {
		assert.Equal(t, "kubectl", name)
		assert.Equal(t, []string{"--context", "cluster", "-n", "hub", "get", "secret", "integration-test-credentials", "-o", "jsonpath={.data.key}"}, args)
		return []byte(base64.StdEncoding.EncodeToString([]byte("secret"))), nil
	}}
	secret, err := readSecret(context.Background(), deps, physicalTenantAcceptanceOptions{hubNamespace: "hub", kubeContext: "cluster"}, "key")
	require.NoError(t, err)
	assert.Equal(t, "secret", secret)
}

type fakeAcceptanceProcess struct {
	alive   bool
	output  string
	stopped bool
}

func (p *fakeAcceptanceProcess) Alive() bool    { return p.alive }
func (p *fakeAcceptanceProcess) Output() string { return p.output }
func (p *fakeAcceptanceProcess) Stop() error {
	p.stopped = true
	return nil
}

func TestRunPhysicalTenantAcceptance(t *testing.T) {
	t.Parallel()

	opts := physicalTenantAcceptanceOptions{
		namespace: "camunda", release: "integration", hubNamespace: "hub", hubHost: "hub.example",
		orchHost: "orch.example", kubeContext: "cluster",
		optimizePaths: map[string]string{"default": "/opt-default", "tenanta": "/opt-a", "tenantb": "/opt-b"},
	}
	process := &fakeAcceptanceProcess{alive: true, output: "Forwarding from 127.0.0.1:43123 -> 9600\n"}
	processes := map[string]string{}
	tokens := map[string]string{}
	secretReads, deployments, starts, crossChecks := 0, 0, 0, 0
	processPattern := regexp.MustCompile(`pt-accept-(default|tenanta|tenantb)-1700000000-42`)

	deps := acceptanceDependencies{
		startForward: func(_ context.Context, args []string) (acceptanceProcess, error) {
			assert.Equal(t, []string{"-n", "camunda", "port-forward", "statefulset/integration-zeebe", ":9600"}, args)
			return process, nil
		},
		runCommand: func(_ context.Context, name string, args ...string) ([]byte, error) {
			assert.Equal(t, "kubectl", name)
			joined := strings.Join(args, " ")
			if strings.Contains(joined, " logs ") {
				assert.Contains(t, joined, "--context cluster")
				return []byte("broker.exporter.elasticsearch opened physicalTenant=default}\n" +
					"broker.exporter.elasticsearch opened physicalTenant=tenanta}\n" +
					"broker.exporter.opensearch opened physicalTenant=tenantb}"), nil
			}
			secretReads++
			assert.Contains(t, joined, "--context cluster -n hub get secret integration-test-credentials")
			return []byte(base64.StdEncoding.EncodeToString([]byte("secret"))), nil
		},
		doHTTP: func(req *http.Request) (*http.Response, error) {
			path := req.URL.Path
			switch {
			case req.URL.Host == "127.0.0.1:43123" && path == "/orchestration/actuator":
				return testResponse(http.StatusOK, "{}"), nil
			case req.URL.Host == "127.0.0.1:43123" && path == "/orchestration/actuator/partitions":
				return testResponse(http.StatusOK, `{"default":[{"exporterPhase":"EXPORTING","exportedPosition":1}],"tenanta":[{"exporterPhase":"EXPORTING","exportedPosition":2}],"tenantb":[{"exporterPhase":"EXPORTING","exportedPosition":3}]}`), nil
			case strings.HasSuffix(path, "/protocol/openid-connect/token"):
				require.NoError(t, req.ParseForm())
				clientID := req.Form.Get("client_id")
				token := testJWT(`{"aud":"` + clientID + `-api"}`)
				if clientID == "venom" {
					token = testJWT(`{"aud":"orchestration-api"}`)
				}
				tokens[clientID] = token
				return testResponse(http.StatusOK, `{"access_token":"`+token+`"}`), nil
			case strings.HasSuffix(path, "/deployments"):
				deployments++
				body, err := io.ReadAll(req.Body)
				require.NoError(t, err)
				processID := processPattern.FindString(string(body))
				require.NotEmpty(t, processID)
				tenant := strings.TrimPrefix(strings.Split(processID, "-1700000000")[0], "pt-accept-")
				processes[tenant] = processID
				return testResponse(http.StatusOK, `{"deployments":[{"processDefinition":{"processDefinitionKey":"key-`+tenant+`"}}]}`), nil
			case strings.HasSuffix(path, "/process-instances"):
				starts++
				return testResponse(http.StatusNoContent, ""), nil
			case strings.HasSuffix(path, "/api/definition/process/keys"):
				target := map[string]string{"/opt-default": "default", "/opt-a": "tenanta", "/opt-b": "tenantb"}[strings.TrimSuffix(path, "/api/definition/process/keys")]
				auth := strings.TrimPrefix(req.Header.Get("Authorization"), "Bearer ")
				if auth != tokens["optimize-orcha-"+target] {
					crossChecks++
					return testResponse(http.StatusForbidden, "denied"), nil
				}
				return testResponse(http.StatusOK, `[{"key":"`+processes[target]+`"}]`), nil
			default:
				t.Fatalf("unexpected HTTP request: %s %s", req.Method, req.URL)
				return nil, nil
			}
		},
		sleep:  func(context.Context, time.Duration) error { return nil },
		now:    func() time.Time { return time.Unix(1700000000, 0) },
		pid:    func() int { return 42 },
		out:    io.Discard,
		errOut: io.Discard,
	}

	require.NoError(t, runPhysicalTenantAcceptance(context.Background(), opts, deps))
	assert.True(t, process.stopped)
	assert.Equal(t, 4, secretReads)
	assert.Equal(t, 3, deployments)
	assert.Equal(t, 3, starts)
	assert.Equal(t, 6, crossChecks)
}

func testJWT(payload string) string {
	return "header." + base64.RawURLEncoding.EncodeToString([]byte(payload)) + ".signature"
}

func testResponse(status int, body string) *http.Response {
	return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}
}

func pollingDependencies(doHTTP func(*http.Request) (*http.Response, error), sleeps *int) acceptanceDependencies {
	return acceptanceDependencies{
		doHTTP: doHTTP,
		sleep: func(context.Context, time.Duration) error {
			*sleeps++
			return nil
		},
	}
}
