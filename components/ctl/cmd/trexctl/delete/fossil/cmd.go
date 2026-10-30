package fossil

import (
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"

	"github.com/openshift-online/rh-trex-ai/components/ctl/pkg/config"
	"github.com/openshift-online/rh-trex-ai/components/ctl/pkg/confirm"
	"github.com/openshift-online/rh-trex-ai/components/ctl/pkg/connection"
	"github.com/openshift-online/rh-trex-ai/components/ctl/pkg/urls"
)

// ConfirmPrompt returns the confirmation question for one resource. A project
// can replace it, for example from its hand-maintained main.go, to customize the
// wording without replacing the command.
var ConfirmPrompt = func(id string) string {
	return "Delete fossil " + id + "?"
}

var args struct {
	yes bool
}

var Cmd = &cobra.Command{
	Use:     "fossil ID [flags]",
	Aliases: []string{"fossils"},
	Short:   "Delete a fossil by ID",
	Long: "Delete a fossil by ID after asking for confirmation.\n\n" +
		"Without a terminal on standard input the command refuses to proceed unless --yes is given.",
	Args: cobra.ExactArgs(1),
	RunE: run,
}

func init() {
	Cmd.Flags().BoolVar(&args.yes, "yes", false, "Skip the confirmation prompt.")
}

func run(cmd *cobra.Command, argv []string) error {
	id := argv[0]

	cfg, err := config.Load()
	if err != nil {
		return err
	}

	if err := confirm.Ask(ConfirmPrompt(id), args.yes); err != nil {
		return err
	}

	conn, err := connection.NewConnectionBuilder().Config(cfg).Build()
	if err != nil {
		return err
	}
	defer conn.Close()

	resp, err := conn.Delete(urls.FossilPath(id))
	if err != nil {
		return fmt.Errorf("can't delete fossil: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("can't read response: %v", err)
	}

	switch resp.StatusCode {
	case 200, 202, 204:
	default:
		return fmt.Errorf("API returned %d: %s", resp.StatusCode, string(body))
	}

	fmt.Fprintf(os.Stdout, "Fossil %s deleted.\n", id)
	return nil
}
