/*
Copyright (c) 2018 Red Hat, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

  http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package integration

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"

	. "github.com/onsi/gomega"
	"gopkg.in/resty.v1"

	"github.com/openshift-online/rh-trex-ai/pkg/api"
	"github.com/openshift-online/rh-trex-ai/pkg/trex"
	"github.com/openshift-online/rh-trex-ai/test"
)

func TestMetadataGet(t *testing.T) {
	h, _ := test.RegisterIntegration(t)

	apiPrefix := strings.TrimSuffix(trex.GetConfig().BasePath, "/v1")
	protocol := "http"
	if h.AppConfig.Server.EnableHTTPS {
		protocol = "https"
	}
	metadataURL := fmt.Sprintf("%s://%s%s", protocol, h.AppConfig.Server.BindAddress, apiPrefix)
	resp, err := resty.R().
		SetHeader("Content-Type", "application/json").
		Get(metadataURL)

	Expect(err).NotTo(HaveOccurred(), "Error getting metadata: %v", err)
	Expect(resp.StatusCode()).To(Equal(http.StatusOK))

	// Parse the response body
	var metadata api.Metadata
	err = json.Unmarshal(resp.Body(), &metadata)
	Expect(err).NotTo(HaveOccurred(), "Error parsing metadata response: %v", err)
	//
	// Verify content type header
	contentType := resp.Header().Get("Content-Type")
	Expect(contentType).To(Equal("application/json"), "Expected Content-Type to be application/json")

	// Verify all metadata fields
	Expect(metadata.ID).To(Equal(trex.GetConfig().MetadataID))
	Expect(metadata.Kind).To(Equal("API"))
	Expect(metadata.HREF).To(Equal(apiPrefix))
	Expect(metadata.Version).NotTo(BeEmpty(), "Expected Version to be set")
	Expect(metadata.BuildTime).NotTo(BeEmpty(), "Expected BuildTime to be set")
}
