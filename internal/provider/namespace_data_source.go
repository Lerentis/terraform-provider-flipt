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

var _ datasource.DataSource = &NamespaceDataSource{}

func NewNamespaceDataSource() datasource.DataSource {
	return &NamespaceDataSource{}
}

type NamespaceDataSource struct {
	client *flipt.APIClient
}

type NamespaceDataSourceModel struct {
	Key         types.String `tfsdk:"key"`
	Name        types.String `tfsdk:"name"`
	Description types.String `tfsdk:"description"`
	Protected   types.Bool   `tfsdk:"protected"`
	CreatedAt   types.String `tfsdk:"created_at"`
	UpdatedAt   types.String `tfsdk:"updated_at"`
}

func (d *NamespaceDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_namespace"
}

func (d *NamespaceDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Flipt namespace data source",
		Description:         "Flipt namespace data source",

		Attributes: map[string]schema.Attribute{
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
			"created_at": schema.StringAttribute{
				MarkdownDescription: "Timestamp when the namespace was created",
				Computed:            true,
			},
			"updated_at": schema.StringAttribute{
				MarkdownDescription: "Timestamp when the namespace was last updated",
				Computed:            true,
			},
		},
	}
}

func (d *NamespaceDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *NamespaceDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data NamespaceDataSourceModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	namespace, httpResp, err := d.client.NamespacesServiceAPI.GetNamespace(ctx, data.Key.ValueString()).Execute()
	if err != nil {
		if httpResp != nil && httpResp.StatusCode == 404 {
			resp.Diagnostics.AddError("Not Found", fmt.Sprintf("Namespace with key '%s' not found", data.Key.ValueString()))
			return
		}
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to read namespace, got error: %s", err))
		return
	}

	data.Key = types.StringValue(namespace.GetKey())
	data.Name = types.StringValue(namespace.GetName())

	if desc, ok := namespace.GetDescriptionOk(); ok {
		data.Description = types.StringValue(*desc)
	} else {
		data.Description = types.StringNull()
	}

	if protected, ok := namespace.GetProtectedOk(); ok {
		data.Protected = types.BoolValue(*protected)
	}

	if createdAt, ok := namespace.GetCreatedAtOk(); ok {
		data.CreatedAt = types.StringValue(createdAt.String())
	}

	if updatedAt, ok := namespace.GetUpdatedAtOk(); ok {
		data.UpdatedAt = types.StringValue(updatedAt.String())
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
