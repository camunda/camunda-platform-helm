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

package deploy

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

const DefaultImagePlatform = defaultImagePlatform

type EnterpriseImageResult struct {
	Ref    string
	OK     bool
	Detail string
}

const enterpriseImageAuditAttempts = 3

var enterpriseImageAuditRetryDelay = 2 * time.Second

func AuditEnterpriseImages(ctx context.Context, valuesFile, platform string) []EnterpriseImageResult {
	images := collectPinnedImages([]string{valuesFile})
	if len(images) == 0 {
		return nil
	}

	resolutions := resolveImagesConcurrently(ctx, images, platform)
	for attempt := 1; attempt < enterpriseImageAuditAttempts; attempt++ {
		pending, at := unresolvedImages(images, resolutions)
		if len(pending) == 0 || !waitBeforeRetry(ctx, enterpriseImageAuditRetryDelay) {
			break
		}
		for i, res := range resolveImagesConcurrently(ctx, pending, platform) {
			resolutions[at[i]] = res
		}
	}

	results := make([]EnterpriseImageResult, 0, len(resolutions))
	for _, res := range resolutions {
		results = append(results, auditResult(res))
	}
	return results
}

func unresolvedImages(images []pinnedImage, resolutions []imageResolution) ([]pinnedImage, []int) {
	var pending []pinnedImage
	var at []int
	for i, res := range resolutions {
		if res.Err != nil || res.Unverified != nil {
			pending = append(pending, images[i])
			at = append(at, i)
		}
	}
	return pending, at
}

func waitBeforeRetry(ctx context.Context, d time.Duration) bool {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}

func auditResult(res imageResolution) EnterpriseImageResult {
	cause, reason := res.Err, "not pullable"
	if cause == nil {
		cause, reason = res.Unverified, "could not be verified"
	}
	if cause == nil {
		return EnterpriseImageResult{Ref: res.Ref, OK: true}
	}
	if res.Digest != "" {
		return EnterpriseImageResult{Ref: res.Ref, Detail: fmt.Sprintf(
			"%s on %s: child manifest %s %s: %s",
			res.Ref, res.Platform, res.Digest, reason, causeText(cause))}
	}
	return EnterpriseImageResult{Ref: res.Ref, Detail: fmt.Sprintf(
		"%s on %s: %s: %s", res.Ref, res.Platform, reason, causeText(cause))}
}

func causeText(err error) string {
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		for _, line := range strings.Split(string(exit.Stderr), "\n") {
			if line = strings.TrimSpace(line); line != "" {
				return fmt.Sprintf("%v: %s", err, line)
			}
		}
	}
	return err.Error()
}
