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
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDocumentStoreAcceptance(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name, failure, wantError string
		linkStatus               int
	}{
		{name: "complete lifecycle with HTTP 200 link", linkStatus: http.StatusOK},
		{name: "complete lifecycle with HTTP 201 link", linkStatus: http.StatusCreated},
		{name: "link rejected", linkStatus: http.StatusInternalServerError, wantError: "document link"},
		{name: "wrong API content", failure: "download", wantError: "document download"},
		{name: "wrong backend", failure: "endpoint", wantError: "configured MinIO endpoint"},
		{name: "unsigned URL", failure: "signature", wantError: "signed path-style URL"},
		{name: "wrong store", failure: "store", wantError: "aws storeId"},
		{name: "missing content hash", failure: "hash", wantError: "contentHash"},
		{name: "bad reference JSON", failure: "reference", wantError: "decode document reference"},
		{name: "bad link JSON", failure: "link", wantError: "decode document link"},
		{name: "wrong MinIO content", failure: "minio", wantError: "presigned download"},
		{name: "delete rejected", failure: "delete", wantError: "document deletion"},
		{name: "API still serves deleted document", failure: "api-delete", wantError: "deleted document download"},
		{name: "MinIO object not deleted", failure: "minio-delete", wantError: "deleted MinIO object"},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			var payload []byte
			deleted, minioReads, deletes := false, 0, 0
			doHTTP := func(request *http.Request) (*http.Response, error) {
				if request.URL.Host == "127.0.0.1:9001" {
					minioReads++
					assert.Equal(t, "documentstore-minio:9000", request.Host)
					assert.Empty(t, request.Header.Get("Authorization"))
					assert.Equal(t, "/documents/test-document", request.URL.Path)
					assert.Equal(t, "test-signature", request.URL.Query().Get("X-Amz-Signature"))
					if deleted && test.failure != "minio-delete" {
						return testResponse(http.StatusNotFound, ""), nil
					}
					if test.failure == "minio" {
						return testResponse(http.StatusOK, "wrong content"), nil
					}
					return testResponse(http.StatusOK, string(payload)), nil
				}
				assert.Equal(t, "orchestration.local", request.URL.Host)
				username, password, ok := request.BasicAuth()
				assert.True(t, ok)
				assert.Equal(t, "demo", username)
				assert.Equal(t, "demo", password)
				if request.URL.Path == "/orchestration/v2/documents" {
					assert.Equal(t, http.MethodPost, request.Method)
					require.NoError(t, request.ParseMultipartForm(1<<20))
					defer request.MultipartForm.RemoveAll()
					file, header, err := request.FormFile("file")
					require.NoError(t, err)
					defer file.Close()
					assert.Equal(t, "minio-acceptance.txt", header.Filename)
					payload, err = io.ReadAll(file)
					require.NoError(t, err)
					if test.failure == "reference" {
						return testResponse(http.StatusCreated, "{"), nil
					}
					reference := map[string]string{"documentId": "test-document", "storeId": "aws", "contentHash": "hash+/="}
					if test.failure == "store" {
						reference["storeId"] = "inmemory"
					}
					if test.failure == "hash" {
						delete(reference, "contentHash")
					}
					body, err := json.Marshal(reference)
					require.NoError(t, err)
					return testResponse(http.StatusCreated, string(body)), nil
				}
				assert.Equal(t, "aws", request.URL.Query().Get("storeId"))
				if request.Method == http.MethodDelete {
					deletes++
					if test.failure == "delete" {
						return testResponse(http.StatusInternalServerError, ""), nil
					}
					deleted = true
					return testResponse(http.StatusNoContent, ""), nil
				}
				assert.Equal(t, "hash+/=", request.URL.Query().Get("contentHash"))
				if strings.HasSuffix(request.URL.Path, "/links") {
					assert.Equal(t, http.MethodPost, request.Method)
					if test.failure == "link" {
						return testResponse(http.StatusCreated, "{"), nil
					}
					target := minioDocumentEndpoint + "/documents/test-document?X-Amz-Signature=test-signature"
					if test.failure == "endpoint" {
						target = "https://s3.amazonaws.com/documents/test-document?X-Amz-Signature=test-signature"
					}
					if test.failure == "signature" {
						target = minioDocumentEndpoint + "/documents/test-document"
					}
					body, err := json.Marshal(map[string]string{"url": target})
					require.NoError(t, err)
					linkStatus := test.linkStatus
					if linkStatus == 0 {
						linkStatus = http.StatusOK
					}
					return testResponse(linkStatus, string(body)), nil
				}
				assert.Equal(t, http.MethodGet, request.Method)
				if deleted && test.failure != "api-delete" {
					return testResponse(http.StatusNotFound, ""), nil
				}
				if test.failure == "download" {
					return testResponse(http.StatusOK, "wrong content"), nil
				}
				return testResponse(http.StatusOK, string(payload)), nil
			}
			err := verifyDocumentStore(context.Background(), doHTTP, "http://orchestration.local/orchestration/v2", "127.0.0.1:9001")
			if test.wantError == "" {
				require.NoError(t, err)
				assert.True(t, deleted)
				assert.Equal(t, 2, minioReads)
				assert.Equal(t, 1, deletes)
			} else {
				require.ErrorContains(t, err, test.wantError)
				if test.failure != "reference" && test.failure != "store" && test.failure != "hash" {
					assert.Positive(t, deletes)
				}
			}
		})
	}
}

