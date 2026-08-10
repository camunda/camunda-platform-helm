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

// Package orphans identifies PersistentVolumeClaims left behind by an upgrade.
//
// Kubernetes never deletes a claim created from a StatefulSet's
// volumeClaimTemplates, so removing a subchart strands its storage: the
// workload goes, the data stays, and nothing surfaces it. Detection is by
// reference rather than by label, because claims created by the StatefulSet
// controller carry the workload's labels, not Helm's release metadata.
package orphans

import (
	"sort"
	"strconv"
	"strings"
)

// StatefulSetRef describes a StatefulSet's claim-generating templates.
type StatefulSetRef struct {
	Name string
	// ClaimTemplates are volumeClaimTemplate names, which combine with the
	// StatefulSet name and an ordinal to form claim names.
	ClaimTemplates []string
	// Replicas bounds the ordinals considered live.
	Replicas int32
}

// Inventory is the namespace state orphan detection reads.
type Inventory struct {
	// Claims is every PersistentVolumeClaim in the namespace.
	Claims []string
	// PodClaims are claims referenced by a pod's volumes.
	PodClaims []string
	// StatefulSets generate claims whose pods may not currently exist, so a
	// scaled-down workload still counts as referencing its storage.
	StatefulSets []StatefulSetRef
}

// Orphan is a claim nothing references.
type Orphan struct {
	Claim string `json:"claim"`
}

// Detect returns claims that no pod references and no StatefulSet would
// generate, sorted by name.
func Detect(inv Inventory) []Orphan {
	referenced := map[string]bool{}
	for _, c := range inv.PodClaims {
		referenced[c] = true
	}
	var out []Orphan
	for _, c := range inv.Claims {
		if !referenced[c] && !generatedByStatefulSet(c, inv.StatefulSets) {
			out = append(out, Orphan{Claim: c})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Claim < out[j].Claim })
	return out
}

func generatedByStatefulSet(claim string, sets []StatefulSetRef) bool {
	for _, sts := range sets {
		for _, tpl := range sts.ClaimTemplates {
			ordinal := strings.TrimPrefix(claim, tpl+"-"+sts.Name+"-")
			if ordinal != claim {
				if _, err := strconv.ParseUint(ordinal, 10, 32); err == nil {
					return true
				}
			}
		}
	}
	return false
}

// Names returns just the claim names.
func Names(o []Orphan) []string {
	out := make([]string, 0, len(o))
	for _, x := range o {
		out = append(out, x.Claim)
	}
	return out
}

// Appeared returns orphans present in after but not before, so an upgrade is
// judged on the storage it strands rather than on storage that was already
// stranded when it started.
func Appeared(before, after []Orphan) []Orphan {
	was := map[string]bool{}
	for _, o := range before {
		was[o.Claim] = true
	}
	var out []Orphan
	for _, o := range after {
		if !was[o.Claim] {
			out = append(out, o)
		}
	}
	return out
}
