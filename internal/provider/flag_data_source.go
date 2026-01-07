// Copyright (c) terraform-flipt contributors
// SPDX-License-Identifier: MIT

package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	flipt "github.com/lerentis/flipt-server-rest-sdk-go/generated"
)

var _ datasource.DataSource = &FlagDataSource{}

func NewFlagDataSource() datasource.DataSource {
	return &FlagDataSource{}
}

type FlagDataSource struct {
	client *flipt.APIClient
}

type FlagDataSourceModel struct {
	NamespaceKey types.String `tfsdk:"namespace_key"`
	Key          types.String `tfsdk:"key"`
	Name         types.String `tfsdk:"name"`
	Description  types.String `tfsdk:"description"`
	Enabled      types.Bool   `tfsdk:"enabled"`
	Type         types.String `tfsdk:"type"`
	CreatedAt    types.String `tfsdk:"created_at"`
	UpdatedAt    types.String `tfsdk:"updated_at"`
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
			"created_at": schema.StringAttribute{
				MarkdownDescription: "Timestamp when the flag was created",
				Computed:            true,
			},
			"updated_at": schema.StringAttribute{
				MarkdownDescription: "Timestamp when the flag was last updated",
				Computed:            true,
			},
		},
	}
}

func (d *FlagDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(*flipt.APIClient)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Data Source Configure Type",
			fmt.Sprintf("Expected *flipt.APIClient, got: %T", req.ProviderData),
		)
		return
	}

	d.client = client
}

func (d *FlagDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data FlagDataSourceModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	flag, httpResp, err := d.client.FlagsServiceAPI.GetFlag(ctx, data.NamespaceKey.ValueString(), data.Key.ValueString()).Execute()
	if err != nil {
		if httpResp != nil && httpResp.StatusCode == 404 {
			resp.Diagnostics.AddError("Not Found", fmt.Sprintf("Flag with key '%s' not found in namespace '%s'", data.Key.ValueString(), data.NamespaceKey.ValueString()))
			return
		}
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to read flag, got error: %s", err))
		return
	}

	data.Key = types.StringValue(flag.GetKey())
	data.Name = types.StringValue(flag.GetName())
	data.Enabled = types.BoolValue(flag.GetEnabled())

	if desc, ok := flag.GetDescriptionOk(); ok {
		data.Description = types.StringValue(*desc)
	} else {
		data.Description = types.StringNull()
	}

	if flagType, ok := flag.GetTypeOk(); ok {
		data.Type = types.StringValue(*flagType)
	}

	if createdAt, ok := flag.GetCreatedAtOk(); ok {
		data.CreatedAt = types.StringValue(createdAt.String())
	}

	if updatedAt, ok := flag.GetUpdatedAtOk(); ok {
		data.UpdatedAt = types.StringValue(updatedAt.String())
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
