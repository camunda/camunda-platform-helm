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

package entra

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"

	"scripts/camunda-core/pkg/logging"
)

// qaNamespacePrefix is the namespace prefix the QA flows deploy with.
const qaNamespacePrefix = "qa-"

// qaEntraShortname is the shortname of the Entra QA scenarios.
const qaEntraShortname = "qaen"

// sweepPrefix is the display-name prefix the Graph listing filters on.
var sweepPrefix = loginAppDisplayName(qaNamespacePrefix)

// sweepNamePattern narrows the listing to namespaces that also carry the
// Entra QA shortname.
var sweepNamePattern = regexp.MustCompile(
	`^` + regexp.QuoteMeta(sweepPrefix) + `[a-z0-9-]*` + regexp.QuoteMeta(qaEntraShortname),
)

// sweepPageSize is the Graph page size for the application listing.
const sweepPageSize = 999

// minSweepMaxAge is the lowest --max-age a sweep accepts.
const minSweepMaxAge = 12 * time.Hour

// SweepOptions configures a sweep of leaked QA Entra login app registrations.
type SweepOptions struct {
	// Options supplies the Graph credentials and HTTP client; its Namespace is
	// unused.
	Options

	// MaxAge keeps app registrations younger than this, at least minSweepMaxAge.
	MaxAge time.Duration

	// DryRun lists what would be deleted without deleting anything.
	DryRun bool

	// Now overrides the clock the age comparison uses. Zero means time.Now.
	Now time.Time
}

// SweptApp is one QA login app registration the sweep matched.
type SweptApp struct {
	DisplayName string    `json:"displayName"`
	AppID       string    `json:"appId"`
	ObjectID    string    `json:"objectId"`
	CreatedAt   time.Time `json:"createdAt"`
	// Error is why a delete failed, empty when it succeeded.
	Error string `json:"error,omitempty"`
}

// SweepResult reports what a sweep found and what it did about it.
type SweepResult struct {
	// DryRun repeats the mode the sweep ran in.
	DryRun bool `json:"dryRun"`
	// Scanned counts the app registrations the prefix query returned.
	Scanned int `json:"scanned"`
	// Candidates are the matched apps older than MaxAge.
	Candidates []SweptApp `json:"candidates"`
	// Deleted and Failed are empty on a dry run.
	Deleted []SweptApp `json:"deleted"`
	Failed  []SweptApp `json:"failed"`
}

// graphApplication is the subset of a Graph application a sweep reads.
type graphApplication struct {
	ID          string `json:"id"`
	AppID       string `json:"appId"`
	DisplayName string `json:"displayName"`
	CreatedAt   string `json:"createdDateTime"`
}

// SweepLoginApps deletes the login app registrations that match
// sweepNamePattern and are older than opts.MaxAge. On a dry run it reports the
// same candidates without deleting any.
func SweepLoginApps(ctx context.Context, opts SweepOptions) (SweepResult, error) {
	result := SweepResult{DryRun: opts.DryRun}

	if err := resolveCredentials(&opts.Options); err != nil {
		return result, err
	}
	if opts.MaxAge < minSweepMaxAge {
		return result, fmt.Errorf("max age must be at least %s, got %s", minSweepMaxAge, opts.MaxAge)
	}
	now := opts.Now
	if now.IsZero() {
		now = time.Now()
	}
	cutoff := now.Add(-opts.MaxAge).UTC()

	client := httpClient(&opts.Options)
	token, err := acquireBearerToken(ctx, &opts.Options)
	if err != nil {
		return result, fmt.Errorf("obtain bearer token: %w", err)
	}

	apps, err := listAppsByPrefix(ctx, client, token, sweepPrefix)
	if err != nil {
		return result, err
	}
	result.Scanned = len(apps)

	logging.Logger.Info().
		Int("scanned", len(apps)).
		Str("cutoff", cutoff.Format(time.RFC3339)).
		Bool("dryRun", opts.DryRun).
		Msg("Sweeping QA Entra login app registrations")

	for _, app := range apps {
		if !sweepNamePattern.MatchString(app.DisplayName) {
			continue
		}
		createdAt, err := time.Parse(time.RFC3339, app.CreatedAt)
		if err != nil {
			logging.Logger.Warn().
				Str("displayName", app.DisplayName).
				Str("createdDateTime", app.CreatedAt).
				Msg("Skipping app with unreadable creation time")
			continue
		}
		if !createdAt.Before(cutoff) {
			continue
		}
		result.Candidates = append(result.Candidates, SweptApp{
			DisplayName: app.DisplayName,
			AppID:       app.AppID,
			ObjectID:    app.ID,
			CreatedAt:   createdAt.UTC(),
		})
	}

	for _, candidate := range result.Candidates {
		log := logging.Logger.With().
			Str("displayName", candidate.DisplayName).
			Str("createdAt", candidate.CreatedAt.Format(time.RFC3339)).
			Logger()

		if opts.DryRun {
			log.Info().Msg("Would delete orphaned Entra login app (dry run)")
			continue
		}

		statusCode, err := graphDelete(ctx, client, token, "/applications/"+candidate.ObjectID)
		switch {
		case err != nil:
			candidate.Error = err.Error()
			log.Warn().Err(err).Msg("Delete request failed")
			result.Failed = append(result.Failed, candidate)
		case statusCode == http.StatusNoContent || statusCode == http.StatusNotFound:
			log.Info().Int("status", statusCode).Msg("Deleted orphaned Entra login app")
			result.Deleted = append(result.Deleted, candidate)
		default:
			candidate.Error = fmt.Sprintf("unexpected status %d", statusCode)
			log.Warn().Int("status", statusCode).Msg("Unexpected status when deleting Entra app")
			result.Failed = append(result.Failed, candidate)
		}
	}

	return result, nil
}

// listAppsByPrefix returns every app registration whose display name starts
// with prefix, following Graph's paging links.
func listAppsByPrefix(ctx context.Context, client *http.Client, token, prefix string) ([]graphApplication, error) {
	path := fmt.Sprintf(
		"/applications?$filter=startswith(displayName,'%s')&$select=id,appId,displayName,createdDateTime&$top=%d",
		prefix, sweepPageSize,
	)

	var apps []graphApplication
	for path != "" {
		body, err := graphGet(ctx, client, token, path)
		if err != nil {
			return nil, fmt.Errorf("list app registrations: %w", err)
		}

		var page struct {
			Value    []graphApplication `json:"value"`
			NextLink string             `json:"@odata.nextLink"`
		}
		if err := json.Unmarshal(body, &page); err != nil {
			return nil, fmt.Errorf("parse app registration listing: %w", err)
		}
		apps = append(apps, page.Value...)

		if page.NextLink == "" {
			break
		}
		if !strings.HasPrefix(page.NextLink, graphBaseURL) {
			return nil, fmt.Errorf("unexpected paging link %q", page.NextLink)
		}
		path = strings.TrimPrefix(page.NextLink, graphBaseURL)
	}

	return apps, nil
}
