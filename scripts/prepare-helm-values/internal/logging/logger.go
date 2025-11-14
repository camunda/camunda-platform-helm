package logging

import (
	"fmt"

	"github.com/ttacon/chalk"
)

type Logger struct {
	Verbose bool
	Color   bool
}

func (l Logger) Tag(text string) string {
	if !l.Color {
		return text
	}
	return chalk.Red.Color(text)
}

func (l Logger) tag(text string, color chalk.Color) string {
	if !l.Color {
		return text
	}
	return color.Color(text)
}

func (l Logger) Infof(format string, a ...any)  { fmt.Printf("%s %s\n", l.tag("[INFO]", chalk.Blue), fmt.Sprintf(format, a...)) }
func (l Logger) Okf(format string, a ...any)    { fmt.Printf("%s %s\n", l.tag("[ OK ]", chalk.Green), fmt.Sprintf(format, a...)) }
func (l Logger) Warnf(format string, a ...any)  { fmt.Printf("%s %s\n", l.tag("[WARN]", chalk.Yellow), fmt.Sprintf(format, a...)) }
func (l Logger) Errorf(format string, a ...any) { fmt.Printf("%s %s\n", l.tag("[ERR ]", chalk.Red), fmt.Sprintf(format, a...)) }
func (l Logger) Debugf(format string, a ...any) {
	if l.Verbose {
		fmt.Printf("%s %s\n", l.tag("[DBG ]", chalk.Cyan), fmt.Sprintf(format, a...))
	}
}


