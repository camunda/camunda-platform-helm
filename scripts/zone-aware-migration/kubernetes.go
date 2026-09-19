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

package main

import (
	"context"
	"fmt"
)

func (r runner) rollout(ctx context.Context, statefulSet string) error {
	_, err := r.command(ctx, "kubectl", "rollout", "status", "statefulset/"+statefulSet, "--namespace", r.cfg.namespace, "--timeout", r.cfg.timeout)
	if err != nil {
		return fmt.Errorf("wait for %s rollout: %w", statefulSet, err)
	}
	return nil
}

func (r runner) podValue(ctx context.Context, pod, path string) (string, error) {
	value, err := r.command(ctx, "kubectl", "get", "pod", pod, "--namespace", r.cfg.namespace, "-o", "jsonpath="+path)
	if err != nil {
		return "", fmt.Errorf("get pod %s value %s: %w", pod, path, err)
	}
	return value, nil
}

func (r runner) statefulSetValue(ctx context.Context, statefulSet, path string) (string, error) {
	value, err := r.command(ctx, "kubectl", "get", "statefulset/"+statefulSet, "--namespace", r.cfg.namespace, "-o", "jsonpath="+path)
	if err != nil {
		return "", fmt.Errorf("get StatefulSet %s value %s: %w", statefulSet, path, err)
	}
	return value, nil
}

func (r runner) assertValue(ctx context.Context, pod, path, want string) error {
	got, err := r.podValue(ctx, pod, path)
	if err != nil {
		return err
	}
	if got != want {
		return fmt.Errorf("pod %s value %s is %q, expected %q", pod, path, got, want)
	}
	return nil
}

func (r runner) assertStatefulSetValue(ctx context.Context, statefulSet, path, want string) error {
	got, err := r.statefulSetValue(ctx, statefulSet, path)
	if err != nil {
		return err
	}
	if got != want {
		return fmt.Errorf("StatefulSet %s value %s is %q, expected %q", statefulSet, path, got, want)
	}
	return nil
}

func (r runner) assertAbsent(ctx context.Context, resource string) error {
	value, err := r.command(ctx, "kubectl", "get", resource, "--namespace", r.cfg.namespace, "--ignore-not-found", "-o", "name")
	if err != nil {
		return fmt.Errorf("check absent %s: %w", resource, err)
	}
	if value != "" {
		return fmt.Errorf("resource %s still exists", resource)
	}
	return nil
}

func (r runner) assertPresent(ctx context.Context, resource string) error {
	if _, err := r.command(ctx, "kubectl", "get", resource, "--namespace", r.cfg.namespace); err != nil {
		return fmt.Errorf("required resource %s is absent: %w", resource, err)
	}
	return nil
}
