// Copyright 2026 Camunda Services GmbH
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package kube

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"scripts/camunda-core/pkg/orphans"
)

// ClaimInventory reads the namespace state that orphan detection needs: every
// claim, the claims pods currently mount, and the templates each StatefulSet
// would generate.
func (c *Client) ClaimInventory(ctx context.Context, namespace string) (orphans.Inventory, error) {
	var inv orphans.Inventory

	pvcs, err := c.clientset.CoreV1().PersistentVolumeClaims(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return inv, fmt.Errorf("list persistentvolumeclaims in %s: %w", namespace, err)
	}
	for _, p := range pvcs.Items {
		inv.Claims = append(inv.Claims, p.Name)
	}

	pods, err := c.clientset.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return inv, fmt.Errorf("list pods in %s: %w", namespace, err)
	}
	for _, p := range pods.Items {
		for _, v := range p.Spec.Volumes {
			if v.PersistentVolumeClaim != nil {
				inv.PodClaims = append(inv.PodClaims, v.PersistentVolumeClaim.ClaimName)
			}
		}
	}

	sts, err := c.clientset.AppsV1().StatefulSets(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return inv, fmt.Errorf("list statefulsets in %s: %w", namespace, err)
	}
	for _, s := range sts.Items {
		ref := orphans.StatefulSetRef{Name: s.Name}
		if s.Spec.Replicas != nil {
			ref.Replicas = *s.Spec.Replicas
		}
		for _, t := range s.Spec.VolumeClaimTemplates {
			ref.ClaimTemplates = append(ref.ClaimTemplates, t.Name)
		}
		if len(ref.ClaimTemplates) > 0 {
			inv.StatefulSets = append(inv.StatefulSets, ref)
		}
	}

	return inv, nil
}
