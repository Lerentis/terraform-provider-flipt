// Copyright (c) terraform-flipt contributors
// SPDX-License-Identifier: MIT

package provider

import (
"context"
"fmt"

"github.com/hashicorp/terraform-plugin-framework/path"
"github.com/hashicorp/terraform-plugin-framework/resource"
"github.com/hashicorp/terraform-plugin-framework/resource/schema"
"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
"github.com/hashicorp/terraform-plugin-framework/types"
"github.com/hashicorp/terraform-plugin-log/tflog"
sdk "go.flipt.io/flipt/sdk/go"
flipt "go.flipt.io/flipt/rpc/flipt"
)

var _ resource.Resource = &SegmentResource{}
var _ resource.ResourceWithImportState = &SegmentResource{}

func NewSegmentResource() resource.Resource {
	return &SegmentResource{}
}

type SegmentResource struct {
	client *sdk.SDK
}

type SegmentResourceModel struct {
	NamespaceKey types.String `tfsdk:"namespace_key"`
	Key          types.String `tfsdk:"key"`
	Name         types.String `tfsdk:"name"`
	Description  types.String `tfsdk:"description"`
	MatchType    types.String `tfsdk:"match_type"`
	CreatedAt    types.String `tfsdk:"created_at"`
	UpdatedAt    types.String `tfsdk:"updated_at"`
}

func (r *SegmentResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_segment"
}

func (r *SegmentResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Flipt segment resource",

		Attributes: map[string]schema.Attribute{
			"namespace_key": schema.StringAttribute{
				MarkdownDescription: "Namespace key where the segment belongs",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"key": schema.StringAttribute{
				MarkdownDescription: "Unique key for the segment",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "Display name of the segment",
				Required:            true,
			},
			"description": schema.StringAttribute{
				MarkdownDescription: "Description of the segment",
				Optional:            true,
			},
			"match_type": schema.StringAttribute{
				MarkdownDescription: "Match type for the segment (ALL_MATCH_TYPE or ANY_MATCH_TYPE)",
				Optional:            true,
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

func (r *SegmentResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(*sdk.SDK)
	if !ok {
		resp.Diagnostics.AddError(
"Unexpected Resource Configure Type",
fmt.Sprintf("Expected *sdk.SDK, got: %T", req.ProviderData),
)
		return
	}

	r.client = client
}

func (r *SegmentResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data SegmentResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	matchType := flipt.MatchType_ALL_MATCH_TYPE
	if !data.MatchType.IsNull() && !data.MatchType.IsUnknown() {
		if data.MatchType.ValueString() == "ANY_MATCH_TYPE" {
			matchType = flipt.MatchType_ANY_MATCH_TYPE
		}
	}

	createReq := &flipt.CreateSegmentRequest{
		NamespaceKey: data.NamespaceKey.ValueString(),
		Key:          data.Key.ValueString(),
		Name:         data.Name.ValueString(),
		MatchType:    matchType,
	}

	if !data.Description.IsNull() {
		createReq.Description = data.Description.ValueString()
	}

	segment, err := r.client.Flipt().CreateSegment(ctx, createReq)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to create segment, got error: %s", err))
		return
	}

	data.Key = types.StringValue(segment.Key)
	data.Name = types.StringValue(segment.Name)

	if segment.Description != "" {
		data.Description = types.StringValue(segment.Description)
	}

	data.MatchType = types.StringValue(segment.MatchType.String())

	if segment.CreatedAt != nil {
		data.CreatedAt = types.StringValue(segment.CreatedAt.AsTime().String())
	}

	if segment.UpdatedAt != nil {
		data.UpdatedAt = types.StringValue(segment.UpdatedAt.AsTime().String())
	}

	tflog.Trace(ctx, "created a segment resource")
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *SegmentResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data SegmentResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	segment, err := r.client.Flipt().GetSegment(ctx, &flipt.GetSegmentRequest{
		NamespaceKey: data.NamespaceKey.ValueString(),
		Key:          data.Key.ValueString(),
	})
	if err != nil {
		resp.State.RemoveResource(ctx)
		return
	}

	data.Key = types.StringValue(segment.Key)
	data.Name = types.StringValue(segment.Name)

	if segment.Description != "" {
		data.Description = types.StringValue(segment.Description)
	} else {
		data.Description = types.StringNull()
	}

	data.MatchType = types.StringValue(segment.MatchType.String())

	if segment.CreatedAt != nil {
		data.CreatedAt = types.StringValue(segment.CreatedAt.AsTime().String())
	}

	if segment.UpdatedAt != nil {
		data.UpdatedAt = types.StringValue(segment.UpdatedAt.AsTime().String())
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *SegmentResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data SegmentResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	matchType := flipt.MatchType_ALL_MATCH_TYPE
	if !data.MatchType.IsNull() && !data.MatchType.IsUnknown() {
		if data.MatchType.ValueString() == "ANY_MATCH_TYPE" {
			matchType = flipt.MatchType_ANY_MATCH_TYPE
		}
	}

	updateReq := &flipt.UpdateSegmentRequest{
		NamespaceKey: data.NamespaceKey.ValueString(),
		Key:          data.Key.ValueString(),
		Name:         data.Name.ValueString(),
		MatchType:    matchType,
	}

	if !data.Description.IsNull() {
		updateReq.Description = data.Description.ValueString()
	}

	segment, err := r.client.Flipt().UpdateSegment(ctx, updateReq)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to update segment, got error: %s", err))
		return
	}

	data.Name = types.StringValue(segment.Name)

	if segment.Description != "" {
		data.Description = types.StringValue(segment.Description)
	}

	data.MatchType = types.StringValue(segment.MatchType.String())

	if segment.UpdatedAt != nil {
		data.UpdatedAt = types.StringValue(segment.UpdatedAt.AsTime().String())
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *SegmentResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data SegmentResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	err := r.client.Flipt().DeleteSegment(ctx, &flipt.DeleteSegmentRequest{
		NamespaceKey: data.NamespaceKey.ValueString(),
		Key:          data.Key.ValueString(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to delete segment, got error: %s", err))
		return
	}

	tflog.Trace(ctx, "deleted a segment resource")
}

func (r *SegmentResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("key"), req, resp)
}
