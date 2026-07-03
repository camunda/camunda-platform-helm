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
	"flag"
	"fmt"

	"scripts/camunda-core/pkg/harbor"
)

// runHarborTag performs Harbor artifact-tag operations. The API base, repository
// path, and credentials (HARBOR_REGISTRY_USER / HARBOR_REGISTRY_PASSWORD env) come
// from the caller; this command owns the tag decision logic.
//
//	harbor-tag <op> --api <base> --repo <path> [--dry-run] [op flags]
//
// ops:
//
//	digest --ref <r>                         print the artifact digest for a reference
//	add    --digest <d> --tag <t>            add a tag (fails on non-2xx)
//	delete --ref <r> --tag <t> [--ignore-missing]   untag via the tag endpoint
//	ensure --digest <d> --tag <t> [--move]   idempotently point a tag at a digest
//
// All deletes use the deleteTag endpoint (never deleteArtifact); see the package doc.
func runHarborTag(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("harbor-tag requires an op: digest|add|delete|ensure")
	}
	op := args[0]
	fs := flag.NewFlagSet("harbor-tag "+op, flag.ContinueOnError)
	var (
		api           string
		repo          string
		dryRun        bool
		digest        string
		tag           string
		ref           string
		move          bool
		ignoreMissing bool
	)
	fs.StringVar(&api, "api", "", "Harbor API base, e.g. https://registry.camunda.cloud/api/v2.0")
	fs.StringVar(&repo, "repo", "", "repository path, e.g. projects/<project>/repositories/<name>")
	fs.BoolVar(&dryRun, "dry-run", false, "print intended mutations without executing them")
	fs.StringVar(&digest, "digest", "", "artifact digest the tag should point at (add/ensure)")
	fs.StringVar(&tag, "tag", "", "tag name (add/delete/ensure)")
	fs.StringVar(&ref, "ref", "", "artifact reference: digest or tag (digest/delete)")
	fs.BoolVar(&move, "move", false, "rolling tag: move it even if it currently points elsewhere (ensure)")
	fs.BoolVar(&ignoreMissing, "ignore-missing", false, "tolerate a missing tag (delete)")
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}
	if api == "" || repo == "" {
		return fmt.Errorf("--api and --repo are required")
	}

	c := harbor.New(api, repo, dryRun)

	switch op {
	case "digest":
		if ref == "" {
			return fmt.Errorf("digest: --ref is required")
		}
		d, err := c.Digest(ref)
		if err != nil {
			return err
		}
		fmt.Println(d)
		return nil
	case "add":
		if digest == "" || tag == "" {
			return fmt.Errorf("add: --digest and --tag are required")
		}
		return c.AddTag(digest, tag)
	case "delete":
		if ref == "" || tag == "" {
			return fmt.Errorf("delete: --ref and --tag are required")
		}
		return c.DeleteTag(ref, tag, ignoreMissing)
	case "ensure":
		if digest == "" || tag == "" {
			return fmt.Errorf("ensure: --digest and --tag are required")
		}
		return c.EnsureTag(digest, tag, move)
	default:
		return fmt.Errorf("unknown harbor-tag op %q (want digest|add|delete|ensure)", op)
	}
}
