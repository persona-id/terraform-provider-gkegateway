// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"fmt"

	compute "cloud.google.com/go/compute/apiv1"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// Ensure GKEGatewayProvider satisfies various provider interfaces.
var _ provider.Provider = &GKEGatewayProvider{}

// GKEGatewayProvider defines the provider implementation.
type GKEGatewayProvider struct {
	// version is set to the provider version on release, "dev" when the
	// provider is built and ran locally, and "test" when running acceptance
	// testing.
	version string
}

type GKEGatewayProviderData struct {
	backendServicesClient          *compute.BackendServicesClient
	forwardingRulesClient          *compute.ForwardingRulesClient
	globalForwardingRulesClient    *compute.GlobalForwardingRulesClient
	project                        types.String
	region                         types.String
	regionBackendServicesClient    *compute.RegionBackendServicesClient
	regionTargetHttpsProxiesClient *compute.RegionTargetHttpsProxiesClient
	regionUrlMapsClient            *compute.RegionUrlMapsClient
	targetHttpsProxiesClient       *compute.TargetHttpsProxiesClient
	urlMapsClient                  *compute.UrlMapsClient
}

// GKEGatewayProviderModel describes the provider data model.
type GKEGatewayProviderModel struct {
	Project types.String `tfsdk:"project"`
	Region  types.String `tfsdk:"region"`
}

func New(version string) func() provider.Provider {
	return func() provider.Provider {
		return &GKEGatewayProvider{
			version: version,
		}
	}
}

func (p *GKEGatewayProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	var data GKEGatewayProviderModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	if data.Project.IsUnknown() {
		resp.Diagnostics.AddError("Unknown project", "The project field on the provider cannot be set to an unknown value")
		return
	}

	if data.Region.IsUnknown() {
		resp.Diagnostics.AddError("Unknown region", "The region field on the provider cannot be set to an unknown value")
		return
	}

	backendServicesClient, err := compute.NewBackendServicesRESTClient(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Unable to configure provider", fmt.Sprintf("Error setting up Google BackendServices : %+v", err))
		return
	}

	forwardingRulesClient, err := compute.NewForwardingRulesRESTClient(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Unable to configure provider", fmt.Sprintf("Error setting up Google Forwarding Rules client: %+v", err))
		return
	}

	globalForwardingRulesClient, err := compute.NewGlobalForwardingRulesRESTClient(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Unable to configure provider", fmt.Sprintf("Error setting up Google Global Forwarding Rules client: %+v", err))
		return
	}

	regionBackendServicesClient, err := compute.NewRegionBackendServicesRESTClient(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Unable to configure provider", fmt.Sprintf("Error setting up Google Regional Backend Services client: %+v", err))
		return
	}

	regionTargetHttpsProxiesClient, err := compute.NewRegionTargetHttpsProxiesRESTClient(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Unable to configure provider", fmt.Sprintf("Error setting up Google Regional Target HTTPS Proxies client: %+v", err))
		return
	}

	regionUrlMapsClient, err := compute.NewRegionUrlMapsRESTClient(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Unable to configure provider", fmt.Sprintf("Error setting up Google Regional URL Maps client: %+v", err))
		return
	}

	targetHttpsProxiesClient, err := compute.NewTargetHttpsProxiesRESTClient(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Unable to configure provider", fmt.Sprintf("Error setting up Google Target HTTPS Proxies client: %+v", err))
		return
	}

	urlMapsClient, err := compute.NewUrlMapsRESTClient(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Unable to configure provider", fmt.Sprintf("Error setting up Google URL Maps client: %+v", err))
		return
	}

	providerData := &GKEGatewayProviderData{
		backendServicesClient:          backendServicesClient,
		forwardingRulesClient:          forwardingRulesClient,
		globalForwardingRulesClient:    globalForwardingRulesClient,
		project:                        data.Project,
		region:                         data.Region,
		regionBackendServicesClient:    regionBackendServicesClient,
		regionTargetHttpsProxiesClient: regionTargetHttpsProxiesClient,
		regionUrlMapsClient:            regionUrlMapsClient,
		targetHttpsProxiesClient:       targetHttpsProxiesClient,
		urlMapsClient:                  urlMapsClient,
	}
	resp.DataSourceData = providerData
	resp.ResourceData = providerData
}

func (p *GKEGatewayProvider) DataSources(ctx context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{
		NewBackendServiceDataSource,
	}
}

func (p *GKEGatewayProvider) Metadata(ctx context.Context, req provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "gkegateway"
	resp.Version = p.version
}

func (p *GKEGatewayProvider) Resources(ctx context.Context) []func() resource.Resource {
	return []func() resource.Resource{}
}

func (p *GKEGatewayProvider) Schema(ctx context.Context, req provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"project": schema.StringAttribute{
				MarkdownDescription: "The ID of the project in which the resources belong. If another project is specified on the data block, it will take precedence.",
				Optional:            true,
			},
			"region": schema.StringAttribute{
				MarkdownDescription: "The region in which the resources belong. If another region is specified on the data block, it will take precedence. When not provided, the resources are presumed to be global.",
				Optional:            true,
			},
		},
		MarkdownDescription: "The GKE Gateway provider is used to lookup GCP load balancing resources created by Kubernetes Gateway resources.",
	}
}
