package tuicmd

import (
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/spf13/cobra"

	"github.com/openshift-online/rh-trex-ai/components/api-server/pkg/tui"
)

func TestTUICommandBuildsModelDirectlyWithEstablishedOptions(t *testing.T) {
	descriptorJSON := []byte(`{"title":"Inventory API","servers":[{"url":"https://default.example.test"}],"views":[],"operations":[]}`)
	var capturedDescriptor tui.Descriptor
	var capturedConfig tui.ClientConfig
	var capturedTokenPath string
	runs := 0
	command := newTUICommand(
		func() ([]byte, error) { return descriptorJSON, nil },
		func(descriptor tui.Descriptor, config tui.ClientConfig) (*tui.Model, error) {
			capturedDescriptor, capturedConfig = descriptor, config
			return &tui.Model{}, nil
		},
		func(*tui.Model) error { runs++; return nil },
		func(path string) ([]byte, error) {
			capturedTokenPath = path
			return []byte("  secret-token\n"), nil
		},
	)
	command.SetArgs([]string{
		"--server", "https://override.example.test", "--token-file", "/credentials/token",
		"--insecure", "--trust-origin", "https://one.example.test", "--trust-origin", "https://two.example.test",
		"--refresh-interval", "17s",
	})
	if err := command.Execute(); err != nil {
		t.Fatal(err)
	}
	if capturedDescriptor.Title != "Inventory API" || capturedTokenPath != "/credentials/token" || runs != 1 {
		t.Fatalf("direct command inputs = descriptor %#v token path %q runs %d", capturedDescriptor, capturedTokenPath, runs)
	}
	if capturedConfig.BaseURL != "https://override.example.test" || capturedConfig.Token != "secret-token" || !capturedConfig.Insecure || capturedConfig.RefreshInterval != 17*time.Second {
		t.Fatalf("client config = %#v", capturedConfig)
	}
	wantOrigins := []string{"https://one.example.test", "https://two.example.test"}
	if !reflect.DeepEqual(capturedConfig.TrustedOrigins, wantOrigins) {
		t.Fatalf("trusted origins = %#v, want %#v", capturedConfig.TrustedOrigins, wantOrigins)
	}
}

func TestTUICommandDefaultsServerAndRejectsArguments(t *testing.T) {
	descriptorJSON := []byte(`{"title":"Inventory API","servers":[{"url":"https://default.example.test"}],"views":[],"operations":[]}`)
	var capturedConfig tui.ClientConfig
	runs := 0
	newCommand := func() *cobra.Command {
		return newTUICommand(
			func() ([]byte, error) { return descriptorJSON, nil },
			func(_ tui.Descriptor, config tui.ClientConfig) (*tui.Model, error) {
				capturedConfig = config
				return &tui.Model{}, nil
			},
			func(*tui.Model) error { runs++; return nil },
			func(string) ([]byte, error) { return nil, nil },
		)
	}
	command := newCommand()
	command.SetArgs(nil)
	if err := command.Execute(); err != nil {
		t.Fatal(err)
	}
	if capturedConfig.BaseURL != "https://default.example.test" || capturedConfig.RefreshInterval != 5*time.Second || runs != 1 {
		t.Fatalf("default config = %#v, runs %d", capturedConfig, runs)
	}

	command = newCommand()
	command.SetArgs([]string{"unexpected"})
	if err := command.Execute(); err == nil || !strings.Contains(err.Error(), "unknown command") && !strings.Contains(err.Error(), "arg") {
		t.Fatalf("positional argument error = %v", err)
	}
	if runs != 1 {
		t.Fatalf("runner invoked for rejected argument: %d", runs)
	}
}

func TestTUICommandReturnsDescriptorFileModelAndProgramErrors(t *testing.T) {
	descriptorJSON := []byte(`{"title":"Inventory API","views":[],"operations":[]}`)
	tests := []struct {
		name    string
		command func() *cobra.Command
		want    string
	}{
		{
			name: "descriptor provider",
			command: func() *cobra.Command {
				return newTUICommand(func() ([]byte, error) { return nil, errors.New("descriptor failed") }, nil, nil, nil)
			},
			want: "load TUI descriptor: descriptor failed",
		},
		{
			name: "descriptor parse",
			command: func() *cobra.Command {
				return newTUICommand(func() ([]byte, error) { return []byte("{"), nil }, nil, nil, nil)
			},
			want: "decode generated TUI descriptor",
		},
		{
			name: "token file",
			command: func() *cobra.Command {
				command := newTUICommand(func() ([]byte, error) { return descriptorJSON, nil }, nil, nil, func(string) ([]byte, error) { return nil, errors.New("read failed") })
				command.SetArgs([]string{"--token-file", "/missing"})
				return command
			},
			want: "read token file: read failed",
		},
		{
			name: "model",
			command: func() *cobra.Command {
				return newTUICommand(func() ([]byte, error) { return descriptorJSON, nil }, func(tui.Descriptor, tui.ClientConfig) (*tui.Model, error) { return nil, errors.New("model failed") }, nil, func(string) ([]byte, error) { return nil, nil })
			},
			want: "initialize TUI: model failed",
		},
		{
			name: "program",
			command: func() *cobra.Command {
				return newTUICommand(func() ([]byte, error) { return descriptorJSON, nil }, func(tui.Descriptor, tui.ClientConfig) (*tui.Model, error) { return &tui.Model{}, nil }, func(*tui.Model) error { return errors.New("program failed") }, func(string) ([]byte, error) { return nil, nil })
			},
			want: "run TUI: program failed",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			command := test.command()
			if command.Flags().Lookup("token-file") == nil || command.Flags().Lookup("trust-origin") == nil || command.Flags().Lookup("refresh-interval") == nil {
				t.Fatal("integrated command omitted established flags")
			}
			err := command.Execute()
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("command error = %v, want %q", err, test.want)
			}
		})
	}
}

