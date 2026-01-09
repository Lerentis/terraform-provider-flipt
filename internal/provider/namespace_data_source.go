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

var _ datasource.DataSource = &NamespaceDataSource{}

func NewNamespaceDataSource() datasource.DataSource {
	return &NamespaceDataSource{}
}

type NamespaceDataSource struct {
	httpClient *http.Client
	endpoint   string
}

type NamespaceDataSourceModel struct {
	EnvironmentKey types.String `tfsdk:"environment_key"`
	Key            types.String `tfsdk:"key"`
	Name           types.String `tfsdk:"name"`
	Description    types.String `tfsdk:"description"`
	Protected      types.Bool   `tfsdk:"protected"`
}

func (d *NamespaceDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_namespace"
}

func (d *NamespaceDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Flipt namespace data source",
		Description:         "Flipt namespace data source",

		Attributes: map[string]schema.Attribute{
			"environment_key": schema.StringAttribute{
				MarkdownDescription: "Environment key (defaults to 'default')",
				Optional:            true,
			},
			"key": schema.StringAttribute{
				MarkdownDescription: "Unique key for the namespace",
				Required:            true,
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "Display name of the namespace",
				Computed:            true,
			},
			"description": schema.StringAttribute{
				MarkdownDescription: "Description of the namespace",
				Computed:            true,
			},
			"protected": schema.BoolAttribute{
				MarkdownDescription: "Whether the namespace is protected",
				Computed:            true,
			},
		},
	}
}

func (d *NamespaceDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *NamespaceDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data NamespaceDataSourceModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Default to "default" environment if not specified
	envKey := "default"
	if !data.EnvironmentKey.IsNull() && !data.EnvironmentKey.IsUnknown() {
		envKey = data.EnvironmentKey.ValueString()
	}
	data.EnvironmentKey = types.StringValue(envKey)

	tflog.Debug(ctx, "Reading namespace", map[string]interface{}{
		"environment_key": envKey,
		"key":             data.Key.ValueString(),
	})

	// Get the namespace from Flipt
	url := fmt.Sprintf("%s/namespaces/%s", d.endpoint, data.Key.ValueString())
	httpReq, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		resp.Diagnostics.AddError("Request Error", fmt.Sprintf("Unable to create request: %s", err))
		return
	}

	httpResp, err := d.httpClient.Do(httpReq)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to read namespace, got error: %s", err))
		return
	}
	defer httpResp.Body.Close()

	if httpResp.StatusCode == http.StatusNotFound {
		resp.Diagnostics.AddError("Not Found", fmt.Sprintf("Namespace with key '%s' not found", data.Key.ValueString()))
		return
	}

	body, err := io.ReadAll(httpResp.Body)
	if err != nil {
		resp.Diagnostics.AddError("Read Error", fmt.Sprintf("Unable to read response: %s", err))
		return
	}

	if httpResp.StatusCode != http.StatusOK {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to read namespace, status: %d, body: %s", httpResp.StatusCode, string(body)))
		return
	}

	var response struct {
		Namespace struct {
			Key         string `json:"key"`
			Name        string `json:"name"`
			Description string `json:"description"`
			Protected   bool   `json:"protected"`
		} `json:"namespace"`
		Revision string `json:"revision"`
	}

	if err := json.Unmarshal(body, &response); err != nil {
		resp.Diagnostics.AddError("Parse Error", fmt.Sprintf("Unable to parse response: %s", err))
		return
	}

	namespace := response.Namespace

	data.Key = types.StringValue(namespace.Key)
	data.Name = types.StringValue(namespace.Name)

	if namespace.Description != "" {
		data.Description = types.StringValue(namespace.Description)
	} else {
		data.Description = types.StringNull()
	}

	data.Protected = types.BoolValue(namespace.Protected)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
