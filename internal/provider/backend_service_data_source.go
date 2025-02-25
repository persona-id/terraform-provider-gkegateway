// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	compute "cloud.google.com/go/compute/apiv1"
	"cloud.google.com/go/compute/apiv1/computepb"
	"github.com/googleapis/gax-go/v2/apierror"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"google.golang.org/api/iterator"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ datasource.DataSource = &BackendServiceDataSource{}

func NewBackendServiceDataSource() datasource.DataSource {
	return &BackendServiceDataSource{}
}

// BackendServiceDataSource defines the data source implementation.
type BackendServiceDataSource struct {
	providerData *GKEGatewayProviderData
}

// BackendServiceDataSourceModel describes the data source data model.
type BackendServiceDataSourceModel struct {
	BackendService *BackendServiceDataSourceModelBackendService `tfsdk:"backend_service"`
	Gateway        types.String                                 `tfsdk:"gateway"`
	Namespace      types.String                                 `tfsdk:"namespace"`
	Project        types.String                                 `tfsdk:"project"`
	Region         types.String                                 `tfsdk:"region"`
}

type BackendServiceDataSourceModelBackendService struct {
	ID   types.String `tfsdk:"id"`
	Name types.String `tfsdk:"name"`
}

type forwardingRuleDescription struct {
	K8sResource *string `json:"k8sResource"`
}

func (d *BackendServiceDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	// Prevent panic if the provider has not been configured.
	if req.ProviderData == nil {
		return
	}

	data, ok := req.ProviderData.(*GKEGatewayProviderData)

	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Data Source Configure Type",
			fmt.Sprintf("Expected *GKEGatewayProviderData, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)

		return
	}

	d.providerData = data
}

func (d *BackendServiceDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_backend_service"
}

