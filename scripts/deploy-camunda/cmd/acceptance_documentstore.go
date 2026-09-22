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
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

const minioDocumentEndpoint = "http://documentstore-minio:9000"

type documentStoreAcceptanceOptions struct {
	namespace, release, kubeContext string
}

func newDocumentStoreAcceptanceCommand() *cobra.Command {
	var opts documentStoreAcceptanceOptions
	command := &cobra.Command{
		Use:   "documentstore-minio",
		Short: "Verify document operations against the MinIO CI fixture",
		Args:  cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			if strings.TrimSpace(opts.namespace) == "" || strings.TrimSpace(opts.release) == "" {
				return errors.New("namespace and release are required")
			}
			deps := productionAcceptanceDependencies(physicalTenantAcceptanceOptions{kubeContext: opts.kubeContext})
			client := &http.Client{
				Timeout: 30 * time.Second,
				CheckRedirect: func(*http.Request, []*http.Request) error {
					return http.ErrUseLastResponse
				},
			}
			deps.doHTTP = client.Do
			return runDocumentStoreAcceptance(command.Context(), opts, deps)
		},
	}
	command.Flags().StringVar(&opts.namespace, "namespace", os.Getenv("TEST_NAMESPACE"), "Camunda namespace")
	command.Flags().StringVar(&opts.release, "release", envDefault("RELEASE_NAME", "integration"), "Camunda release")
	command.Flags().StringVar(&opts.kubeContext, "kube-context", os.Getenv("KUBE_CONTEXT"), "kubectl context")
	return command
}

func runDocumentStoreAcceptance(ctx context.Context, opts documentStoreAcceptanceOptions, deps acceptanceDependencies) error {
	ctx, cancel := context.WithTimeout(ctx, 8*time.Minute)
	defer cancel()
	args := kubectlArgs(opts.kubeContext, "-n", opts.namespace, "wait", "--for=condition=complete", "--timeout=300s", "job/documentstore-minio-bucket")
	if output, err := deps.runCommand(ctx, "kubectl", args...); err != nil {
		return fmt.Errorf("MinIO bucket initialization failed: %w: %s", err, output)
	}
	ports := make([]string, 0, 2)
	for _, target := range []struct{ resource, port string }{
		{"service/" + opts.release + "-zeebe-gateway", ":8080"},
		{"service/documentstore-minio", ":9000"},
	} {
		process, err := deps.startForward(ctx, []string{"-n", opts.namespace, "port-forward", target.resource, target.port})
		if err != nil {
			return fmt.Errorf("start %s port-forward: %w", target.resource, err)
		}
		defer process.Stop()
		port, err := waitForForwardPort(ctx, process, deps.sleep)
		if err != nil {
			return fmt.Errorf("forward %s: %w", target.resource, err)
		}
		ports = append(ports, port)
	}
	apiURL := "http://127.0.0.1:" + ports[0] + "/orchestration/v2"
	if err := verifyDocumentStore(ctx, deps.doHTTP, apiURL, "127.0.0.1:"+ports[1]); err != nil {
		return err
	}
	fmt.Fprintln(deps.out, "MinIO document upload, API download, presigned download, and deletion verified.")
	return nil
}

