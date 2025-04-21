package main

type Input struct {
	HelmChartModifiedDirectories []string
}

type Output struct {
	CamundaVersion           string `json:"version"`
	HelmChartVersion         string `json:"chartVersion"`
	PreviousHelmChartVersion string `json:"previousHelmVersion"`
	PreviousHelmChartDir     string `json:"previousHelmDir"`
}

type ChartVersion struct {
	Version string
	Dir     string
}
