package config

import "scripts/deploy-camunda/internal/util"

// CommonFields defines the core fields shared between configuration sources.
// This type documents the common fields and provides utility methods for merging.
// Note: We don't use struct embedding directly because YAML/mapstructure tags
// differ between DeploymentConfig (yaml) and RuntimeFlags (CLI), and Go's
// struct embedding doesn't flatten properly for YAML unmarshaling.
type CommonFields struct {
	// Chart identification
	Chart     string
	ChartPath string
	Version   string

	// Deployment identifiers
	Namespace string
	Release   string
	Scenario  string

	// Scenario configuration
	ScenarioPath string
	Auth         string

	// Environment settings
	Platform string
	LogLevel string
	Flow     string
	EnvFile  string

	// Elasticsearch index prefixes
	KeycloakRealm            string
	OptimizeIndexPrefix      string
	OrchestrationIndexPrefix string
	TasklistIndexPrefix      string
	OperateIndexPrefix       string

	// Networking
	IngressSubdomain string
	IngressHostname  string

	// Docker registry
	DockerUsername string
	DockerPassword string

	// Secrets
	VaultSecretMapping string

	// Repository paths
	RepoRoot     string
	ScenarioRoot string
	ValuesPreset string

	// Output
	RenderOutputDir string

	// Lists
	ExtraValues []string
}

// FieldMerger provides a fluent interface for merging configuration fields.
type FieldMerger struct {
	target *RuntimeFlags
}

// NewFieldMerger creates a new FieldMerger for the target RuntimeFlags.
func NewFieldMerger(target *RuntimeFlags) *FieldMerger {
	return &FieldMerger{target: target}
}

// MergeStrings merges multiple string field pairs in a fluent manner.
// Each pair is (targetField, depValue, rootValue).
func (m *FieldMerger) MergeStrings(merges ...StringMerge) *FieldMerger {
	for _, merge := range merges {
		if util.IsEmpty(*merge.Target) {
			*merge.Target = util.FirstNonEmpty(merge.DepVal, merge.RootVal)
		}
	}
	return m
}

// MergeBools merges multiple boolean field pairs.
func (m *FieldMerger) MergeBools(merges ...BoolMerge) *FieldMerger {
	for _, merge := range merges {
		if merge.DepVal != nil {
			*merge.Target = *merge.DepVal
		} else if merge.RootVal != nil {
			*merge.Target = *merge.RootVal
		}
	}
	return m
}

// MergeSlices merges multiple slice field pairs.
func (m *FieldMerger) MergeSlices(merges ...SliceMerge) *FieldMerger {
	for _, merge := range merges {
		if len(*merge.Target) == 0 {
			if len(merge.DepVal) > 0 {
				*merge.Target = append(*merge.Target, merge.DepVal...)
			} else if len(merge.RootVal) > 0 {
				*merge.Target = append(*merge.Target, merge.RootVal...)
			}
		}
	}
	return m
}

// StringMerge represents a string field merge operation.
type StringMerge struct {
	Target  *string
	DepVal  string
	RootVal string
}

// BoolMerge represents a boolean field merge operation.
type BoolMerge struct {
	Target  *bool
	DepVal  *bool
	RootVal *bool
}

// SliceMerge represents a slice field merge operation.
type SliceMerge struct {
	Target  *[]string
	DepVal  []string
	RootVal []string
}

// S creates a StringMerge helper.
func S(target *string, depVal, rootVal string) StringMerge {
	return StringMerge{Target: target, DepVal: depVal, RootVal: rootVal}
}

// B creates a BoolMerge helper.
func B(target *bool, depVal, rootVal *bool) BoolMerge {
	return BoolMerge{Target: target, DepVal: depVal, RootVal: rootVal}
}

// Sl creates a SliceMerge helper.
func Sl(target *[]string, depVal, rootVal []string) SliceMerge {
	return SliceMerge{Target: target, DepVal: depVal, RootVal: rootVal}
}

