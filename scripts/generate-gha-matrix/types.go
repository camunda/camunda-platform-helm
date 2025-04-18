package main

type Input struct {
	HelmChartModifiedDirectories []string
}

type Output struct {
	CamundaVersion           string `json:"version"`
	HelmChartVersion         string `json:"chartVersion"`
	PreviousHelmChartVersion string `json:"previousHelmVersion"`
}
