package scientist

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"

	"github.com/openshift-online/rh-trex-ai/components/ctl/pkg/config"
	"github.com/openshift-online/rh-trex-ai/components/ctl/pkg/connection"
	"github.com/openshift-online/rh-trex-ai/components/ctl/pkg/dump"
	"github.com/openshift-online/rh-trex-ai/components/ctl/pkg/urls"
)

var args struct {
	field string
	name string
	bodyFile string
}

var Cmd = &cobra.Command{
	Use:   "scientist [flags]",
	Short: "Create a scientist",
	Long: "Create a new scientist.\n\n" +
		"Examples:\n" +
		"  trexctl create scientist --field <value> --name <value> \n" +
		"  trexctl create scientist --body request.json",
	Args: cobra.NoArgs,
	RunE: run,
}

func init() {
	fs := Cmd.Flags()
	fs.StringVar(&args.field, "field", "", "field value.")
	fs.StringVar(&args.name, "name", "", "name value.")
	fs.StringVar(&args.bodyFile, "body", "", "File containing the request body as JSON.")
}

func run(cmd *cobra.Command, argv []string) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	conn, err := connection.NewConnectionBuilder().Config(cfg).Build()
	if err != nil {
		return err
	}
	defer conn.Close()

	var body []byte

	if args.bodyFile != "" {
		body, err = os.ReadFile(args.bodyFile)
		if err != nil {
			return fmt.Errorf("can't read body file: %v", err)
		}
	} else {
		request := map[string]interface{}{}
		if args.field != "" {
			request["field"] = args.field
		}
		if args.name != "" {
			request["name"] = args.name
		}
		body, err = json.Marshal(request)
		if err != nil {
			return fmt.Errorf("can't marshal request: %v", err)
		}
	}

	resp, err := conn.Post(urls.ScientistsPath, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("can't create scientist: %v", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("can't read response: %v", err)
	}

	if resp.StatusCode != 201 && resp.StatusCode != 200 {
		return fmt.Errorf("API returned %d: %s", resp.StatusCode, string(respBody))
	}

	return dump.Pretty(os.Stdout, respBody)
}
