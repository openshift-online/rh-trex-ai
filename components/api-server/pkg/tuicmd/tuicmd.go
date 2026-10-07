// Package tuicmd provides the terminal UI command. It depends only on cobra,
// bubbletea and pkg/tui, so a CLI can import it without linking the server
// framework (database, gRPC, metrics) that pkg/cmd's serve and migrate commands
// need. pkg/cmd re-exports it for existing callers.
package tuicmd

import (
	"fmt"
	"os"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"

	"github.com/openshift-online/rh-trex-ai/components/api-server/pkg/tui"
)

type tuiModelFactory func(tui.Descriptor, tui.ClientConfig) (*tui.Model, error)
type tuiRunner func(*tui.Model) error
type tuiFileReader func(string) ([]byte, error)

// TUIOption customizes the command created by NewTUICommand.
type TUIOption func(*tuiOptions)

type tuiOptions struct {
	tokenProvider tui.TokenProvider
	loadSession   func() (Session, error)
}

// Session is the connection the terminal UI uses, typically taken from a saved
// login.
type Session struct {
	// Server is the API server URL.
	Server   string
	Insecure bool
	// TokenProvider supplies the bearer token per request, like WithTokenProvider.
	TokenProvider tui.TokenProvider
}

// WithSession makes the command call load when it runs, before it builds the
// model or switches to the alternate screen, so a generated CLI's terminal UI
// can reuse the saved login. Server and Insecure apply unless --server or
// --insecure was set on the command line, and a non-empty Server replaces the
// descriptor's default. TokenProvider is used like WithTokenProvider and takes
// precedence over it and over --token-file. An error from load is returned
// unchanged, so the caller can report "not logged in" or "requires a terminal".
func WithSession(load func() (Session, error)) TUIOption {
	return func(options *tuiOptions) { options.loadSession = load }
}

// WithTokenProvider makes the TUI ask provider for the bearer token on every
// authenticated request, so a credential refreshed during a long-lived session
// is used without restarting. It takes precedence over --token-file.
func WithTokenProvider(provider tui.TokenProvider) TUIOption {
	return func(options *tuiOptions) { options.tokenProvider = provider }
}

// NewTUICommand creates the OpenAPI-derived terminal browser command compiled
// directly into the service executable.
func NewTUICommand(getDescriptor func() ([]byte, error), options ...TUIOption) *cobra.Command {
	return newTUICommand(getDescriptor, tui.NewModel, runTUI, os.ReadFile, options...)
}

func newTUICommand(getDescriptor func() ([]byte, error), newModel tuiModelFactory, run tuiRunner, readFile tuiFileReader, options ...TUIOption) *cobra.Command {
	var settings tuiOptions
	for _, option := range options {
		option(&settings)
	}
	var server string
	var tokenFile string
	var insecure bool
	var trustedOrigins []string
	var refreshInterval time.Duration

	command := &cobra.Command{
		Use:   "tui",
		Short: "Browse and operate the service from a terminal UI",
		Long:  "Browse and operate the service through its OpenAPI-derived terminal UI.",
		Args:  cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			tokenProvider := settings.tokenProvider
			if settings.loadSession != nil {
				session, err := settings.loadSession()
				if err != nil {
					return err
				}
				if !command.Flags().Changed("server") && session.Server != "" {
					server = session.Server
				}
				if !command.Flags().Changed("insecure") {
					insecure = session.Insecure
				}
				if session.TokenProvider != nil {
					tokenProvider = session.TokenProvider
				}
			}
			if getDescriptor == nil {
				return fmt.Errorf("load TUI descriptor: descriptor provider is nil")
			}
			data, err := getDescriptor()
			if err != nil {
				return fmt.Errorf("load TUI descriptor: %w", err)
			}
			descriptor, err := tui.ParseDescriptor(data)
			if err != nil {
				return err
			}
			resolvedServer := server
			if resolvedServer == "" && len(descriptor.Servers) > 0 {
				resolvedServer = descriptor.Servers[0].URL
			}
			credential := ""
			if tokenFile != "" {
				token, readErr := readFile(tokenFile)
				if readErr != nil {
					return fmt.Errorf("read token file: %w", readErr)
				}
				credential = strings.TrimSpace(string(token))
			}
			model, err := newModel(descriptor, tui.ClientConfig{
				BaseURL: resolvedServer, Token: credential, TokenProvider: tokenProvider, Insecure: insecure,
				TrustedOrigins: append([]string(nil), trustedOrigins...), RefreshInterval: refreshInterval,
			})
			if err != nil {
				return fmt.Errorf("initialize TUI: %w", err)
			}
			if err := run(model); err != nil {
				return fmt.Errorf("run TUI: %w", err)
			}
			return nil
		},
	}
	flags := command.Flags()
	flags.StringVar(&server, "server", "", "API server URL (defaults to the first OpenAPI server)")
	flags.StringVar(&tokenFile, "token-file", "", "file containing a bearer token; use /dev/stdin to read standard input")
	flags.BoolVar(&insecure, "insecure", false, "allow non-loopback HTTP or skip TLS verification")
	flags.StringArrayVar(&trustedOrigins, "trust-origin", nil, "additional operation-server origin trusted to receive credentials (repeatable)")
	flags.DurationVar(&refreshInterval, "refresh-interval", 5*time.Second, "polling interval; 0 disables polling")
	return command
}

func runTUI(model *tui.Model) error {
	_, err := tea.NewProgram(model, tea.WithAltScreen()).Run()
	return err
}