func verifyDocumentStore(ctx context.Context, doHTTP func(*http.Request) (*http.Response, error), apiURL, minioAddress string) error {
	payload := []byte("Camunda Helm S3-compatible document-store acceptance\n")
	var upload bytes.Buffer
	writer := multipart.NewWriter(&upload)
	part, err := writer.CreateFormFile("file", "minio-acceptance.txt")
	if err != nil {
		return err
	}
	if _, err = part.Write(payload); err != nil {
		return err
	}
	if err = writer.Close(); err != nil {
		return err
	}
	requestAPI := func(method, target, contentType string, body io.Reader) ([]byte, int, error) {
		request, err := http.NewRequestWithContext(ctx, method, target, body)
		if err != nil {
			return nil, 0, err
		}
		request.SetBasicAuth("demo", "demo")
		if contentType != "" {
			request.Header.Set("Content-Type", contentType)
		}
		return documentStoreResponse(doHTTP, request)
	}
	body, status, err := requestAPI(http.MethodPost, apiURL+"/documents", writer.FormDataContentType(), &upload)
	if err != nil || status != http.StatusCreated {
		return fmt.Errorf("document upload: expected HTTP 201, got %d (error: %v)", status, err)
	}
	var reference struct {
		DocumentID  string `json:"documentId"`
		StoreID     string `json:"storeId"`
		ContentHash string `json:"contentHash"`
	}
	if err := json.Unmarshal(body, &reference); err != nil {
		return fmt.Errorf("decode document reference: %w", err)
	}
	if reference.DocumentID == "" || reference.StoreID != "aws" || reference.ContentHash == "" {
		return errors.New("document reference must contain a documentId, aws storeId, and contentHash")
	}
	documentURL := apiURL + "/documents/" + url.PathEscape(reference.DocumentID)
	query := url.Values{"storeId": {reference.StoreID}, "contentHash": {reference.ContentHash}}.Encode()
	deleteURL := documentURL + "?" + url.Values{"storeId": {reference.StoreID}}.Encode()
	deleted := false
	defer func() {
		if !deleted {
			_, _, _ = requestAPI(http.MethodDelete, deleteURL, "", nil)
		}
	}()
	body, status, err = requestAPI(http.MethodGet, documentURL+"?"+query, "", nil)
	if err != nil || status != http.StatusOK || !bytes.Equal(body, payload) {
		return fmt.Errorf("document download: expected HTTP 200 and original content, got %d (error: %v)", status, err)
	}
	body, status, err = requestAPI(http.MethodPost, documentURL+"/links?"+query, "application/json", strings.NewReader("{}"))
	if err != nil || (status != http.StatusOK && status != http.StatusCreated) {
		return fmt.Errorf("document link: expected HTTP 200 or 201, got %d (error: %v)", status, err)
	}
	var link struct {
		URL string `json:"url"`
	}
	if err := json.Unmarshal(body, &link); err != nil {
		return fmt.Errorf("decode document link: %w", err)
	}
	signedURL, err := url.Parse(link.URL)
	if err != nil {
		return errors.New("document link is not a valid URL")
	}
	if signedURL.Scheme+"://"+signedURL.Host != minioDocumentEndpoint || signedURL.User != nil ||
		!strings.HasPrefix(signedURL.Path, "/documents/") || signedURL.Query().Get("X-Amz-Signature") == "" {
		return errors.New("document link must be a signed path-style URL at the configured MinIO endpoint")
	}
	minioHost := signedURL.Host
	signedURL.Host = minioAddress
	requestMinio := func() ([]byte, int, error) {
		request, err := http.NewRequestWithContext(ctx, http.MethodGet, signedURL.String(), nil)
		if err != nil {
			return nil, 0, err
		}
		request.Host = minioHost
		return documentStoreResponse(doHTTP, request)
	}
	body, status, err = requestMinio()
	if err != nil || status != http.StatusOK || !bytes.Equal(body, payload) {
		return fmt.Errorf("presigned download: expected HTTP 200 and original content, got %d (error: %v)", status, err)
	}
	_, status, err = requestAPI(http.MethodDelete, deleteURL, "", nil)
	if err != nil || status != http.StatusNoContent {
		return fmt.Errorf("document deletion: expected HTTP 204, got %d (error: %v)", status, err)
	}
	deleted = true
	_, status, err = requestAPI(http.MethodGet, documentURL+"?"+query, "", nil)
	if err != nil || status != http.StatusNotFound {
		return fmt.Errorf("deleted document download: expected HTTP 404, got %d (error: %v)", status, err)
	}
	_, status, err = requestMinio()
	if err != nil || status != http.StatusNotFound {
		return fmt.Errorf("deleted MinIO object: expected HTTP 404, got %d (error: %v)", status, err)
	}
	return nil
}

func documentStoreResponse(doHTTP func(*http.Request) (*http.Response, error), request *http.Request) ([]byte, int, error) {
	response, err := doHTTP(request)
	if err != nil {
		var urlError *url.Error
		if errors.As(err, &urlError) {
			err = urlError.Err
		}
		return nil, 0, err
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	return body, response.StatusCode, err
}
