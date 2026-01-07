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

var _ resource.Resource = &RuleResource{}
var _ resource.ResourceWithImportState = &RuleResource{}

func NewRuleResource() resource.Resource {
	return &RuleResource{}
}

type RuleResource struct {
	client *flipt.APIClient
}

type RuleResourceModel struct {
	NamespaceKey types.String `tfsdk:"namespace_key"`
	FlagKey      types.String `tfsdk:"flag_key"`
	ID           types.String `tfsdk:"id"`
	SegmentKey   types.String `tfsdk:"segment_key"`
	Rank         types.Int64  `tfsdk:"rank"`
	CreatedAt    types.String `tfsdk:"created_at"`
	UpdatedAt    types.String `tfsdk:"updated_at"`
}

func (r *RuleResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_rule"
}

func (r *RuleResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Flipt rule resource (belongs to a flag)",

		Attributes: map[string]schema.Attribute{
			"namespace_key": schema.StringAttribute{
				MarkdownDescription: "Namespace key where the flag belongs",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"flag_key": schema.StringAttribute{
				MarkdownDescription: "Flag key that this rule belongs to",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"id": schema.StringAttribute{
				MarkdownDescription: "Unique identifier for the rule",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"segment_key": schema.StringAttribute{
				MarkdownDescription: "Segment key to evaluate for this rule",
				Required:            true,
			},
			"rank": schema.Int64Attribute{
				MarkdownDescription: "Rank/order of the rule",
				Optional:            true,
				Computed:            true,
			},
			"created_at": schema.StringAttribute{
				MarkdownDescription: "Timestamp when the rule was created",
				Computed:            true,
			},
			"updated_at": schema.StringAttribute{
				MarkdownDescription: "Timestamp when the rule was last updated",
				Computed:            true,
			},
		},
	}
}

func (r *RuleResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *RuleResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data RuleResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	rank := int32(1)
	if !data.Rank.IsNull() {
		rank = int32(data.Rank.ValueInt64())
	}

	createReq := *flipt.NewCreateRuleRequest(rank)
	segmentKey := data.SegmentKey.ValueString()
	createReq.SegmentKey = &segmentKey

	rule, httpResp, err := r.client.RulesServiceAPI.CreateRule(ctx, data.NamespaceKey.ValueString(), data.FlagKey.ValueString()).CreateRuleRequest(createReq).Execute()
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to create rule, got error: %s", err))
		return
	}

	if httpResp.StatusCode != 200 && httpResp.StatusCode != 201 {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to create rule, got status: %d", httpResp.StatusCode))
		return
	}

	data.ID = types.StringValue(rule.GetId())
	data.SegmentKey = types.StringValue(rule.GetSegmentKey())

	if rank, ok := rule.GetRankOk(); ok {
		data.Rank = types.Int64Value(int64(*rank))
	}

	if createdAt, ok := rule.GetCreatedAtOk(); ok {
		data.CreatedAt = types.StringValue(createdAt.String())
	}

	if updatedAt, ok := rule.GetUpdatedAtOk(); ok {
		data.UpdatedAt = types.StringValue(updatedAt.String())
	}

	tflog.Trace(ctx, "created a rule resource")
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *RuleResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data RuleResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	rule, httpResp, err := r.client.RulesServiceAPI.GetRule(ctx, data.NamespaceKey.ValueString(), data.FlagKey.ValueString(), data.ID.ValueString()).Execute()
	if err != nil {
		if httpResp != nil && httpResp.StatusCode == 404 {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to read rule, got error: %s", err))
		return
	}

	data.SegmentKey = types.StringValue(rule.GetSegmentKey())

	if rank, ok := rule.GetRankOk(); ok {
		data.Rank = types.Int64Value(int64(*rank))
	}

	if createdAt, ok := rule.GetCreatedAtOk(); ok {
		data.CreatedAt = types.StringValue(createdAt.String())
	}

	if updatedAt, ok := rule.GetUpdatedAtOk(); ok {
		data.UpdatedAt = types.StringValue(updatedAt.String())
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *RuleResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data RuleResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	updateReq := *flipt.NewUpdateRuleRequest()
	segmentKey := data.SegmentKey.ValueString()
	updateReq.SegmentKey = &segmentKey

	rule, httpResp, err := r.client.RulesServiceAPI.UpdateRule(ctx, data.NamespaceKey.ValueString(), data.FlagKey.ValueString(), data.ID.ValueString()).UpdateRuleRequest(updateReq).Execute()
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to update rule, got error: %s", err))
		return
	}

	if httpResp.StatusCode != 200 {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to update rule, got status: %d", httpResp.StatusCode))
		return
	}

	data.SegmentKey = types.StringValue(rule.GetSegmentKey())

	if rank, ok := rule.GetRankOk(); ok {
		data.Rank = types.Int64Value(int64(*rank))
	}

	if updatedAt, ok := rule.GetUpdatedAtOk(); ok {
		data.UpdatedAt = types.StringValue(updatedAt.String())
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *RuleResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data RuleResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	httpResp, err := r.client.RulesServiceAPI.DeleteRule(ctx, data.NamespaceKey.ValueString(), data.FlagKey.ValueString(), data.ID.ValueString()).Execute()
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to delete rule, got error: %s", err))
		return
	}

	if httpResp.StatusCode != 204 && httpResp.StatusCode != 200 {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to delete rule, got status: %d", httpResp.StatusCode))
		return
	}

	tflog.Trace(ctx, "deleted a rule resource")
}

func (r *RuleResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
