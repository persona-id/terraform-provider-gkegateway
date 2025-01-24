// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccBackendServiceDataSourceValidations(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// missing fields
			{
				Config: `
					data "gkegateway_backend_service" "example" {
						namespace = "my-cool-app"
						project   = "my-gcp-project"
						region    = "us-central1"
					}
				`,
				ExpectError: regexp.MustCompile(`The argument "gateway" is required, but no definition was found.`),
			},
			{
				Config: `
					data "gkegateway_backend_service" "example" {
						gateway   = "my-gateway-name"
						project   = "my-gcp-project"
						region    = "us-central1"
					}
				`,
				ExpectError: regexp.MustCompile(`The argument "namespace" is required, but no definition was found.`),
			},
			{
				Config: `
					data "gkegateway_backend_service" "example" {
						gateway   = "my-gateway-name"
						namespace = "my-cool-app"
						region    = "us-central1"
					}
				`,
				ExpectError: regexp.MustCompile(`The project field must be set on either the provider or data source.`),
			},
		},
	})
}
