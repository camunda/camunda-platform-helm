package deploy

import (
	"os"
	"path/filepath"
	"testing"

	"gopkg.in/yaml.v3"
)

// helper: write a temp YAML file and return its path.
func writeTempYAML(t *testing.T, dir, name string, content string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

// helper: read YAML file into map.
func readYAMLMap(t *testing.T, path string) map[string]any {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := yaml.Unmarshal(data, &m); err != nil {
		t.Fatal(err)
	}
	return m
}

// helper: get a nested value from map by dot-separated path.
func getPath(m map[string]any, keys ...string) any {
	var cur any = m
	for _, k := range keys {
		mm, ok := cur.(map[string]any)
		if !ok {
			return nil
		}
		cur = mm[k]
	}
	return cur
}

// helper: get env array as slice of {name, value} maps.
func getEnvArray(t *testing.T, m map[string]any, keys ...string) []map[string]any {
	t.Helper()
	raw := getPath(m, keys...)
	arr, ok := raw.([]any)
	if !ok {
		t.Fatalf("expected []any at path %v, got %T", keys, raw)
	}
	result := make([]map[string]any, len(arr))
	for i, elem := range arr {
		mm, ok := elem.(map[string]any)
		if !ok {
			t.Fatalf("element %d is not map[string]any: %T", i, elem)
		}
		result[i] = mm
	}
	return result
}

// findEnvByName looks up an env var by name in a slice of env maps.
func findEnvByName(envs []map[string]any, name string) (map[string]any, bool) {
	for _, e := range envs {
		if e["name"] == name {
			return e, true
		}
	}
	return nil, false
}

// TestMergeYAMLFiles_EnvArrayMerge tests the core use case: merging operate.env
// and tasklist.env arrays from elasticsearch-external.yaml and rba.yaml layers.
func TestMergeYAMLFiles_EnvArrayMerge(t *testing.T) {
	dir := t.TempDir()

	// Layer 1: elasticsearch-external.yaml (persistence layer)
	esExternal := `
operate:
  env:
    - name: CAMUNDA_OPERATE_ELASTICSEARCH_INDEXPREFIX
      value: op-test-install
    - name: CAMUNDA_OPERATE_ZEEBEELASTICSEARCH_INDEXPREFIX
      value: orch-test-install
tasklist:
  env:
    - name: CAMUNDA_TASKLIST_ELASTICSEARCH_INDEXPREFIX
      value: tl-test-install
    - name: CAMUNDA_TASKLIST_ZEEBEELASTICSEARCH_INDEXPREFIX
      value: orch-test-install
optimize:
  env:
    - name: CAMUNDA_OPTIMIZE_ELASTICSEARCH_SETTINGS_INDEX_PREFIX
      value: opt-test-install
`

	// Layer 2: rba.yaml (feature layer — applied after persistence)
	rba := `
operate:
  env:
    - name: CAMUNDA_OPERATE_IDENTITY_RESOURCEPERMISSIONSENABLED
      value: 'true'
tasklist:
  env:
    - name: CAMUNDA_TASKLIST_IDENTITY_RESOURCE_PERMISSIONS_ENABLED
      value: 'true'
identity:
  env:
    - name: RESOURCE_PERMISSIONS_ENABLED
      value: 'true'
`

	f1 := writeTempYAML(t, dir, "es-external.yaml", esExternal)
	f2 := writeTempYAML(t, dir, "rba.yaml", rba)
	outPath := filepath.Join(dir, "merged.yaml")

	result, err := MergeYAMLFiles([]string{f1, f2}, outPath)
	if err != nil {
		t.Fatalf("MergeYAMLFiles failed: %v", err)
	}
	if result != outPath {
		t.Fatalf("expected output path %q, got %q", outPath, result)
	}

	m := readYAMLMap(t, outPath)

	// Verify operate.env has ALL entries from both layers (3 total).
	operateEnv := getEnvArray(t, m, "operate", "env")
	if len(operateEnv) != 3 {
		t.Fatalf("expected 3 operate.env entries, got %d: %v", len(operateEnv), operateEnv)
	}
	if _, found := findEnvByName(operateEnv, "CAMUNDA_OPERATE_ELASTICSEARCH_INDEXPREFIX"); !found {
		t.Error("missing CAMUNDA_OPERATE_ELASTICSEARCH_INDEXPREFIX in operate.env")
	}
	if _, found := findEnvByName(operateEnv, "CAMUNDA_OPERATE_ZEEBEELASTICSEARCH_INDEXPREFIX"); !found {
		t.Error("missing CAMUNDA_OPERATE_ZEEBEELASTICSEARCH_INDEXPREFIX in operate.env")
	}
	if e, found := findEnvByName(operateEnv, "CAMUNDA_OPERATE_IDENTITY_RESOURCEPERMISSIONSENABLED"); !found {
		t.Error("missing CAMUNDA_OPERATE_IDENTITY_RESOURCEPERMISSIONSENABLED in operate.env")
	} else if e["value"] != "true" {
		t.Errorf("expected 'true', got %q", e["value"])
	}

	// Verify tasklist.env has ALL entries from both layers (3 total).
	tasklistEnv := getEnvArray(t, m, "tasklist", "env")
	if len(tasklistEnv) != 3 {
		t.Fatalf("expected 3 tasklist.env entries, got %d: %v", len(tasklistEnv), tasklistEnv)
	}
	if _, found := findEnvByName(tasklistEnv, "CAMUNDA_TASKLIST_ELASTICSEARCH_INDEXPREFIX"); !found {
		t.Error("missing CAMUNDA_TASKLIST_ELASTICSEARCH_INDEXPREFIX in tasklist.env")
	}
	if _, found := findEnvByName(tasklistEnv, "CAMUNDA_TASKLIST_ZEEBEELASTICSEARCH_INDEXPREFIX"); !found {
		t.Error("missing CAMUNDA_TASKLIST_ZEEBEELASTICSEARCH_INDEXPREFIX in tasklist.env")
	}
	if _, found := findEnvByName(tasklistEnv, "CAMUNDA_TASKLIST_IDENTITY_RESOURCE_PERMISSIONS_ENABLED"); !found {
		t.Error("missing CAMUNDA_TASKLIST_IDENTITY_RESOURCE_PERMISSIONS_ENABLED in tasklist.env")
	}

	// Verify optimize.env is preserved (only in layer 1, no conflict).
	optimizeEnv := getEnvArray(t, m, "optimize", "env")
	if len(optimizeEnv) != 1 {
		t.Fatalf("expected 1 optimize.env entry, got %d", len(optimizeEnv))
	}
	if _, found := findEnvByName(optimizeEnv, "CAMUNDA_OPTIMIZE_ELASTICSEARCH_SETTINGS_INDEX_PREFIX"); !found {
		t.Error("missing CAMUNDA_OPTIMIZE_ELASTICSEARCH_SETTINGS_INDEX_PREFIX in optimize.env")
	}

	// Verify identity.env is present (only in layer 2, no conflict).
	identityEnv := getEnvArray(t, m, "identity", "env")
	if len(identityEnv) != 1 {
		t.Fatalf("expected 1 identity.env entry, got %d", len(identityEnv))
	}
}

// TestMergeYAMLFiles_ScalarOverride verifies that scalar values from later
// layers override earlier ones (standard Helm behaviour).
func TestMergeYAMLFiles_ScalarOverride(t *testing.T) {
	dir := t.TempDir()

	f1 := writeTempYAML(t, dir, "base.yaml", `
global:
  image:
    tag: "8.7.0"
  elasticsearch:
    prefix: base-prefix
`)
	f2 := writeTempYAML(t, dir, "override.yaml", `
global:
  image:
    tag: "8.7.1"
`)
	outPath := filepath.Join(dir, "merged.yaml")

	_, err := MergeYAMLFiles([]string{f1, f2}, outPath)
	if err != nil {
		t.Fatal(err)
	}

	m := readYAMLMap(t, outPath)
	tag := getPath(m, "global", "image", "tag")
	if tag != "8.7.1" {
		t.Errorf("expected tag '8.7.1', got %v", tag)
	}
	// Verify non-overridden values are preserved.
	prefix := getPath(m, "global", "elasticsearch", "prefix")
	if prefix != "base-prefix" {
		t.Errorf("expected prefix 'base-prefix', got %v", prefix)
	}
}

// TestMergeYAMLFiles_EnvOverrideSameName verifies that when two layers define
// the same env var name, the later layer's value wins.
func TestMergeYAMLFiles_EnvOverrideSameName(t *testing.T) {
	dir := t.TempDir()

	f1 := writeTempYAML(t, dir, "layer1.yaml", `
operate:
  env:
    - name: FOO
      value: bar
    - name: BAZ
      value: qux
`)
	f2 := writeTempYAML(t, dir, "layer2.yaml", `
operate:
  env:
    - name: FOO
      value: overridden
    - name: NEW
      value: added
`)
	outPath := filepath.Join(dir, "merged.yaml")

	_, err := MergeYAMLFiles([]string{f1, f2}, outPath)
	if err != nil {
		t.Fatal(err)
	}

	m := readYAMLMap(t, outPath)
	envs := getEnvArray(t, m, "operate", "env")

	// Should have 3 entries: FOO (overridden), BAZ (kept), NEW (added).
	if len(envs) != 3 {
		t.Fatalf("expected 3 env entries, got %d: %v", len(envs), envs)
	}

	foo, found := findEnvByName(envs, "FOO")
	if !found {
		t.Fatal("missing FOO")
	}
	if foo["value"] != "overridden" {
		t.Errorf("expected FOO='overridden', got %q", foo["value"])
	}

	if _, found := findEnvByName(envs, "BAZ"); !found {
		t.Error("missing BAZ")
	}
	if _, found := findEnvByName(envs, "NEW"); !found {
		t.Error("missing NEW")
	}
}

// TestMergeYAMLFiles_SingleFile verifies short-circuit for single file.
func TestMergeYAMLFiles_SingleFile(t *testing.T) {
	dir := t.TempDir()
	f1 := writeTempYAML(t, dir, "only.yaml", `foo: bar`)
	result, err := MergeYAMLFiles([]string{f1}, filepath.Join(dir, "merged.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if result != f1 {
		t.Errorf("expected original file path %q, got %q", f1, result)
	}
}

// TestMergeYAMLFiles_ThreeLayers tests merge across 3 layers (common, persistence, feature).
func TestMergeYAMLFiles_ThreeLayers(t *testing.T) {
	dir := t.TempDir()

	f1 := writeTempYAML(t, dir, "common.yaml", `
global:
  image:
    tag: "8.7.0"
operate:
  env:
    - name: COMMON_VAR
      value: from-common
`)
	f2 := writeTempYAML(t, dir, "persistence.yaml", `
operate:
  env:
    - name: CAMUNDA_OPERATE_ELASTICSEARCH_INDEXPREFIX
      value: op-prefix
`)
	f3 := writeTempYAML(t, dir, "feature.yaml", `
operate:
  env:
    - name: CAMUNDA_OPERATE_IDENTITY_RESOURCEPERMISSIONSENABLED
      value: 'true'
`)
	outPath := filepath.Join(dir, "merged.yaml")

	_, err := MergeYAMLFiles([]string{f1, f2, f3}, outPath)
	if err != nil {
		t.Fatal(err)
	}

	m := readYAMLMap(t, outPath)
	envs := getEnvArray(t, m, "operate", "env")

	// All 3 env vars from 3 different layers should be present.
	if len(envs) != 3 {
		t.Fatalf("expected 3 env entries, got %d: %v", len(envs), envs)
	}
	for _, name := range []string{
		"COMMON_VAR",
		"CAMUNDA_OPERATE_ELASTICSEARCH_INDEXPREFIX",
		"CAMUNDA_OPERATE_IDENTITY_RESOURCEPERMISSIONSENABLED",
	} {
		if _, found := findEnvByName(envs, name); !found {
			t.Errorf("missing %s in operate.env", name)
		}
	}

	// Verify global.image.tag is preserved through the merge.
	tag := getPath(m, "global", "image", "tag")
	if tag != "8.7.0" {
		t.Errorf("expected tag '8.7.0', got %v", tag)
	}
}

// TestMergeYAMLFiles_EmptyFile verifies that an empty YAML file doesn't break the merge.
func TestMergeYAMLFiles_EmptyFile(t *testing.T) {
	dir := t.TempDir()

	f1 := writeTempYAML(t, dir, "base.yaml", `foo: bar`)
	f2 := writeTempYAML(t, dir, "empty.yaml", ``)
	outPath := filepath.Join(dir, "merged.yaml")

	_, err := MergeYAMLFiles([]string{f1, f2}, outPath)
	if err != nil {
		t.Fatal(err)
	}

	m := readYAMLMap(t, outPath)
	if m["foo"] != "bar" {
		t.Errorf("expected foo='bar', got %v", m["foo"])
	}
}

// TestMergeYAMLFiles_DeepNestedMaps verifies deep recursive map merge.
func TestMergeYAMLFiles_DeepNestedMaps(t *testing.T) {
	dir := t.TempDir()

	f1 := writeTempYAML(t, dir, "base.yaml", `
a:
  b:
    c:
      d: 1
      e: 2
`)
	f2 := writeTempYAML(t, dir, "override.yaml", `
a:
  b:
    c:
      d: 10
    f: new
`)
	outPath := filepath.Join(dir, "merged.yaml")

	_, err := MergeYAMLFiles([]string{f1, f2}, outPath)
	if err != nil {
		t.Fatal(err)
	}

	m := readYAMLMap(t, outPath)
	d := getPath(m, "a", "b", "c", "d")
	e := getPath(m, "a", "b", "c", "e")
	f := getPath(m, "a", "b", "f")

	if d != 10 {
		t.Errorf("expected d=10, got %v", d)
	}
	if e != 2 {
		t.Errorf("expected e=2, got %v", e)
	}
	if f != "new" {
		t.Errorf("expected f='new', got %v", f)
	}
}

// TestMergeLayeredValues_ShortCircuit verifies MergeLayeredValues returns
// the original slice for 0 or 1 files.
func TestMergeLayeredValues_ShortCircuit(t *testing.T) {
	result, err := MergeLayeredValues(nil, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if len(result) != 0 {
		t.Errorf("expected 0 files, got %d", len(result))
	}

	result, err = MergeLayeredValues([]string{"/tmp/single.yaml"}, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if len(result) != 1 || result[0] != "/tmp/single.yaml" {
		t.Errorf("expected [/tmp/single.yaml], got %v", result)
	}
}

// TestMergeYAMLFiles_RealWorldScenario uses the exact YAML structures from
// the actual elasticsearch-external.yaml and rba.yaml files to verify the
// fix works for the real-world bug.
func TestMergeYAMLFiles_RealWorldScenario(t *testing.T) {
	dir := t.TempDir()

	// Exact structure from charts/camunda-platform-8.7/.../elasticsearch-external.yaml
	// with env vars already substituted (as they would be after values.Process)
	esExternal := `
global:
  elasticsearch:
    enabled: true
    external: true
    prefix: orch-keycloak-rba-jnsrqgb9-install
    auth:
      username: elastic
      existingSecret: infra-credentials
      existingSecretKey: elasticsearch-password
    url:
      protocol: https
      host: elasticsearch-21-6-3.ci.distro.ultrawombat.com
      port: 443
operate:
  env:
    - name: CAMUNDA_OPERATE_ELASTICSEARCH_INDEXPREFIX
      value: op-keycloak-rba-jnsrqgb9-install
    - name: CAMUNDA_OPERATE_ZEEBEELASTICSEARCH_INDEXPREFIX
      value: orch-keycloak-rba-jnsrqgb9-install
optimize:
  env:
    - name: CAMUNDA_OPTIMIZE_ELASTICSEARCH_SETTINGS_INDEX_PREFIX
      value: opt-keycloak-rba-jnsrqgb9-install
    - name: CAMUNDA_OPTIMIZE_ZEEBE_NAME
      value: orch-keycloak-rba-jnsrqgb9-install
zeebe:
  broker:
    exporters:
      elasticsearch:
        index:
          prefix: "orch-keycloak-rba-jnsrqgb9-install"
  env:
    - name: ZEEBE_BROKER_EXPORTERS_ELASTICSEARCH_ARGS_INDEX_PREFIX
      value: orch-keycloak-rba-jnsrqgb9-install
tasklist:
  env:
    - name: CAMUNDA_TASKLIST_ELASTICSEARCH_INDEXPREFIX
      value: tl-keycloak-rba-jnsrqgb9-install
    - name: CAMUNDA_TASKLIST_ZEEBEELASTICSEARCH_INDEXPREFIX
      value: orch-keycloak-rba-jnsrqgb9-install
elasticsearch:
  enabled: false
`

	// Exact structure from charts/camunda-platform-8.7/.../rba.yaml
	rba := `
tasklist:
  env:
    - name: CAMUNDA_TASKLIST_IDENTITY_RESOURCE_PERMISSIONS_ENABLED
      value: 'true'
operate:
  env:
    - name: CAMUNDA_OPERATE_IDENTITY_RESOURCEPERMISSIONSENABLED
      value: 'true'
identity:
  env:
    - name: RESOURCE_PERMISSIONS_ENABLED
      value: 'true'
`

	f1 := writeTempYAML(t, dir, "es-external.yaml", esExternal)
	f2 := writeTempYAML(t, dir, "rba.yaml", rba)
	outPath := filepath.Join(dir, "merged.yaml")

	_, err := MergeYAMLFiles([]string{f1, f2}, outPath)
	if err != nil {
		t.Fatalf("merge failed: %v", err)
	}

	m := readYAMLMap(t, outPath)

	// THE BUG: Without merge, Helm would replace operate.env entirely with rba.yaml's
	// single entry, losing CAMUNDA_OPERATE_ELASTICSEARCH_INDEXPREFIX and
	// CAMUNDA_OPERATE_ZEEBEELASTICSEARCH_INDEXPREFIX.

	// Verify operate.env has all 3 entries.
	operateEnv := getEnvArray(t, m, "operate", "env")
	expectedOperate := []string{
		"CAMUNDA_OPERATE_ELASTICSEARCH_INDEXPREFIX",
		"CAMUNDA_OPERATE_ZEEBEELASTICSEARCH_INDEXPREFIX",
		"CAMUNDA_OPERATE_IDENTITY_RESOURCEPERMISSIONSENABLED",
	}
	if len(operateEnv) != len(expectedOperate) {
		t.Fatalf("operate.env: expected %d entries, got %d: %v",
			len(expectedOperate), len(operateEnv), operateEnv)
	}
	for _, name := range expectedOperate {
		if _, found := findEnvByName(operateEnv, name); !found {
			t.Errorf("operate.env: missing %s", name)
		}
	}

	// Verify the index prefix VALUE is correct (from es-external layer).
	indexPrefix, _ := findEnvByName(operateEnv, "CAMUNDA_OPERATE_ELASTICSEARCH_INDEXPREFIX")
	if indexPrefix["value"] != "op-keycloak-rba-jnsrqgb9-install" {
		t.Errorf("operate index prefix: expected 'op-keycloak-rba-jnsrqgb9-install', got %q",
			indexPrefix["value"])
	}

	// Verify tasklist.env has all 3 entries.
	tasklistEnv := getEnvArray(t, m, "tasklist", "env")
	expectedTasklist := []string{
		"CAMUNDA_TASKLIST_ELASTICSEARCH_INDEXPREFIX",
		"CAMUNDA_TASKLIST_ZEEBEELASTICSEARCH_INDEXPREFIX",
		"CAMUNDA_TASKLIST_IDENTITY_RESOURCE_PERMISSIONS_ENABLED",
	}
	if len(tasklistEnv) != len(expectedTasklist) {
		t.Fatalf("tasklist.env: expected %d entries, got %d: %v",
			len(expectedTasklist), len(tasklistEnv), tasklistEnv)
	}
	for _, name := range expectedTasklist {
		if _, found := findEnvByName(tasklistEnv, name); !found {
			t.Errorf("tasklist.env: missing %s", name)
		}
	}

	// Verify optimize.env is preserved (no conflict — only in es-external).
	optimizeEnv := getEnvArray(t, m, "optimize", "env")
	if len(optimizeEnv) != 2 {
		t.Fatalf("optimize.env: expected 2 entries, got %d", len(optimizeEnv))
	}

	// Verify identity.env is present (only in rba).
	identityEnv := getEnvArray(t, m, "identity", "env")
	if len(identityEnv) != 1 {
		t.Fatalf("identity.env: expected 1 entry, got %d", len(identityEnv))
	}

	// Verify zeebe.env is preserved (only in es-external).
	zeebeEnv := getEnvArray(t, m, "zeebe", "env")
	if len(zeebeEnv) != 1 {
		t.Fatalf("zeebe.env: expected 1 entry, got %d", len(zeebeEnv))
	}

	// Verify global map is deep-merged (not replaced).
	prefix := getPath(m, "global", "elasticsearch", "prefix")
	if prefix != "orch-keycloak-rba-jnsrqgb9-install" {
		t.Errorf("global.elasticsearch.prefix: expected 'orch-keycloak-rba-jnsrqgb9-install', got %v", prefix)
	}

	// Verify elasticsearch.enabled is preserved.
	esEnabled := getPath(m, "elasticsearch", "enabled")
	if esEnabled != false {
		t.Errorf("elasticsearch.enabled: expected false, got %v", esEnabled)
	}
}
