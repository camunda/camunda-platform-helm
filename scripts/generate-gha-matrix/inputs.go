package main

import (
	"bufio"
	"os"
)

func parseInput() Input {
	stdinReader := bufio.NewReader(os.Stdin)
	scanner := bufio.NewScanner(stdinReader)
	scanner.Split(bufio.ScanWords)

	var input Input
	for scanner.Scan() {
		input.HelmChartModifiedDirectories = append(input.HelmChartModifiedDirectories, scanner.Text())
	}
	return input
}
