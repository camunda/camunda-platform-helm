package main

import (
	"encoding/json"
	"fmt"
)

/*
This script accepts input piped into it and outputs a github-actions readable
matrix of tests to run.

ex: echo charts/camunda-platform-8.3 charts/camunda-platform-8.4 charts/camunda-platform-8.8 | go run . | jq
*/
func main() {
	input := parseInput()

	output, err := processInputs(input)
	if err != nil {
		panic(err)
	}

	final, err := json.Marshal(output)
	if err != nil {
		panic(err)
	}
	fmt.Println(string(final))
}
