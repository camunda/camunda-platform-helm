package matrix

import (
	"fmt"

	"scripts/camunda-core/pkg/versionmatrix"
	"scripts/deploy-camunda/config"
	"scripts/deploy-camunda/deploy"
)

func ResolvePostgresCredentials(entries []Entry, opts RunOptions) (string, string, error) {
	if !opts.GeneratePostgresCredentials {
		return "", "", nil
	}
	username, password := "", ""
	requiredAny := false
	for _, entry := range entries {
		flags, _, _, _, cleanup, err := BuildEntryFlags(entry, withoutPostgresBootstrap(opts))
		if err != nil {
			return "", "", err
		}
		required := requiresPostgresCredentials(flags.CompanionCharts)
		if !required {
			cleanup()
			continue
		}
		requiredAny = true
		effective := effectiveCredentialEnv(flags)
		cleanup()
		entryUser, entryPassword := effective["RDBMS_POSTGRESQL_USERNAME"], effective["RDBMS_POSTGRESQL_PASSWORD"]
		if versionmatrix.IsUpgradeOnlyFlow(entry.Flow) && (entryUser == "" || entryPassword == "") {
			return "", "", fmt.Errorf("upgrade-only entry %s requires explicit RDBMS_POSTGRESQL_USERNAME and RDBMS_POSTGRESQL_PASSWORD", EntryID(entry))
		}
		if entryUser != "" && username != "" && entryUser != username {
			return "", "", fmt.Errorf("selected entries resolve different RDBMS_POSTGRESQL_USERNAME values")
		}
		if entryPassword != "" && password != "" && entryPassword != password {
			return "", "", fmt.Errorf("selected entries resolve different RDBMS_POSTGRESQL_PASSWORD values")
		}
		if entryUser != "" {
			username = entryUser
		}
		if entryPassword != "" {
			password = entryPassword
		}
	}
	if !requiredAny {
		return "", "", nil
	}
	if username == "" {
		username = "camunda"
	}
	if password == "" {
		generated, err := deploy.RandomSecret()
		if err != nil {
			return "", "", err
		}
		password = generated
	}
	return username, password, nil
}

func withoutPostgresBootstrap(opts RunOptions) RunOptions {
	opts.GeneratePostgresCredentials = false
	opts.GeneratedPostgresUsername = ""
	opts.GeneratedPostgresPassword = ""
	return opts
}

func requiresPostgresCredentials(companions []config.CompanionChart) bool {
	for _, companion := range companions {
		for _, name := range companion.EnvVars {
			if name == "RDBMS_POSTGRESQL_USERNAME" || name == "RDBMS_POSTGRESQL_PASSWORD" {
				return true
			}
		}
	}
	return false
}

func effectiveCredentialEnv(flags *config.RuntimeFlags) map[string]string {
	values := make(map[string]string)
	for _, item := range deploy.EnvProvenance(flags) {
		values[item.Name] = item.Value
	}
	return values
}
