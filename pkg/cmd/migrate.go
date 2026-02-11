package cmd

import (
	"context"
	"flag"

	"github.com/golang/glog"
	"github.com/spf13/cobra"

	"github.com/openshift-online/rh-trex-ai/pkg/config"
	"github.com/openshift-online/rh-trex-ai/pkg/db"
	"github.com/openshift-online/rh-trex-ai/pkg/db/db_session"
)

func NewMigrateCommand(serviceName string) *cobra.Command {
	dbConfig := config.NewDatabaseConfig()

	cmd := &cobra.Command{
		Use:   "migrate",
		Short: "Run " + serviceName + " service data migrations",
		Long:  "Run " + serviceName + " service data migrations",
		Run: func(cmd *cobra.Command, args []string) {
			err := dbConfig.ReadFiles()
			if err != nil {
				glog.Fatal(err)
			}

			connection := db_session.NewProdFactory(dbConfig)
			if err := db.Migrate(connection.New(context.Background())); err != nil {
				glog.Fatal(err)
			}
		},
	}

	dbConfig.AddFlags(cmd.PersistentFlags())
	cmd.PersistentFlags().AddGoFlagSet(flag.CommandLine)
	return cmd
}
