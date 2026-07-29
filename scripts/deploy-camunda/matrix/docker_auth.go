package matrix

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type DockerAuth struct {
	Username string
	Password string
}

type dockerConfig struct {
	Auths       map[string]dockerAuthEntry `json:"auths"`
	CredsStore  string                     `json:"credsStore"`
	CredHelpers map[string]string          `json:"credHelpers"`
}

type dockerAuthEntry struct {
	Auth     string `json:"auth"`
	Username string `json:"username"`
	Password string `json:"password"`
}

func ImportPlaintextDockerAuth(configPath string, registries ...string) (map[string]DockerAuth, error) {
	if configPath == "" {
		configDir := os.Getenv("DOCKER_CONFIG")
		if configDir == "" {
			home, err := os.UserHomeDir()
			if err != nil {
				return nil, err
			}
			configDir = filepath.Join(home, ".docker")
		}
		configPath = filepath.Join(configDir, "config.json")
	}
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("read Docker config: %w", err)
	}
	var cfg dockerConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("decode Docker config: %w", err)
	}
	result := make(map[string]DockerAuth)
	for _, registry := range registries {
		_, entry, ok := findDockerAuth(cfg.Auths, registry)
		helper := findDockerHelper(cfg.CredHelpers, registry)
		if helper != "" {
			return nil, fmt.Errorf("Docker credentials for %s use helper %q; helper execution is not permitted, provide explicit credentials", registry, helper)
		}
		if cfg.CredsStore != "" {
			return nil, fmt.Errorf("Docker credentials use global store %q; helper execution is not permitted, provide explicit credentials", cfg.CredsStore)
		}
		if !ok {
			continue
		}
		auth, err := decodeDockerAuth(entry)
		if err != nil {
			return nil, fmt.Errorf("decode Docker credentials for %s: %w", registry, err)
		}
		result[registry] = auth
	}
	return result, nil
}

func findDockerHelper(helpers map[string]string, registry string) string {
	aliases := dockerRegistryAliases(registry)
	for _, alias := range aliases {
		if helper := helpers[alias]; helper != "" {
			return helper
		}
	}
	return ""
}

func findDockerAuth(auths map[string]dockerAuthEntry, registry string) (string, dockerAuthEntry, bool) {
	aliases := dockerRegistryAliases(registry)
	for _, alias := range aliases {
		if entry, ok := auths[alias]; ok {
			return alias, entry, true
		}
	}
	return registry, dockerAuthEntry{}, false
}

func dockerRegistryAliases(registry string) []string {
	aliases := []string{registry}
	if registry == "docker.io" {
		aliases = append(aliases, "https://index.docker.io/v1/", "index.docker.io")
	}
	return aliases
}

func decodeDockerAuth(entry dockerAuthEntry) (DockerAuth, error) {
	if entry.Username != "" && entry.Password != "" {
		return DockerAuth{Username: entry.Username, Password: entry.Password}, nil
	}
	if entry.Auth == "" {
		return DockerAuth{}, errors.New("auth entry has no username/password")
	}
	decoded, err := base64.StdEncoding.DecodeString(entry.Auth)
	if err != nil {
		return DockerAuth{}, err
	}
	username, password, ok := strings.Cut(string(decoded), ":")
	if !ok || username == "" || password == "" {
		return DockerAuth{}, errors.New("auth entry is not username:password")
	}
	return DockerAuth{Username: username, Password: password}, nil
}
