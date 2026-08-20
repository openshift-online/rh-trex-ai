package cmd

import (
	"fmt"
	"os"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"

	"github.com/openshift-online/rh-trex-ai/pkg/tui"
)

type tuiModelFactory func(tui.Descriptor, tui.ClientConfig) (*tui.Model, error)
type tuiRunner func(*tui.Model) error
type tuiFileReader func(string) ([]byte, error)

// NewTUICommand creates the OpenAPI-derived terminal browser command compiled
// directly into the service executable.
func NewTUICommand(getDescriptor func() ([]byte, error)) *cobra.Command {
	return newTUICommand(getDescriptor, tui.NewModel, runTUI, os.ReadFile)
}

func newTUICommand(getDescriptor func() ([]byte, error), newModel tuiModelFactory, run tuiRunner, readFile tuiFileReader) *cobra.Command {
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
		RunE: func(_ *cobra.Command, _ []string) error {
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
				BaseURL: resolvedServer, Token: credential, Insecure: insecure,
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