func TestDocumentStoreResponseRedactsSignedURL(t *testing.T) {
	t.Parallel()
	request, err := http.NewRequest(http.MethodGet, "http://localhost/?X-Amz-Signature=secret", nil)
	require.NoError(t, err)
	_, _, err = documentStoreResponse(func(*http.Request) (*http.Response, error) {
		return nil, &url.Error{Op: "Get", URL: request.URL.String(), Err: errors.New("connection refused")}
	}, request)
	require.ErrorContains(t, err, "connection refused")
	assert.NotContains(t, err.Error(), "secret")
}

func TestRunDocumentStoreAcceptanceCleansUp(t *testing.T) {
	t.Parallel()
	for _, failure := range []string{"bucket", "forward-start", "forward-dead", "upload"} {
		t.Run(failure, func(t *testing.T) {
			t.Parallel()
			var processes []*fakeAcceptanceProcess
			deps := acceptanceDependencies{
				out: io.Discard,
				runCommand: func(ctx context.Context, name string, args ...string) ([]byte, error) {
					_, hasDeadline := ctx.Deadline()
					assert.True(t, hasDeadline)
					assert.Equal(t, "kubectl", name)
					assert.Equal(t, []string{"--context", "test-cluster", "-n", "test-namespace", "wait", "--for=condition=complete", "--timeout=300s", "job/documentstore-minio-bucket"}, args)
					if failure == "bucket" {
						return nil, errors.New("bucket unavailable")
					}
					return nil, nil
				},
				startForward: func(_ context.Context, args []string) (acceptanceProcess, error) {
					if len(processes) == 0 {
						assert.Equal(t, []string{"-n", "test-namespace", "port-forward", "service/integration-zeebe-gateway", ":8080"}, args)
					} else {
						assert.Equal(t, []string{"-n", "test-namespace", "port-forward", "service/documentstore-minio", ":9000"}, args)
						if failure == "forward-start" {
							return nil, errors.New("forward unavailable")
						}
					}
					process := &fakeAcceptanceProcess{alive: true, output: fmt.Sprintf("Forwarding from 127.0.0.1:%d", 18080+len(processes))}
					if failure == "forward-dead" {
						process.alive, process.output = false, "forward died"
					}
					processes = append(processes, process)
					return process, nil
				},
				doHTTP: func(request *http.Request) (*http.Response, error) {
					assert.Equal(t, "127.0.0.1:18080", request.URL.Host)
					return testResponse(http.StatusServiceUnavailable, ""), nil
				},
			}
			err := runDocumentStoreAcceptance(context.Background(), documentStoreAcceptanceOptions{
				namespace: "test-namespace", release: "integration", kubeContext: "test-cluster",
			}, deps)
			require.Error(t, err)
			if failure == "bucket" {
				assert.Empty(t, processes)
			} else {
				assert.NotEmpty(t, processes)
				for _, process := range processes {
					assert.True(t, process.stopped)
				}
			}
		})
	}
}

func TestDocumentStoreAcceptanceCommand(t *testing.T) {
	t.Parallel()
	parent := newAcceptanceCommand()
	command, _, err := parent.Find([]string{"documentstore-minio"})
	require.NoError(t, err)
	assert.Equal(t, "documentstore-minio", command.Name())
	parent.SetArgs([]string{"documentstore-minio", "--namespace=", "--release=integration"})
	require.ErrorContains(t, parent.Execute(), "namespace and release are required")
}
