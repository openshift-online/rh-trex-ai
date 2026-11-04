package config

import "github.com/spf13/pflag"

// removedFlags belonged to the OCM/AMS API client, which was removed because
// nothing used it. They stay registered as hidden, deprecated no-ops so that
// old command lines and old environment Flags() maps still start. Using one
// prints a deprecation warning. They will be deleted in a later release.
var removedFlags = []struct {
	name   string
	isBool bool
}{
	{"enable-mock", true},
	{"debug", true},
	{"api-client-id-file", false},
	{"api-client-secret-file", false},
	{"self-token-file", false},
	{"api-base-url", false},
	{"api-token-url", false},
}

const removedFlagMessage = "the OCM/AMS API client was removed; this flag does nothing and will be deleted in a later release"

func addRemovedFlags(fs *pflag.FlagSet) {
	for _, f := range removedFlags {
		if fs.Lookup(f.name) != nil {
			continue
		}
		if f.isBool {
			fs.Bool(f.name, false, "Deprecated: no effect")
		} else {
			fs.String(f.name, "", "Deprecated: no effect")
		}
		_ = fs.MarkDeprecated(f.name, removedFlagMessage)
	}
}
