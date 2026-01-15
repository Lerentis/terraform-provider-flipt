// Copyright (c) terraform-provider-flipt contributors
// SPDX-License-Identifier: MIT

package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

var _ datasource.DataSource = &SegmentDataSource{}

func NewSegmentDataSource() datasource.DataSource {
	return &SegmentDataSource{}
}

type SegmentDataSource struct {
	httpClient *http.Client
	endpoint   string
}

type SegmentDataSourceModel struct {
	NamespaceKey   types.String `tfsdk:"namespace_key"`
	EnvironmentKey types.String `tfsdk:"environment_key"`
	Key            types.String `tfsdk:"key"`
	Name           types.String `tfsdk:"name"`
	Description    types.String `tfsdk:"description"`
	MatchType      types.String `tfsdk:"match_type"`
}

func (d *SegmentDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_segment"
}

func (d *SegmentDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Flipt segment data source",
		Description:         "Flipt segment data source",

		Attributes: map[string]schema.Attribute{
			"namespace_key": schema.StringAttribute{
				MarkdownDescription: "Namespace key",
				Description:         "Namespace key",
				Required:            true,
			},
			"environment_key": schema.StringAttribute{
				MarkdownDescription: "Environment key (defaults to 'default' if not specified)",
				Description:         "Environment key (defaults to 'default' if not specified)",
				Optional:            true,
			},
			"key": schema.StringAttribute{
				MarkdownDescription: "Segment key",
				Description:         "Segment key",
				Required:            true,
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "Segment name",
				Description:         "Segment name",
				Computed:            true,
			},
			"description": schema.StringAttribute{
				MarkdownDescription: "Segment description",
				Description:         "Segment description",
				Computed:            true,
			},
			"match_type": schema.StringAttribute{
				MarkdownDescription: "Segment match type",
				Description:         "Segment match type",
				Computed:            true,
			},
		},
	}
}

func (d *SegmentDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	providerConfig, ok := req.ProviderData.(*FliptProviderConfig)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Data Source Configure Type",
			fmt.Sprintf("Expected *FliptProviderConfig, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)
		return
	}

	d.httpClient = providerConfig.HTTPClient
	d.endpoint = providerConfig.Endpoint
}

func (d *SegmentDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data SegmentDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Determine environment key (default to "default" if not specified)
	envKey := "default"
	if !data.EnvironmentKey.IsNull() && !data.EnvironmentKey.IsUnknown() {
		envKey = data.EnvironmentKey.ValueString()
	}

	tflog.Debug(ctx, "Reading segment data source", map[string]interface{}{
		"environment_key": envKey,
		"namespace_key":   data.NamespaceKey.ValueString(),
		"segment_key":     data.Key.ValueString(),
	})

	url := fmt.Sprintf("%s/api/v2/environments/%s/namespaces/%s/resources/flipt.core.Segment/%s",
		d.endpoint, envKey, data.NamespaceKey.ValueString(), data.Key.ValueString())

	httpReq, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		resp.Diagnostics.AddError("Request Error", fmt.Sprintf("Unable to create request: %s", err))
		return
	}

	httpResp, err := d.httpClient.Do(httpReq)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to read segment, got error: %s", err))
		return
	}
	defer httpResp.Body.Close()

	body, err := io.ReadAll(httpResp.Body)
	if err != nil {
		resp.Diagnostics.AddError("Read Error", fmt.Sprintf("Unable to read response: %s", err))
		return
	}

	if httpResp.StatusCode != http.StatusOK {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to read segment, status: %d, body: %s", httpResp.StatusCode, string(body)))
		return
	}

	var segmentResponse struct {
		Resource struct {
			Payload struct {
				Key         string `json:"key"`
				Name        string `json:"name"`
				Description string `json:"description"`
				MatchType   string `json:"matchType"`
			} `json:"payload"`
		} `json:"resource"`
	}

	if err := json.Unmarshal(body, &segmentResponse); err != nil {
		resp.Diagnostics.AddError("Parse Error", fmt.Sprintf("Unable to parse response: %s", err))
		return
	}

	data.Name = types.StringValue(segmentResponse.Resource.Payload.Name)

	if segmentResponse.Resource.Payload.Description != "" {
		data.Description = types.StringValue(segmentResponse.Resource.Payload.Description)
	} else {
		data.Description = types.StringNull()
	}

	data.MatchType = types.StringValue(segmentResponse.Resource.Payload.MatchType)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
