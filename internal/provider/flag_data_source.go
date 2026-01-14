// Copyright (c) terraform-flipt contributors
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

var _ datasource.DataSource = &FlagDataSource{}

func NewFlagDataSource() datasource.DataSource {
	return &FlagDataSource{}
}

type FlagDataSource struct {
	httpClient *http.Client
	endpoint   string
}

type FlagDataSourceModel struct {
	NamespaceKey   types.String `tfsdk:"namespace_key"`
	EnvironmentKey types.String `tfsdk:"environment_key"`
	Key            types.String `tfsdk:"key"`
	Name           types.String `tfsdk:"name"`
	Description    types.String `tfsdk:"description"`
	Enabled        types.Bool   `tfsdk:"enabled"`
	Type           types.String `tfsdk:"type"`
	Metadata       types.Map    `tfsdk:"metadata"`
}

func (d *FlagDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_flag"
}

func (d *FlagDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Flipt flag data source",
		Description:         "Flipt flag data source",

		Attributes: map[string]schema.Attribute{
			"namespace_key": schema.StringAttribute{
				MarkdownDescription: "Namespace key where the flag belongs",
				Required:            true,
			},
			"environment_key": schema.StringAttribute{
				MarkdownDescription: "Environment key (defaults to 'default' if not specified)",
				Optional:            true,
			},
			"key": schema.StringAttribute{
				MarkdownDescription: "Unique key for the flag",
				Required:            true,
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "Display name of the flag",
				Computed:            true,
			},
			"description": schema.StringAttribute{
				MarkdownDescription: "Description of the flag",
				Computed:            true,
			},
			"enabled": schema.BoolAttribute{
				MarkdownDescription: "Whether the flag is enabled",
				Computed:            true,
			},
			"type": schema.StringAttribute{
				MarkdownDescription: "Type of the flag (VARIANT_FLAG_TYPE or BOOLEAN_FLAG_TYPE)",
				Computed:            true,
			},
			"metadata": schema.MapAttribute{
				MarkdownDescription: "Metadata key-value pairs for the flag",
				Computed:            true,
				ElementType:         types.StringType,
			},
		},
	}
}

func (d *FlagDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	providerConfig, ok := req.ProviderData.(*FliptProviderConfig)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Data Source Configure Type",
			fmt.Sprintf("Expected *FliptProviderConfig, got: %T", req.ProviderData),
		)
		return
	}

	d.httpClient = providerConfig.HTTPClient
	d.endpoint = providerConfig.Endpoint
}

func (d *FlagDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data FlagDataSourceModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Determine environment key (default to "default" if not specified)
	envKey := "default"
	if !data.EnvironmentKey.IsNull() && !data.EnvironmentKey.IsUnknown() {
		envKey = data.EnvironmentKey.ValueString()
	}

	tflog.Debug(ctx, "Reading flag", map[string]interface{}{
		"environment_key": envKey,
		"namespace_key":   data.NamespaceKey.ValueString(),
		"key":             data.Key.ValueString(),
	})

	// GET URL includes flipt.core.Flag prefix
	url := fmt.Sprintf("%s/api/v2/environments/%s/namespaces/%s/resources/flipt.core.Flag/%s", d.endpoint, envKey, data.NamespaceKey.ValueString(), data.Key.ValueString())
	httpReq, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		resp.Diagnostics.AddError("Request Error", fmt.Sprintf("Unable to create request: %s", err))
		return
	}

	httpResp, err := d.httpClient.Do(httpReq)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to read flag, got error: %s", err))
		return
	}
	defer httpResp.Body.Close()

	if httpResp.StatusCode == http.StatusNotFound {
		resp.Diagnostics.AddError("Not Found", fmt.Sprintf("Flag with key '%s' not found in namespace '%s'", data.Key.ValueString(), data.NamespaceKey.ValueString()))
		return
	}

	body, err := io.ReadAll(httpResp.Body)
	if err != nil {
		resp.Diagnostics.AddError("Read Error", fmt.Sprintf("Unable to read response: %s", err))
		return
	}

	if httpResp.StatusCode != http.StatusOK {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to read flag, status: %d, body: %s", httpResp.StatusCode, string(body)))
		return
	}

	// Parse response with correct structure
	var response struct {
		Resource struct {
			NamespaceKey string `json:"namespaceKey"`
			Key          string `json:"key"`
			Payload      struct {
				Type        string                 `json:"type"`
				Key         string                 `json:"key"`
				Name        string                 `json:"name"`
				Description string                 `json:"description"`
				Enabled     bool                   `json:"enabled"`
				Metadata    map[string]interface{} `json:"metadata"`
			} `json:"payload"`
		} `json:"resource"`
		Revision string `json:"revision"`
	}

	if err := json.Unmarshal(body, &response); err != nil {
		resp.Diagnostics.AddError("Parse Error", fmt.Sprintf("Unable to parse response: %s", err))
		return
	}

	flag := response.Resource.Payload

	data.Key = types.StringValue(flag.Key)
	data.Name = types.StringValue(flag.Name)
	data.Enabled = types.BoolValue(flag.Enabled)
	data.Type = types.StringValue(flag.Type)

	if flag.Description != "" {
		data.Description = types.StringValue(flag.Description)
	} else {
		data.Description = types.StringNull()
	}

	// Set metadata if present
	if len(flag.Metadata) > 0 {
		metadataMap := make(map[string]string)
		for k, v := range flag.Metadata {
			metadataMap[k] = fmt.Sprintf("%v", v)
		}
		metadataValue, diags := types.MapValueFrom(ctx, types.StringType, metadataMap)
		if diags.HasError() {
			resp.Diagnostics.Append(diags...)
			return
		}
		data.Metadata = metadataValue
	} else {
		data.Metadata = types.MapNull(types.StringType)
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
