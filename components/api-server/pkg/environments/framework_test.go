package environments

import (
	"log/slog"
	"testing"

	"github.com/spf13/pflag"

	"github.com/openshift-online/rh-trex-ai/components/api-server/pkg/config"
)

// TestLogLevelWithEnvironmentDefaults checks that environment flag defaults
// do not hide what the user passed on the command line.
func TestLogLevelWithEnvironmentDefaults(t *testing.T) {
	cases := []struct {
		name     string
		defaults map[string]string
		args     []string
		want     slog.Level
	}{
		{"env default only", map[string]string{"log-level": "debug"}, nil, slog.LevelDebug},
		{"user -v over env log-level", map[string]string{"log-level": "info"}, []string{"-v=10"}, slog.LevelDebug},
		{"user -v=1 over env debug", map[string]string{"log-level": "debug"}, []string{"-v=1"}, slog.LevelInfo},
		{"user --log-level over env", map[string]string{"log-level": "debug"}, []string{"--log-level=warn"}, slog.LevelWarn},
		{"env -v only", map[string]string{"v": "10"}, nil, slog.LevelDebug},
		{"env log-level over env -v", map[string]string{"v": "10", "log-level": "warn"}, nil, slog.LevelWarn},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fs := pflag.NewFlagSet("test", pflag.ContinueOnError)
			c := config.NewLoggingConfig()
			c.AddFlags(fs)
			if err := SetConfigDefaults(fs, tc.defaults); err != nil {
				t.Fatal(err)
			}
			if err := fs.Parse(tc.args); err != nil {
				t.Fatal(err)
			}
			got, err := c.SlogLevel()
			if err != nil || got != tc.want {
				t.Errorf("got %v, %v; want %v", got, err, tc.want)
			}
		})
	}
}

func TestSetConfigDefaults(t *testing.T) {
	fs := pflag.NewFlagSet("test", pflag.ContinueOnError)
	name := fs.String("name", "a", "")
	if err := SetConfigDefaults(fs, map[string]string{"name": "b"}); err != nil {
		t.Fatal(err)
	}
	f := fs.Lookup("name")
	if *name != "b" || f.DefValue != "b" || f.Changed {
		t.Errorf("value %q, default %q, changed %v; want b, b, false", *name, f.DefValue, f.Changed)
	}
	if err := SetConfigDefaults(fs, map[string]string{"missing": "x"}); err == nil {
		t.Error("want an error for an unknown flag")
	}
}
