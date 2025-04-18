package main

import (
	"fmt"
)

type ProcessError string

func (e ProcessError) Error() string {
	return "process: " + string(e)
}

func ProcessErrorf(msg string, args ...interface{}) error {
	return ProcessError(fmt.Sprintf(msg, args...))
}

type VersionParsingError string

func (e VersionParsingError) Error() string {
	return "versions: " + string(e)
}

func VersionParsingErrorf(msg string, args ...interface{}) error {
	return VersionParsingError(fmt.Sprintf(msg, args...))
}