func (d *BackendServiceDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var (
		data BackendServiceDataSourceModel
		err  error
	)

	// Read Terraform configuration data into the model
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// project can be set at the provider or data source level, the latter taking precedence, but ultimately required.
	if d.providerData.project.IsNull() && data.Project.IsNull() {
		resp.Diagnostics.AddError("Missing project", "The project field must be set on either the provider or data source.")
		return
	}

	if data.Project.IsUnknown() {
		resp.Diagnostics.AddError("Unknown project", "The project field on the data source cannot be set to an unknown value")
		return
	}

	project := d.providerData.project.ValueString()
	if !data.Project.IsNull() {
		project = data.Project.ValueString()
	}

	// region can be set at the provider, data source level, or not at all.
	if data.Region.IsUnknown() {
		resp.Diagnostics.AddError("Unknown region", "The region field on the data source cannot be set to an unknown value")
		return
	}

	region := d.providerData.region
	if !data.Region.IsNull() {
		region = data.Region
	}

	// Loop over the forwarding rules.
	var forwardingRulesIterator *compute.ForwardingRuleIterator
	if region.IsNull() {
		forwardingRulesIterator = d.providerData.globalForwardingRulesClient.List(ctx, &computepb.ListGlobalForwardingRulesRequest{
			Project: project,
		})
	} else {
		forwardingRulesIterator = d.providerData.forwardingRulesClient.List(ctx, &computepb.ListForwardingRulesRequest{
			Project: project,
			Region:  region.ValueString(),
		})
	}

	matchingForwardingRules := make([]*computepb.ForwardingRule, 0)

	for {
		forwardingRule, err := forwardingRulesIterator.Next()

		if err == iterator.Done {
			break
		}

		if err != nil {
			// Ignore 404 errors for projects that don't exist yet.
			if e, ok := err.(*apierror.APIError); ok && e.HTTPCode() == 404 {
				return
			}

			resp.Diagnostics.AddError("Unable to iterate over forwarding rules", fmt.Sprintf("Error calling Google API: %+v", err))
			return
		}

		// Most rules won't have a JSON description.
		frd := forwardingRuleDescription{}
		if err := json.Unmarshal([]byte(forwardingRule.GetDescription()), &frd); err == nil {
			if frd.K8sResource != nil && *frd.K8sResource == fmt.Sprintf("/namespaces/%s/gateways/%s", data.Namespace.ValueString(), data.Gateway.ValueString()) {
				matchingForwardingRules = append(matchingForwardingRules, forwardingRule)
			}
		}
	}

	if len(matchingForwardingRules) == 0 {
		return
	} else if len(matchingForwardingRules) > 1 {
		debugMessage := "The following forwarding rules matched:\n\n"
		for _, rule := range matchingForwardingRules {
			debugMessage = fmt.Sprintf("%s  - %s\n", debugMessage, rule.GetName())
		}

		resp.Diagnostics.AddError("Multiple matching forwarding rules found", debugMessage)
		return
	}

	// Lookup the target.
	targetComponents := strings.Split(matchingForwardingRules[0].GetTarget(), "/")

	var urlMapResource string

	switch targetComponents[len(targetComponents)-2] {
	case "targetHttpsProxies":
		var proxy *computepb.TargetHttpsProxy

		if region.IsNull() {
			proxy, err = d.providerData.targetHttpsProxiesClient.Get(ctx, &computepb.GetTargetHttpsProxyRequest{
				Project:          project,
				TargetHttpsProxy: targetComponents[len(targetComponents)-1],
			})
		} else {
			proxy, err = d.providerData.regionTargetHttpsProxiesClient.Get(ctx, &computepb.GetRegionTargetHttpsProxyRequest{
				Project:          project,
				Region:           region.ValueString(),
				TargetHttpsProxy: targetComponents[len(targetComponents)-1],
			})
		}

		if err != nil {
			resp.Diagnostics.AddError(fmt.Sprintf("Error looking up HTTPS target proxy %s", targetComponents[len(targetComponents)-1]), fmt.Sprintf("Error calling Google API: %+v", err))
			return
		}

		urlMapResource = proxy.GetUrlMap()
	default:
		resp.Diagnostics.AddError("Unsupported target type for forwarding rule", fmt.Sprintf("The %s forwarding rule has a target with a type of %s which is currently unsupported by this provider.", matchingForwardingRules[0].GetName(), targetComponents[len(targetComponents)-2]))
		return
	}

	// Lookup the URL map.
	urlMapComponents := strings.Split(urlMapResource, "/")

	var urlMap *computepb.UrlMap
	if region.IsNull() {
		urlMap, err = d.providerData.urlMapsClient.Get(ctx, &computepb.GetUrlMapRequest{
			Project: project,
			UrlMap:  urlMapComponents[len(urlMapComponents)-1],
		})
	} else {
		urlMap, err = d.providerData.regionUrlMapsClient.Get(ctx, &computepb.GetRegionUrlMapRequest{
			Project: project,
			Region:  region.ValueString(),
			UrlMap:  urlMapComponents[len(urlMapComponents)-1],
		})
	}

	if err != nil {
		resp.Diagnostics.AddError(fmt.Sprintf("Error looking up URL map %s", urlMapComponents[len(urlMapComponents)-1]), fmt.Sprintf("Error calling Google API: %+v", err))
		return
	}

	// Prase the URL map to determine eligible backend services
	backendServicePaths := []*string{}
	routeActions := []*computepb.HttpRouteAction{
		urlMap.DefaultRouteAction,
	}

	if urlMap.DefaultService != nil {
		backendServicePaths = append(backendServicePaths, urlMap.DefaultService)
	}

	for _, matcher := range urlMap.PathMatchers {
		routeActions = append(routeActions, matcher.DefaultRouteAction)

		if matcher.DefaultService != nil {
			backendServicePaths = append(backendServicePaths, matcher.DefaultService)
		}

		for _, rule := range matcher.RouteRules {
			routeActions = append(routeActions, rule.RouteAction)
		}
	}

	for _, action := range routeActions {
		if action == nil || action.FaultInjectionPolicy != nil {
			continue
		}

		for _, wbs := range action.WeightedBackendServices {
			backendServicePaths = append(backendServicePaths, wbs.BackendService)
		}
	}

	if len(backendServicePaths) == 0 {
		resp.Diagnostics.AddError("No backend services found", "")
		return
	} else if len(backendServicePaths) > 1 {
		debugMessage := "The following backend services matched:\n\n"
		for _, path := range backendServicePaths {
			components := strings.Split(*path, "/")
			debugMessage = fmt.Sprintf("%s  - %s\n", debugMessage, components[len(components)-1])
		}

		resp.Diagnostics.AddError("Multiple backend services found", debugMessage)
		return
	}

	// Finally, lookup the backend service.
	backendServiceComponents := strings.Split(*backendServicePaths[0], "/")

	var backendService *computepb.BackendService
	if region.IsNull() {
		backendService, err = d.providerData.backendServicesClient.Get(ctx, &computepb.GetBackendServiceRequest{
			BackendService: backendServiceComponents[len(backendServiceComponents)-1],
			Project:        project,
		})
	} else {
		backendService, err = d.providerData.regionBackendServicesClient.Get(ctx, &computepb.GetRegionBackendServiceRequest{
			BackendService: backendServiceComponents[len(backendServiceComponents)-1],
			Region:         region.ValueString(),
			Project:        project,
		})
	}

	if err != nil {
		resp.Diagnostics.AddError(fmt.Sprintf("Error looking up backend service %s", backendServiceComponents[len(backendServiceComponents)-1]), fmt.Sprintf("Error calling Google API: %+v", err))
		return
	}

	// Save data into Terraform state
	data.BackendService = &BackendServiceDataSourceModelBackendService{
		ID:   types.StringValue(strconv.FormatUint(backendService.GetId(), 10)),
		Name: types.StringValue(backendService.GetName()),
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (d *BackendServiceDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"backend_service": schema.SingleNestedAttribute{
				Attributes: map[string]schema.Attribute{
					"id": schema.StringAttribute{
						Computed:            true,
						MarkdownDescription: "Identifier for the backend service with format `projects/{{project}}/global/backendServices/{{name}}` or `projects/{{project}}/regions/{{region}}/backendServices/{{name}}`.",
					},
					"name": schema.StringAttribute{
						Computed:            true,
						MarkdownDescription: "Name of the backend service.",
					},
				},
				Computed:            true,
				MarkdownDescription: "Details about the backend service - will be null if none is found.",
			},
			"gateway": schema.StringAttribute{
				MarkdownDescription: "Name of the Kubernetes gateway resource.",
				Required:            true,
			},
			"namespace": schema.StringAttribute{
				MarkdownDescription: "Name of the Kubernetes namespace the gateway resource is in.",
				Required:            true,
			},
			"project": schema.StringAttribute{
				MarkdownDescription: "The ID of the project in which the load balancer belongs. If it is not provided, the provider project is used.",
				Optional:            true,
			},
			"region": schema.StringAttribute{
				MarkdownDescription: "The region in which the load balancer belongs. If it is not provided, the provider region is used. When neither are provided, the load balancer is presumed to be global.",
				Optional:            true,
			},
		},
		MarkdownDescription: "Finds the backend service details for the load balancer created from a Kubernetes Gateway resource by GKE. This assumes the Gateway only has one Service but is untested with multiple HTTPRoutes.",
	}
}
