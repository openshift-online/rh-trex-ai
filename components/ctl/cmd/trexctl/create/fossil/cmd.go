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
	Use:   "fossil [flags]",
	Short: "Create a fossil",
	Long: "Create a new fossil.\n\n" +
		"Examples:\n" +
		"  trexctl create fossil --discovery-location <value> --estimated-age <value> --excavator-name <value> --fossil-type <value> \n" +
		"  trexctl create fossil --body request.json",
	Args: cobra.NoArgs,
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
		if args.discoveryLocation != "" {
			request["discovery_location"] = args.discoveryLocation
		}
		if args.estimatedAge != 0 {
			request["estimated_age"] = args.estimatedAge
		}
		if args.excavatorName != "" {
			request["excavator_name"] = args.excavatorName
		}
		if args.fossilType != "" {
			request["fossil_type"] = args.fossilType
		}
		body, err = json.Marshal(request)
		if err != nil {
			return fmt.Errorf("can't marshal request: %v", err)
		}
	}

	resp, err := conn.Post(urls.FossilsPath, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("can't create fossil: %v", err)
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
