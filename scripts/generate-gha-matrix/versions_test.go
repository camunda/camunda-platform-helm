package main

import (
	"github.com/stretchr/testify/require"
	"helm.sh/helm/v3/pkg/chart/loader"
	"regexp"
	"testing"
)

var validVersionRegex = regexp.MustCompile(`^\d+\.\d+$`)

func TestGetCamundaVersionPre88(t *testing.T) {
	chart, err := loader.Load("../../charts/camunda-platform-8.6")
	require.NoError(t, err)

	version := getCamundaVersion(chart)
	valid := validVersionRegex.MatchString(version)

	require.True(t, valid)
}

func TestGetCamundaVersionPost88(t *testing.T) {
	chart, err := loader.Load("../../charts/camunda-platform-8.8")
	require.NoError(t, err)

	version := getCamundaVersion(chart)
	valid := validVersionRegex.MatchString(version)

	require.True(t, valid)
}

func TestGetPreviousChart(t *testing.T) {
	chart, err := loader.Load("../../charts/camunda-platform-8.8")
	require.NoError(t, err)

	version := getCamundaVersion(chart)
	previousVersion, err := getPreviousHelmChartVersion(chart, version)
	require.NoError(t, err)
	require.Equal(t, "12", previousVersion)
}

func TestGetPreviousChartInvalidVersion(t *testing.T) {
	chart, err := loader.Load("../../charts/camunda-platform-8.8")
	require.NoError(t, err)

	version := "badversion"
	_, err = getPreviousHelmChartVersion(chart, version)
	require.NotNil(t, err)
	require.Contains(t, err.Error(), "invalid syntax")
}

func TestGetPreviousChartChartTooOld(t *testing.T) {
	chart, err := loader.Load("../../charts/camunda-platform-8.2")
	require.NoError(t, err)

	version := getCamundaVersion(chart)
	_, err = getPreviousHelmChartVersion(chart, version)
	require.NotNil(t, err)
	require.Contains(t, err.Error(), "failed to load the previous chart")
}
