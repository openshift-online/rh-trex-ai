package tui

import (
	pkgcmd "github.com/openshift-online/rh-trex-ai/components/api-server/pkg/tuicmd"

	"github.com/openshift-online/rh-trex-ai/components/ctl/data/generated/tui"
)

// Cmd browses and operates the service through its OpenAPI-derived terminal
// UI. It reuses the TRex library's Bubble Tea runtime with this project's
// embedded descriptor.
var Cmd = pkgcmd.NewTUICommand(tui.GetDescriptor)
