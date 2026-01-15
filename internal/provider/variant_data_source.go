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

var _ datasource.DataSource = &VariantDataSource{}

func NewVariantDataSource() datasource.DataSource {
	return &VariantDataSource{}
}

type VariantDataSource struct {
	httpClient *http.Client
	endpoint   string
}

type VariantDataSourceModel struct {
	NamespaceKey   types.String `tfsdk:"namespace_key"`
	EnvironmentKey types.String `tfsdk:"environment_key"`
	FlagKey        types.String `tfsdk:"flag_key"`
	Key            types.String `tfsdk:"key"`
	Name           types.String `tfsdk:"name"`
	Description    types.String `tfsdk:"description"`
	Attachment     types.String `tfsdk:"attachment"`
}

func (d *VariantDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_variant"
}

func (d *VariantDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Flipt variant data source",
		Description:         "Flipt variant data source",

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
			"flag_key": schema.StringAttribute{
				MarkdownDescription: "Flag key",
				Description:         "Flag key",
				Required:            true,
			},
			"key": schema.StringAttribute{
				MarkdownDescription: "Variant key",
				Description:         "Variant key",
				Required:            true,
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "Variant name",
				Description:         "Variant name",
				Computed:            true,
			},
			"description": schema.StringAttribute{
				MarkdownDescription: "Variant description",
				Description:         "Variant description",
				Computed:            true,
			},
			"attachment": schema.StringAttribute{
				MarkdownDescription: "Variant attachment (JSON string)",
				Description:         "Variant attachment (JSON string)",
				Computed:            true,
			},
		},
	}
}

func (d *VariantDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *VariantDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data VariantDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Determine environment key (default to "default" if not specified)
	envKey := "default"
	if !data.EnvironmentKey.IsNull() && !data.EnvironmentKey.IsUnknown() {
		envKey = data.EnvironmentKey.ValueString()
	}

	tflog.Debug(ctx, "Reading variant data source", map[string]interface{}{
		"environment_key": envKey,
		"namespace_key":   data.NamespaceKey.ValueString(),
		"flag_key":        data.FlagKey.ValueString(),
		"variant_key":     data.Key.ValueString(),
	})

	// Get the flag to read its variants
	url := fmt.Sprintf("%s/api/v2/environments/%s/namespaces/%s/resources/flipt.core.Flag/%s",
		d.endpoint, envKey, data.NamespaceKey.ValueString(), data.FlagKey.ValueString())

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

	body, err := io.ReadAll(httpResp.Body)
	if err != nil {
		resp.Diagnostics.AddError("Read Error", fmt.Sprintf("Unable to read response: %s", err))
		return
	}

	if httpResp.StatusCode != http.StatusOK {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to read flag, status: %d, body: %s", httpResp.StatusCode, string(body)))
		return
	}

	var flagResponse struct {
		Resource struct {
			Payload struct {
				Variants []struct {
					Key         string                 `json:"key"`
					Name        string                 `json:"name"`
					Description string                 `json:"description"`
					Attachment  map[string]interface{} `json:"attachment"`
				} `json:"variants"`
			} `json:"payload"`
		} `json:"resource"`
	}

	if err := json.Unmarshal(body, &flagResponse); err != nil {
		resp.Diagnostics.AddError("Parse Error", fmt.Sprintf("Unable to parse response: %s", err))
		return
	}

	// Find the variant by key
	var found bool
	for _, v := range flagResponse.Resource.Payload.Variants {
		if v.Key == data.Key.ValueString() {
			found = true

			if v.Name != "" {
				data.Name = types.StringValue(v.Name)
			} else {
				data.Name = types.StringNull()
			}

			if v.Description != "" {
				data.Description = types.StringValue(v.Description)
			} else {
				data.Description = types.StringNull()
			}

			if len(v.Attachment) > 0 {
				attachmentJSON, err := json.Marshal(v.Attachment)
				if err == nil {
					data.Attachment = types.StringValue(string(attachmentJSON))
				} else {
					data.Attachment = types.StringNull()
				}
			} else {
				data.Attachment = types.StringNull()
			}
			break
		}
	}

	if !found {
		resp.Diagnostics.AddError("Variant Not Found", fmt.Sprintf("Variant with key '%s' not found in flag", data.Key.ValueString()))
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
