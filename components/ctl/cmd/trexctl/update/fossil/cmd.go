package fossil

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
	discoveryLocation string
	estimatedAge int
	excavatorName string
	fossilType string
	bodyFile string
}

var Cmd = &cobra.Command{
	Use:     "fossil ID [flags]",
	Aliases: []string{"fossils"},
	Short:   "Update a fossil by ID",
	Long: "Update a fossil by ID using PATCH. Only the flags that are set are sent.\n\n" +
		"Examples:\n" +
		"  trexctl update fossil ID --discovery-location <value> --estimated-age <value> --excavator-name <value> --fossil-type <value>\n" +
		"  trexctl update fossil ID --body request.json",
	Args: cobra.ExactArgs(1),
	RunE: run,
}

func init() {
	fs := Cmd.Flags()
	fs.StringVar(&args.discoveryLocation, "discovery-location", "", "discovery_location value.")
	fs.IntVar(&args.estimatedAge, "estimated-age", 0, "estimated_age value.")
	fs.StringVar(&args.excavatorName, "excavator-name", "", "excavator_name value.")
	fs.StringVar(&args.fossilType, "fossil-type", "", "fossil_type value.")
	fs.StringVar(&args.bodyFile, "body", "", "File containing the request body as JSON.")
}

func run(cmd *cobra.Command, argv []string) error {
	id := argv[0]

	var body []byte
	var err error
	if args.bodyFile != "" {
		body, err = os.ReadFile(args.bodyFile)
		if err != nil {
			return fmt.Errorf("can't read body file: %v", err)
		}
	} else {
		request := map[string]interface{}{}
		if cmd.Flags().Changed("discovery-location") {
			request["discovery_location"] = args.discoveryLocation
		}
		if cmd.Flags().Changed("estimated-age") {
			request["estimated_age"] = args.estimatedAge
		}
		if cmd.Flags().Changed("excavator-name") {
			request["excavator_name"] = args.excavatorName
		}
		if cmd.Flags().Changed("fossil-type") {
			request["fossil_type"] = args.fossilType
		}
		if len(request) == 0 {
			return fmt.Errorf("nothing to update: set at least one field flag or pass --body")
		}
		body, err = json.Marshal(request)
		if err != nil {
			return fmt.Errorf("can't marshal request: %v", err)
		}
	}

	cfg, err := config.Load()
	if err != nil {
		return err
	}

	conn, err := connection.NewConnectionBuilder().Config(cfg).Build()
	if err != nil {
		return err
	}
	defer conn.Close()


	resp, err := conn.Patch(urls.FossilPath(id), bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("can't update fossil: %v", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("can't read response: %v", err)
	}

	switch resp.StatusCode {
	case 200, 202:
	case 204:
		fmt.Fprintf(os.Stdout, "Fossil %s updated.\n", id)
		return nil
	default:
		return fmt.Errorf("API returned %d: %s", resp.StatusCode, string(respBody))
	}

	return dump.Pretty(os.Stdout, respBody)
}
