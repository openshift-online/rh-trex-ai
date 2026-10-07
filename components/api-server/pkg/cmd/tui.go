package cmd

import (
	"github.com/spf13/cobra"

	"github.com/openshift-online/rh-trex-ai/components/api-server/pkg/tui"
	"github.com/openshift-online/rh-trex-ai/components/api-server/pkg/tuicmd"
)

// The terminal UI command lives in pkg/tuicmd so a CLI can use it without
// importing this package, which links the server framework. These re-exports keep
// existing callers of pkg/cmd working unchanged.

// TUIOption customizes the command created by NewTUICommand.
type TUIOption = tuicmd.TUIOption

// Session is the connection the terminal UI uses.
type Session = tuicmd.Session

// NewTUICommand creates the OpenAPI-derived terminal browser command compiled
// directly into the service executable. See tuicmd.NewTUICommand.
func NewTUICommand(getDescriptor func() ([]byte, error), options ...TUIOption) *cobra.Command {
	return tuicmd.NewTUICommand(getDescriptor, options...)
}

// WithTokenProvider makes the TUI ask provider for the bearer token on every
// authenticated request. See tuicmd.WithTokenProvider.
func WithTokenProvider(provider tui.TokenProvider) TUIOption {
	return tuicmd.WithTokenProvider(provider)
}

// WithSession makes the command reuse a saved login. See tuicmd.WithSession.
func WithSession(load func() (Session, error)) TUIOption {
	return tuicmd.WithSession(load)
}
