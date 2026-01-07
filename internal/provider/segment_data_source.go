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

var _ datasource.DataSource = &SegmentDataSource{}

func NewSegmentDataSource() datasource.DataSource {
	return &SegmentDataSource{}
}

type SegmentDataSource struct {
	client *flipt.APIClient
}

type SegmentDataSourceModel struct {
	NamespaceKey types.String `tfsdk:"namespace_key"`
	Key          types.String `tfsdk:"key"`
	Name         types.String `tfsdk:"name"`
	Description  types.String `tfsdk:"description"`
	MatchType    types.String `tfsdk:"match_type"`
	CreatedAt    types.String `tfsdk:"created_at"`
	UpdatedAt    types.String `tfsdk:"updated_at"`
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
				MarkdownDescription: "Namespace key where the segment belongs",
				Required:            true,
			},
			"key": schema.StringAttribute{
				MarkdownDescription: "Unique key for the segment",
				Required:            true,
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "Display name of the segment",
				Computed:            true,
			},
			"description": schema.StringAttribute{
				MarkdownDescription: "Description of the segment",
				Computed:            true,
			},
			"match_type": schema.StringAttribute{
				MarkdownDescription: "Match type for the segment (ALL_MATCH_TYPE or ANY_MATCH_TYPE)",
				Computed:            true,
			},
			"created_at": schema.StringAttribute{
				MarkdownDescription: "Timestamp when the segment was created",
				Computed:            true,
			},
			"updated_at": schema.StringAttribute{
				MarkdownDescription: "Timestamp when the segment was last updated",
				Computed:            true,
			},
		},
	}
}

func (d *SegmentDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *SegmentDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data SegmentDataSourceModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	segment, httpResp, err := d.client.SegmentsServiceAPI.GetSegment(ctx, data.NamespaceKey.ValueString(), data.Key.ValueString()).Execute()
	if err != nil {
		if httpResp != nil && httpResp.StatusCode == 404 {
			resp.Diagnostics.AddError("Not Found", fmt.Sprintf("Segment with key '%s' not found in namespace '%s'", data.Key.ValueString(), data.NamespaceKey.ValueString()))
			return
		}
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to read segment, got error: %s", err))
		return
	}

	data.Key = types.StringValue(segment.GetKey())
	data.Name = types.StringValue(segment.GetName())

	if desc, ok := segment.GetDescriptionOk(); ok {
		data.Description = types.StringValue(*desc)
	} else {
		data.Description = types.StringNull()
	}

	if matchType, ok := segment.GetMatchTypeOk(); ok {
		data.MatchType = types.StringValue(*matchType)
	}

	if createdAt, ok := segment.GetCreatedAtOk(); ok {
		data.CreatedAt = types.StringValue(createdAt.String())
	}

	if updatedAt, ok := segment.GetUpdatedAtOk(); ok {
		data.UpdatedAt = types.StringValue(updatedAt.String())
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
