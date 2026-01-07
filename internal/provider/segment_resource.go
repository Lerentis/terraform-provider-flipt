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
	flipt "github.com/lerentis/flipt-server-rest-sdk-go/generated"
)

var _ resource.Resource = &SegmentResource{}
var _ resource.ResourceWithImportState = &SegmentResource{}

func NewSegmentResource() resource.Resource {
	return &SegmentResource{}
}

type SegmentResource struct {
	client *flipt.APIClient
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

	client, ok := req.ProviderData.(*flipt.APIClient)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *flipt.APIClient, got: %T", req.ProviderData),
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

	matchType := "ALL_MATCH_TYPE"
	if !data.MatchType.IsNull() {
		matchType = data.MatchType.ValueString()
	}

	createReq := *flipt.NewCreateSegmentRequest(data.Key.ValueString(), data.Name.ValueString(), matchType)

	if !data.Description.IsNull() {
		desc := data.Description.ValueString()
		createReq.Description = &desc
	}

	segment, httpResp, err := r.client.SegmentsServiceAPI.CreateSegment(ctx, data.NamespaceKey.ValueString()).CreateSegmentRequest(createReq).Execute()
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to create segment, got error: %s", err))
		return
	}

	if httpResp.StatusCode != 200 && httpResp.StatusCode != 201 {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to create segment, got status: %d", httpResp.StatusCode))
		return
	}

	data.Key = types.StringValue(segment.GetKey())
	data.Name = types.StringValue(segment.GetName())

	if desc, ok := segment.GetDescriptionOk(); ok {
		data.Description = types.StringValue(*desc)
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

	tflog.Trace(ctx, "created a segment resource")
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *SegmentResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data SegmentResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	segment, httpResp, err := r.client.SegmentsServiceAPI.GetSegment(ctx, data.NamespaceKey.ValueString(), data.Key.ValueString()).Execute()
	if err != nil {
		if httpResp != nil && httpResp.StatusCode == 404 {
			resp.State.RemoveResource(ctx)
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

func (r *SegmentResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data SegmentResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	matchType := "ALL_MATCH_TYPE"
	if !data.MatchType.IsNull() {
		matchType = data.MatchType.ValueString()
	}

	updateReq := *flipt.NewUpdateSegmentRequest(data.Name.ValueString(), matchType)

	if !data.Description.IsNull() {
		desc := data.Description.ValueString()
		updateReq.Description = &desc
	}

	segment, httpResp, err := r.client.SegmentsServiceAPI.UpdateSegment(ctx, data.NamespaceKey.ValueString(), data.Key.ValueString()).UpdateSegmentRequest(updateReq).Execute()
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to update segment, got error: %s", err))
		return
	}

	if httpResp.StatusCode != 200 {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to update segment, got status: %d", httpResp.StatusCode))
		return
	}

	data.Name = types.StringValue(segment.GetName())

	if desc, ok := segment.GetDescriptionOk(); ok {
		data.Description = types.StringValue(*desc)
	}

	if matchType, ok := segment.GetMatchTypeOk(); ok {
		data.MatchType = types.StringValue(*matchType)
	}

	if updatedAt, ok := segment.GetUpdatedAtOk(); ok {
		data.UpdatedAt = types.StringValue(updatedAt.String())
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *SegmentResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data SegmentResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	httpResp, err := r.client.SegmentsServiceAPI.DeleteSegment(ctx, data.NamespaceKey.ValueString(), data.Key.ValueString()).Execute()
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to delete segment, got error: %s", err))
		return
	}

	if httpResp.StatusCode != 204 && httpResp.StatusCode != 200 {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to delete segment, got status: %d", httpResp.StatusCode))
		return
	}

	tflog.Trace(ctx, "deleted a segment resource")
}

func (r *SegmentResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("key"), req, resp)
}
