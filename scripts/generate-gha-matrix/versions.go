package main

import (
	"golang.org/x/mod/semver"
	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/chart/loader"
	"strconv"
	"strings"
)

func getCamundaVersion(chart *chart.Chart) string {
	var version string
	if chart.Values["zeebe"] == nil {
		version = "v" + chart.Values["core"].(map[string]interface{})["image"].(map[string]interface{})["tag"].(string)
	} else {
		version = "v" + chart.Values["zeebe"].(map[string]interface{})["image"].(map[string]interface{})["tag"].(string)
	}
	camundaVersionParsed := semver.MajorMinor(version)
	camundaVersionParsed = strings.TrimPrefix(camundaVersionParsed, "v")
	return camundaVersionParsed
}

func getPreviousHelmChartVersion(chart *chart.Chart, version string) (string, error) {
	camundaVersionFloat, err := strconv.ParseFloat(version, 64)
	if err != nil {
		return "", VersionParsingErrorf("failed to parse version from input: %s", err)
	}
	previousVersionFloat := (camundaVersionFloat*10 - 1) / 10
	previousCamundaVersion := strconv.FormatFloat(previousVersionFloat, 'f', -1, 64)
	previousChart, err := loader.Load("../../charts/camunda-platform-" + previousCamundaVersion)
	if err != nil {
		return "", VersionParsingErrorf("failed to load the previous chart: %s", err)
	}
	previousChartVersionSemver := "v" + previousChart.Metadata.Version
	previousVersionMajor := semver.Major(previousChartVersionSemver)
	previousVersionMajor = strings.TrimPrefix(previousVersionMajor, "v")
	return previousVersionMajor, nil
}
