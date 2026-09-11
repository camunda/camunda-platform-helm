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
	"errors"
	"fmt"
	"path/filepath"
)

var errNamespaceExists = errors.New("namespace already exists")

type runner struct {
	cfg     config
	command commandFunc
}

type ownership struct {
	namespace bool
	release   bool
}

func (r runner) run(ctx context.Context) (err error) {
	owned := ownership{}
	defer func() { err = errors.Join(err, r.cleanup(context.WithoutCancel(ctx), owned)) }()

	existing, err := r.command(ctx, "kubectl", "get", "namespace", r.cfg.namespace, "--ignore-not-found", "-o", "name")
	if err != nil {
		return fmt.Errorf("check namespace: %w", err)
	}
	if existing != "" {
		return fmt.Errorf("%w: %s", errNamespaceExists, r.cfg.namespace)
	}
	if _, err := r.command(ctx, "kubectl", "create", "namespace", r.cfg.namespace); err != nil {
		return fmt.Errorf("create namespace: %w", err)
	}
	owned.namespace = true

	if _, err := r.command(ctx, "helm", r.installArgs()...); err != nil {
		return fmt.Errorf("install numbered cluster: %w", err)
	}
	owned.release = true
	if err := r.verifyMigration(ctx); err != nil {
		return err
	}
	return nil
}

func (r runner) verifyMigration(ctx context.Context) error {
	numberedPod := r.cfg.release + "-zeebe-0"
	zonedPod := r.cfg.release + "-zeebe-zone-a-0"
	if err := r.rollout(ctx, r.cfg.release+"-zeebe"); err != nil {
		return err
	}
	uidBefore, err := r.podValue(ctx, numberedPod, "{.metadata.uid}")
	if err != nil {
		return fmt.Errorf("numbered pod before migration: %w", err)
	}
	if uidBefore == "" {
		return fmt.Errorf("numbered pod %s has no UID", numberedPod)
	}
	replicasBefore, err := r.statefulSetValue(ctx, r.cfg.release+"-zeebe", "{.spec.replicas}")
	if err != nil {
		return err
	}
	if _, err := r.command(ctx, "helm", r.migrationArgs(true)...); err != nil {
		return fmt.Errorf("enter migration: %w", err)
	}
	if err := r.rollout(ctx, r.cfg.release+"-zeebe-zone-a"); err != nil {
		return err
	}
	if err := r.rollout(ctx, r.cfg.release+"-zeebe"); err != nil {
		return err
	}
	uidAfter, err := r.podValue(ctx, numberedPod, "{.metadata.uid}")
	if err != nil {
		return fmt.Errorf("retained pod after migration: %w", err)
	}
	if uidAfter != uidBefore {
		return fmt.Errorf("retained pod UID changed from %q to %q", uidBefore, uidAfter)
	}
	if err := r.assertValue(ctx, numberedPod, "{.status.containerStatuses[0].restartCount}", "0"); err != nil {
		return err
	}
	if err := r.assertStatefulSetValue(ctx, r.cfg.release+"-zeebe", "{.spec.replicas}", replicasBefore); err != nil {
		return err
	}
	if err := r.assertValue(ctx, zonedPod, "{.status.phase}", "Running"); err != nil {
		return err
	}
	zonedUID, err := r.podValue(ctx, zonedPod, "{.metadata.uid}")
	if err != nil {
		return fmt.Errorf("zoned pod before cleanup: %w", err)
	}
	if zonedUID == "" {
		return fmt.Errorf("zoned pod %s has no UID", zonedPod)
	}
	if _, err := r.command(ctx, "helm", r.migrationArgs(false)...); err != nil {
		return fmt.Errorf("leave migration: %w", err)
	}
	if _, err := r.command(ctx, "kubectl", "wait", "--for=delete", "statefulset/"+r.cfg.release+"-zeebe", "--namespace", r.cfg.namespace, "--timeout", r.cfg.timeout); err != nil {
		return fmt.Errorf("wait for numbered StatefulSet deletion: %w", err)
	}
	if err := r.rollout(ctx, r.cfg.release+"-zeebe-zone-a"); err != nil {
		return err
	}
	if err := r.assertAbsent(ctx, "statefulset/"+r.cfg.release+"-zeebe"); err != nil {
		return err
	}
	if err := r.assertValue(ctx, zonedPod, "{.metadata.uid}", zonedUID); err != nil {
		return err
	}
	return r.assertPresent(ctx, "pvc/data-"+numberedPod)
}

func (r runner) cleanup(ctx context.Context, owned ownership) error {
	var cleanupErr error
	if owned.release {
		_, err := r.command(ctx, "helm", "uninstall", r.cfg.release, "--namespace", r.cfg.namespace)
		cleanupErr = errors.Join(cleanupErr, err)
	}
	if owned.namespace {
		_, err := r.command(ctx, "kubectl", "delete", "namespace", r.cfg.namespace, "--wait=false")
		cleanupErr = errors.Join(cleanupErr, err)
	}
	return cleanupErr
}

func (r runner) installArgs() []string {
	return []string{"install", r.cfg.release, r.cfg.chartDir, "--namespace", r.cfg.namespace, "--values", filepath.Join(r.cfg.scenarioDir, "values-numbered.yaml"), "--timeout", r.cfg.timeout}
}

func (r runner) migrationArgs(keep bool) []string {
	args := []string{"upgrade", r.cfg.release, r.cfg.chartDir, "--namespace", r.cfg.namespace, "--values", filepath.Join(r.cfg.scenarioDir, "values-numbered.yaml"), "--values", filepath.Join(r.cfg.scenarioDir, "values-migration.yaml")}
	if !keep {
		args = append(args, "--set", "orchestration.multiregion.keepUnzonedBrokers=false", "--set", "orchestration.multiregion.regions=1", "--set-string", "orchestration.clusterSize=1", "--set-string", "orchestration.replicationFactor=1")
	}
	return append(args, "--timeout", r.cfg.timeout)
}
