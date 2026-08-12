package cmd

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

// gcsUploadBaseURL is the Cloud Storage JSON API upload endpoint. It is a
// variable so tests can point it at an httptest server.
var gcsUploadBaseURL = "https://storage.googleapis.com"

// artifactUploadTimeout bounds the whole upload set. A stuck upload must not
// hold a CI job open, and the artifacts are best-effort evidence.
const artifactUploadTimeout = 5 * time.Minute

// tokenEnvDefault is where the GCP OAuth2 access token is read from. The token
// is never taken as a flag: argv is world-readable via /proc on the runner.
const tokenEnvDefault = "GCS_ACCESS_TOKEN"

// newArtifactsCommand creates the `artifacts` parent command.
func newArtifactsCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "artifacts",
		Short: "Upload CI evidence that cannot be published as a public artifact",
	}
	cmd.AddCommand(newArtifactsUploadCommand())
	return cmd
}

// newArtifactsUploadCommand creates `artifacts upload`: a minimal Cloud Storage
// writer for evidence that must not become a public GitHub artifact.
//
// Playwright traces are the motivating case. camunda-platform-helm is public, so
// its workflow artifacts are world-readable, and a trace embeds the per-run
// cluster's request headers and cookies. A trace cannot be redacted, so the only
// safe retention is a private bucket with a short lifecycle.
//
// This speaks the JSON API over net/http rather than depending on the Cloud
// Storage SDK: the playwright-runner image deliberately ships only the 9 MB
// gke-gcloud-auth-plugin instead of the 370 MB gcloud SDK, and deploy-camunda is
// already on PATH there.
func newArtifactsUploadCommand() *cobra.Command {
	var (
		bucket      string
		source      string
		prefix      string
		pattern     string
		tokenEnv    string
		contentType string
	)

	cmd := &cobra.Command{
		Use:   "upload",
		Short: "Upload files to a private Cloud Storage bucket",
		Long: `Upload files to a private Cloud Storage bucket.

Walks --source for files matching --pattern and writes each to
gs://<bucket>/<prefix>/<path-relative-to-source>. Prints one gs:// URI per
uploaded object so a CI log records where the evidence went.

The OAuth2 access token is read from the environment (default GCS_ACCESS_TOKEN),
never from a flag, because process arguments are readable by other processes on
the runner. On GitHub Actions the token comes from the gke-login action, which
already performs a Workload Identity exchange.

Does nothing and succeeds when --bucket is empty, so a caller can wire the step
up before the bucket exists.`,
		SilenceUsage:  true,
		SilenceErrors: true,
		Args:          cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if bucket == "" {
				fmt.Fprintln(cmd.OutOrStdout(), "no bucket configured; skipping artifact upload")
				return nil
			}
			token := os.Getenv(tokenEnv)
			if token == "" {
				return fmt.Errorf("no access token in %s; refusing to upload", tokenEnv)
			}

			ctx, cancel := context.WithTimeout(cmd.Context(), artifactUploadTimeout)
			defer cancel()

			uploaded, err := uploadArtifacts(ctx, artifactUploadOptions{
				Bucket:      bucket,
				Source:      source,
				Prefix:      prefix,
				Pattern:     pattern,
				Token:       token,
				ContentType: contentType,
				Client:      http.DefaultClient,
			}, cmd.OutOrStdout())
			if err != nil {
				return err
			}
			if uploaded == 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "no files matching %q under %s\n", pattern, source)
			}
			return nil
		},
	}

	f := cmd.Flags()
	f.StringVar(&bucket, "bucket", "", "Target bucket name; empty skips the upload")
	f.StringVar(&source, "source", "", "Directory to walk (required)")
	f.StringVar(&prefix, "prefix", "", "Object name prefix (required)")
	f.StringVar(&pattern, "pattern", "*", "Base-name glob selecting files to upload")
	f.StringVar(&tokenEnv, "token-env", tokenEnvDefault, "Environment variable holding the OAuth2 access token")
	f.StringVar(&contentType, "content-type", "application/octet-stream", "Content-Type recorded on each object")
	_ = cmd.MarkFlagRequired("source")
	_ = cmd.MarkFlagRequired("prefix")

	return cmd
}

type artifactUploadOptions struct {
	Bucket      string
	Source      string
	Prefix      string
	Pattern     string
	Token       string
	ContentType string
	Client      *http.Client
}

// uploadArtifacts walks Source and uploads every matching file, returning the
// number of objects written. Files are uploaded in a stable order so a failure
// is reproducible.
func uploadArtifacts(ctx context.Context, opts artifactUploadOptions, out io.Writer) (int, error) {
	info, err := os.Stat(opts.Source)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, fmt.Errorf("stat %s: %w", opts.Source, err)
	}
	if !info.IsDir() {
		return 0, fmt.Errorf("%s is not a directory", opts.Source)
	}

	var matches []string
	walkErr := filepath.WalkDir(opts.Source, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		// A symlink is reported as an irregular file by WalkDir, so this also
		// keeps the walk from following a link out of the source tree.
		if !d.Type().IsRegular() {
			return nil
		}
		ok, matchErr := filepath.Match(opts.Pattern, d.Name())
		if matchErr != nil {
			return fmt.Errorf("bad --pattern %q: %w", opts.Pattern, matchErr)
		}
		if ok {
			matches = append(matches, path)
		}
		return nil
	})
	if walkErr != nil {
		return 0, walkErr
	}
	sort.Strings(matches)

	uploaded := 0
	for _, path := range matches {
		rel, err := filepath.Rel(opts.Source, path)
		if err != nil {
			return uploaded, fmt.Errorf("resolve %s: %w", path, err)
		}
		object := gcsObjectName(opts.Prefix, rel)
		if err := uploadOneArtifact(ctx, opts, path, object); err != nil {
			return uploaded, err
		}
		uploaded++
		fmt.Fprintf(out, "gs://%s/%s\n", opts.Bucket, object)
	}
	return uploaded, nil
}

// gcsObjectName joins the prefix and a source-relative path into an object name,
// normalising Windows separators and collapsing empty segments.
func gcsObjectName(prefix, rel string) string {
	segments := make([]string, 0, 2)
	if trimmed := strings.Trim(prefix, "/"); trimmed != "" {
		segments = append(segments, trimmed)
	}
	segments = append(segments, filepath.ToSlash(rel))
	return strings.Join(segments, "/")
}

func uploadOneArtifact(ctx context.Context, opts artifactUploadOptions, path, object string) error {
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open %s: %w", path, err)
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		return fmt.Errorf("stat %s: %w", path, err)
	}

	endpoint := fmt.Sprintf("%s/upload/storage/v1/b/%s/o?uploadType=media&name=%s",
		strings.TrimSuffix(gcsUploadBaseURL, "/"),
		url.PathEscape(opts.Bucket),
		url.QueryEscape(object),
	)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, f)
	if err != nil {
		return fmt.Errorf("build request for %s: %w", object, err)
	}
	req.Header.Set("Authorization", "Bearer "+opts.Token)
	req.Header.Set("Content-Type", opts.ContentType)
	req.ContentLength = info.Size()

	client := opts.Client
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("upload %s: %w", object, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		// The body can echo request metadata, so surface the status and a short
		// excerpt rather than the whole response.
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("upload %s: %s: %s", object, resp.Status, strings.TrimSpace(string(body)))
	}
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 1<<16))
	return nil
}
