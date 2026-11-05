package templates

import "embed"

//go:embed generate-*.txt
var GeneratorFS embed.FS

// The all: prefix keeps files whose names start with "." or "_" (for
// example .claude/commands and data/generated/tui/.trex-tui-generated),
// which go:embed otherwise skips.
//
//go:embed all:new-project
var NewProjectFS embed.FS
