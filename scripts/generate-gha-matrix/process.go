package main

import (
	"helm.sh/helm/v3/pkg/chart/loader"
)

func processInputs(input Input) ([]Output, error) {
	var output []Output
	for _, modifiedDir := range input.HelmChartModifiedDirectories {
		chart, err := loader.Load("../../" + modifiedDir)
		if err != nil {
			return nil, ProcessErrorf("failed to load chart for modified dir %s : %s", modifiedDir, err)
		}

		version := getCamundaVersion(chart)
		previousVersion, err := getPreviousHelmChartVersion(chart, version)
		if err != nil {
			return nil, ProcessErrorf("failed to get previous helm chart metadata for version %s : %s", version, err)
		}

		matrixRunVector := Output{
			CamundaVersion:           version,
			HelmChartVersion:         chart.Metadata.Version,
			PreviousHelmChartVersion: previousVersion.Version,
			PreviousHelmChartDir:     previousVersion.Dir,
		}

		output = append(output, matrixRunVector)
	}
	return output, nil
}
