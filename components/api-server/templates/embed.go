package templates

import "embed"

//go:embed generate-*.txt
var GeneratorFS embed.FS

//go:embed new-project
var NewProjectFS embed.FS
