package dinosaur

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
	species string
	bodyFile string
}

var Cmd = &cobra.Command{
	Use:   "dinosaur [flags]",
	Short: "Create a dinosaur",
	Long: "Create a new dinosaur.\n\n" +
		"Examples:\n" +
		"  trexctl create dinosaur --species <value> \n" +
		"  trexctl create dinosaur --body request.json",
	Args: cobra.NoArgs,
	RunE: run,
}

func init() {
	fs := Cmd.Flags()
	fs.StringVar(&args.species, "species", "", "species value.")
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
		if args.species != "" {
			request["species"] = args.species
		}
		body, err = json.Marshal(request)
		if err != nil {
			return fmt.Errorf("can't marshal request: %v", err)
		}
	}

	resp, err := conn.Post(urls.DinosaursPath, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("can't create dinosaur: %v", err)
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
