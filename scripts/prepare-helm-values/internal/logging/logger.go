package logging

import (
	"fmt"

	"github.com/jwalton/gchalk"
)

type Logger struct {
	Verbose bool
	Color   bool
}

func (l Logger) Tag(text string) string {
	if !l.Color {
		return text
	}
	return gchalk.Red(text)
}

func (l Logger) tag(text string, colorFn func(...string) string) string {
	if !l.Color {
		return text
	}
	return colorFn(text)
}

func (l Logger) Infof(format string, a ...any)  { fmt.Printf("%s %s\n", l.tag("[INFO]", gchalk.Blue), fmt.Sprintf(format, a...)) }
func (l Logger) Okf(format string, a ...any)    { fmt.Printf("%s %s\n", l.tag("[ OK ]", gchalk.Green), fmt.Sprintf(format, a...)) }
func (l Logger) Warnf(format string, a ...any)  { fmt.Printf("%s %s\n", l.tag("[WARN]", gchalk.Yellow), fmt.Sprintf(format, a...)) }
func (l Logger) Errorf(format string, a ...any) { fmt.Printf("%s %s\n", l.tag("[ERR ]", gchalk.Red), fmt.Sprintf(format, a...)) }
func (l Logger) Debugf(format string, a ...any) {
	if l.Verbose {
		fmt.Printf("%s %s\n", l.tag("[DBG ]", gchalk.Cyan), fmt.Sprintf(format, a...))
	}
}