type fixedTokenProvider struct{ token string }

func (provider fixedTokenProvider) GetToken() (string, error) { return provider.token, nil }

func TestTUICommandPassesTokenProviderToClientConfig(t *testing.T) {
	descriptorJSON := []byte(`{"title":"Inventory API","servers":[{"url":"https://default.example.test"}],"views":[],"operations":[]}`)
	var capturedConfig tui.ClientConfig
	provider := fixedTokenProvider{token: "refreshed"}
	command := newTUICommand(
		func() ([]byte, error) { return descriptorJSON, nil },
		func(_ tui.Descriptor, config tui.ClientConfig) (*tui.Model, error) {
			capturedConfig = config
			return &tui.Model{}, nil
		},
		func(*tui.Model) error { return nil },
		func(string) ([]byte, error) { return nil, nil },
		WithTokenProvider(provider),
	)
	command.SetArgs(nil)
	if err := command.Execute(); err != nil {
		t.Fatal(err)
	}
	if capturedConfig.TokenProvider != provider {
		t.Fatalf("token provider = %#v, want the configured provider", capturedConfig.TokenProvider)
	}
}

func TestTUICommandWithSession(t *testing.T) {
	descriptorJSON := []byte(`{"title":"Inventory API","servers":[{"url":"https://default.example.test"}],"views":[],"operations":[]}`)
	sessionProvider := fixedTokenProvider{token: "session"}
	run := func(t *testing.T, session Session, loadErr error, arguments []string, extra ...TUIOption) (config tui.ClientConfig, events []string, err error) {
		t.Helper()
		options := append([]TUIOption{WithSession(func() (Session, error) {
			events = append(events, "load")
			return session, loadErr
		})}, extra...)
		command := newTUICommand(
			func() ([]byte, error) { return descriptorJSON, nil },
			func(_ tui.Descriptor, clientConfig tui.ClientConfig) (*tui.Model, error) {
				events = append(events, "model")
				config = clientConfig
				return &tui.Model{}, nil
			},
			func(*tui.Model) error { events = append(events, "run"); return nil },
			func(string) ([]byte, error) { return nil, nil },
			options...,
		)
		command.SetArgs(arguments)
		err = command.Execute()
		return config, events, err
	}

	t.Run("the session applies and load runs before the model is built", func(t *testing.T) {
		config, events, err := run(t, Session{Server: "https://session.example.test", Insecure: true, TokenProvider: sessionProvider}, nil, nil)
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(events, []string{"load", "model", "run"}) {
			t.Fatalf("events = %v, want load before model before run", events)
		}
		if config.BaseURL != "https://session.example.test" || !config.Insecure || config.TokenProvider != sessionProvider {
			t.Fatalf("client config = %#v", config)
		}
	})

	t.Run("explicit flags win over the session", func(t *testing.T) {
		config, _, err := run(t, Session{Server: "https://session.example.test", Insecure: true}, nil, []string{"--server", "https://flag.example.test", "--insecure=false"})
		if err != nil {
			t.Fatal(err)
		}
		if config.BaseURL != "https://flag.example.test" || config.Insecure {
			t.Fatalf("flags must override the session: %#v", config)
		}
	})

	t.Run("an empty session server keeps the descriptor default", func(t *testing.T) {
		config, _, err := run(t, Session{}, nil, nil)
		if err != nil || config.BaseURL != "https://default.example.test" {
			t.Fatalf("config = %#v, err = %v", config, err)
		}
	})

	t.Run("the session token provider takes precedence over WithTokenProvider", func(t *testing.T) {
		config, _, err := run(t, Session{TokenProvider: sessionProvider}, nil, nil, WithTokenProvider(fixedTokenProvider{token: "static"}))
		if err != nil || config.TokenProvider != sessionProvider {
			t.Fatalf("config = %#v, err = %v", config, err)
		}
		config, _, err = run(t, Session{}, nil, nil, WithTokenProvider(fixedTokenProvider{token: "static"}))
		if err != nil || config.TokenProvider != (fixedTokenProvider{token: "static"}) {
			t.Fatalf("without a session provider the option applies: %#v, err = %v", config, err)
		}
	})

	t.Run("a load error is returned unchanged and nothing is built or run", func(t *testing.T) {
		loadErr := errors.New("not logged in")
		_, events, err := run(t, Session{}, loadErr, nil)
		if err != loadErr {
			t.Fatalf("error = %v, want the load error unchanged", err)
		}
		if !reflect.DeepEqual(events, []string{"load"}) {
			t.Fatalf("events = %v, want only load", events)
		}
	})
}
