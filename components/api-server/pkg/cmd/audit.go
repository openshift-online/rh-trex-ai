package cmd

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/openshift-online/rh-trex-ai/components/api-server/pkg/audit"
	"github.com/openshift-online/rh-trex-ai/components/api-server/pkg/config"
	"github.com/openshift-online/rh-trex-ai/components/api-server/pkg/db/db_session"
)

// NewAuditCommand returns the "audit" command group. Its "verify" subcommand
// checks the tamper-evident audit chain (SEC-004).
func NewAuditCommand(serviceName string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "audit",
		Short: "Inspect the " + serviceName + " audit trail",
	}
	cmd.AddCommand(newAuditVerifyCommand(serviceName))
	return cmd
}

func newAuditVerifyCommand(serviceName string) *cobra.Command {
	dbConfig := config.NewDatabaseConfig()
	logConfig := config.NewLoggingConfig()
	auditConfig := config.NewAuditConfig()
	var opts audit.VerifyOptions

	cmd := &cobra.Command{
		Use:   "verify",
		Short: "Verify the " + serviceName + " audit hash chain",
		Long: "Walk the audit chain in order and report gaps, broken links, changed events,\n" +
			"and key mismatches. Exits 0 when the chain is intact and 1 otherwise.",
		Run: func(cmd *cobra.Command, args []string) {
			ok, err := runAuditVerify(cmd, dbConfig, logConfig, auditConfig, opts)
			if err != nil {
				slog.Error("Audit verification failed to run", "error", err)
				os.Exit(1)
			}
			if !ok {
				os.Exit(1)
			}
		},
	}

	fs := cmd.Flags()
	dbConfig.AddFlags(fs)
	fs.AddGoFlagSet(flag.CommandLine)
	logConfig.AddFlags(fs)
	fs.StringVar(&auditConfig.HMACKeyFile, "audit-hmac-key-file", "", "File holding the audit hash-chain HMAC key")
	fs.StringVar(&opts.ExpectHead, "expect-head", "", "Require the chain to contain this event, as <chain_seq>:<hash> (for example the newest event in the SIEM)")
	fs.DurationVar(&opts.MaxUnsealedAge, "max-unsealed-age", 5*time.Minute, "Report events left unsealed for longer than this (0 disables)")
	return cmd
}

func runAuditVerify(cmd *cobra.Command, dbConfig *config.DatabaseConfig, logConfig *config.LoggingConfig,
	auditConfig *config.AuditConfig, opts audit.VerifyOptions) (bool, error) {
	if err := logConfig.Install(); err != nil {
		return false, fmt.Errorf("configuring logging: %w", err)
	}
	if err := dbConfig.ReadFiles(); err != nil {
		return false, fmt.Errorf("reading database configuration: %w", err)
	}
	if err := auditConfig.ReadFiles(); err != nil {
		return false, err
	}
	auditConfig.Enabled = true
	auditConfig.Sink = config.AuditSinkNone

	connection := db_session.NewProdFactory(dbConfig)
	defer func() { _ = connection.Close() }()
	svc, err := audit.New(connection, auditConfig)
	if err != nil {
		return false, err
	}
	res, err := svc.Verify(context.Background(), opts)
	if err != nil {
		return false, err
	}

	out := cmd.OutOrStdout()
	_, _ = fmt.Fprintf(out, "checked: %d events\nhead: %d %s\nkey_id: %s\nunsealed: %d\n",
		res.Checked, res.HeadSeq, res.HeadHash, svc.KeyID(), res.Unsealed)
	for _, p := range res.Problems {
		_, _ = fmt.Fprintf(out, "PROBLEM: %s\n", p)
	}
	if res.OK() {
		_, _ = fmt.Fprintln(out, "result: intact")
	} else {
		_, _ = fmt.Fprintln(out, "result: FAILED")
	}
	return res.OK(), nil
}
