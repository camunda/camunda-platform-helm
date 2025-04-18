package main

import (
	"github.com/stretchr/testify/require"
	"testing"
)

func TestProcess(t *testing.T) {
	inputs := Input{
		HelmChartModifiedDirectories: []string{"charts/camunda-platform-8.8"},
	}

	outputs, err := processInputs(inputs)
	require.NoError(t, err)

	require.Len(t, outputs, 1)
	require.Contains(t, outputs[0].CamundaVersion, "8.8")
	require.Contains(t, outputs[0].HelmChartVersion, "13")
	require.Contains(t, outputs[0].PreviousHelmChartVersion, "12")
}

func TestProcessInvalidChartDir(t *testing.T) {
	inputs := Input{
		HelmChartModifiedDirectories: []string{"charts/invalid"},
	}

	outputs, err := processInputs(inputs)
	require.NotNil(t, err)
	require.Nil(t, outputs)
}
