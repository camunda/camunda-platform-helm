module scripts/release-tools

go 1.25.0

require scripts/camunda-core v0.0.0

require (
	github.com/jwalton/gchalk v1.3.0 // indirect
	github.com/jwalton/go-supportscolor v1.1.0 // indirect
	github.com/mattn/go-colorable v0.1.14 // indirect
	github.com/mattn/go-isatty v0.0.20 // indirect
	github.com/rs/zerolog v1.35.1 // indirect
	golang.org/x/sys v0.43.0 // indirect
	golang.org/x/term v0.42.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace scripts/camunda-core => ../camunda-core
