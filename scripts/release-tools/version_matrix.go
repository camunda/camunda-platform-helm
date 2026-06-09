package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"scripts/camunda-core/pkg/executil"
	"scripts/camunda-core/pkg/releasenotes"
	"scripts/camunda-core/pkg/releaseplease"
	"scripts/camunda-core/pkg/versionmatrix"
)

// runVersionMatrix renders version-matrix README files. Modes (exactly one):
//
//	--readme <app> --chart-version <v>  splice the single version <v>'s section
//	                into version-matrix/camunda-<app>/README.md (its entry is
//	                read from version-matrix.json); other rows stay verbatim.
//	--index         scan version-matrix/ dirs, write version-matrix/README.md.
func runVersionMatrix(args []string) error {
	fs := flag.NewFlagSet("version-matrix", flag.ContinueOnError)
	var app, chartVersion string
	var index bool
	fs.StringVar(&app, "readme", "", "app version (e.g. 8.7) — splices a version into version-matrix/camunda-<app>/README.md")
	fs.StringVar(&chartVersion, "chart-version", "", "chart version to splice (e.g. 12.10.0); required with --readme")
	fs.BoolVar(&index, "index", false, "render version-matrix/README.md from all camunda-* dirs")
	if err := fs.Parse(args); err != nil {
		return err
	}
	switch {
	case app != "" && index:
		return fmt.Errorf("--readme and --index are mutually exclusive")
	case app != "":
		if chartVersion == "" {
			return fmt.Errorf("--chart-version is required with --readme")
		}
		return renderVersionMatrixReadme(app, chartVersion)
	case index:
		return renderVersionMatrixIndex()
	default:
		return fmt.Errorf("--readme <app> --chart-version <v>, or --index, is required")
	}
}

// renderVersionMatrixReadme reads chartVersion's entry from the app's
// version-matrix.json and splices its section into the per-app README,
// preserving every other version's section.
func renderVersionMatrixReadme(app, chartVersion string) error {
	matrixFile := filepath.Join("version-matrix", "camunda-"+app, "version-matrix.json")
	data, err := os.ReadFile(matrixFile)
	if err != nil {
		return fmt.Errorf("read %s: %w", matrixFile, err)
	}
	var entries []versionmatrix.ChartEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		return fmt.Errorf("parse %s: %w", matrixFile, err)
	}

	var entry *versionmatrix.ChartEntry
	for i := range entries {
		if entries[i].ChartVersion == chartVersion {
			entry = &entries[i]
			break
		}
	}
	if entry == nil {
		return fmt.Errorf("no entry for chart version %s in %s", chartVersion, matrixFile)
	}

	pin := resolveHelmPin(app, chartVersion)
	helmCLIVersions := splitCSV(releasenotes.HelmCLIVersion(app, pin))

	outPath := filepath.Join("version-matrix", "camunda-"+app, "README.md")
	existing, err := os.ReadFile(outPath)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("read %s: %w", outPath, err)
	}

	readme := versionmatrix.SpliceReadme(string(existing), app, *entry, helmCLIVersions)
	if err := os.WriteFile(outPath, []byte(readme), 0o644); err != nil {
		return fmt.Errorf("write %s: %w", outPath, err)
	}
	fmt.Fprintf(os.Stderr, "spliced %s into %s\n", chartVersion, outPath)
	return nil
}

func renderVersionMatrixIndex() error {
	dir := "version-matrix"
	dirEntries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("read %s: %w", dir, err)
	}

	entriesByApp := make(map[string][]versionmatrix.ChartEntry)
	var appVersions []string
	for _, de := range dirEntries {
		if !de.IsDir() || !strings.HasPrefix(de.Name(), "camunda-") {
			continue
		}
		app := strings.TrimPrefix(de.Name(), "camunda-")
		matrixFile := filepath.Join(dir, de.Name(), "version-matrix.json")
		data, err := os.ReadFile(matrixFile)
		if err != nil {
			continue
		}
		var charts []versionmatrix.ChartEntry
		if err := json.Unmarshal(data, &charts); err != nil {
			return fmt.Errorf("parse %s: %w", matrixFile, err)
		}
		appVersions = append(appVersions, app)
		entriesByApp[app] = versionmatrix.SortEntriesDescending(charts)
	}

	appVersions = versionmatrix.SortAppVersionsDescending(appVersions)

	readme := versionmatrix.RenderIndex(appVersions, entriesByApp)
	outPath := filepath.Join(dir, "README.md")
	if err := os.WriteFile(outPath, []byte(readme), 0o644); err != nil {
		return fmt.Errorf("write %s: %w", outPath, err)
	}
	fmt.Fprintf(os.Stderr, "wrote %s (%d app versions)\n", outPath, len(appVersions))
	return nil
}

// resolveHelmPin reads the helm pin from the git tag for the given chart
// version. Falls back to HEAD's .tool-versions when the tag does not exist.
func resolveHelmPin(app, chartVersion string) string {
	ref := releaseplease.ReleaseTag(app, chartVersion) + ":.tool-versions"
	out, err := executil.RunCommandCapture(
		context.Background(), "git", []string{"show", ref}, nil, "")
	if err != nil {
		data, _ := os.ReadFile(".tool-versions")
		return parseHelmPin(string(data))
	}
	return parseHelmPin(string(out))
}

// parseHelmPin extracts the helm version from a .tool-versions file content.
func parseHelmPin(toolVersions string) string {
	for _, line := range strings.Split(toolVersions, "\n") {
		if strings.HasPrefix(line, "helm ") {
			if fields := strings.Fields(line); len(fields) >= 2 {
				return fields[1]
			}
		}
	}
	return ""
}

// splitCSV splits a comma-separated version list, trimming whitespace.
func splitCSV(csv string) []string {
	var result []string
	for _, v := range strings.Split(csv, ",") {
		if v = strings.TrimSpace(v); v != "" {
			result = append(result, v)
		}
	}
	return result
}
