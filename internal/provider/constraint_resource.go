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

var _ resource.Resource = &ConstraintResource{}
var _ resource.ResourceWithImportState = &ConstraintResource{}

func NewConstraintResource() resource.Resource {
	return &ConstraintResource{}
}

type ConstraintResource struct {
	client *flipt.APIClient
}

type ConstraintResourceModel struct {
	NamespaceKey types.String `tfsdk:"namespace_key"`
	SegmentKey   types.String `tfsdk:"segment_key"`
	ID           types.String `tfsdk:"id"`
	Type         types.String `tfsdk:"type"`
	Property     types.String `tfsdk:"property"`
	Operator     types.String `tfsdk:"operator"`
	Value        types.String `tfsdk:"value"`
	Description  types.String `tfsdk:"description"`
	CreatedAt    types.String `tfsdk:"created_at"`
	UpdatedAt    types.String `tfsdk:"updated_at"`
}

func (r *ConstraintResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_constraint"
}

func (r *ConstraintResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Flipt constraint resource (belongs to a segment)",

		Attributes: map[string]schema.Attribute{
			"namespace_key": schema.StringAttribute{
				MarkdownDescription: "Namespace key where the segment belongs",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"segment_key": schema.StringAttribute{
				MarkdownDescription: "Segment key that this constraint belongs to",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"id": schema.StringAttribute{
				MarkdownDescription: "Unique identifier for the constraint",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"type": schema.StringAttribute{
				MarkdownDescription: "Type of constraint (STRING_COMPARISON_TYPE, NUMBER_COMPARISON_TYPE, BOOLEAN_COMPARISON_TYPE, etc.)",
				Required:            true,
			},
			"property": schema.StringAttribute{
				MarkdownDescription: "Property to evaluate",
				Required:            true,
			},
			"operator": schema.StringAttribute{
				MarkdownDescription: "Comparison operator",
				Required:            true,
			},
			"value": schema.StringAttribute{
				MarkdownDescription: "Value to compare against",
				Optional:            true,
			},
			"description": schema.StringAttribute{
				MarkdownDescription: "Description of the constraint",
				Optional:            true,
			},
			"created_at": schema.StringAttribute{
				MarkdownDescription: "Timestamp when the constraint was created",
				Computed:            true,
			},
			"updated_at": schema.StringAttribute{
				MarkdownDescription: "Timestamp when the constraint was last updated",
				Computed:            true,
			},
		},
	}
}

func (r *ConstraintResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *ConstraintResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data ConstraintResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	createReq := *flipt.NewCreateConstraintRequest(data.Type.ValueString(), data.Property.ValueString(), data.Operator.ValueString())

	if !data.Value.IsNull() {
		value := data.Value.ValueString()
		createReq.Value = &value
	}

	if !data.Description.IsNull() {
		desc := data.Description.ValueString()
		createReq.Description = &desc
	}

	constraint, httpResp, err := r.client.ConstraintsServiceAPI.CreateConstraint(ctx, data.NamespaceKey.ValueString(), data.SegmentKey.ValueString()).CreateConstraintRequest(createReq).Execute()
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to create constraint, got error: %s", err))
		return
	}

	if httpResp.StatusCode != 200 && httpResp.StatusCode != 201 {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to create constraint, got status: %d", httpResp.StatusCode))
		return
	}

	data.ID = types.StringValue(constraint.GetId())
	data.Type = types.StringValue(constraint.GetType())
	data.Property = types.StringValue(constraint.GetProperty())
	data.Operator = types.StringValue(constraint.GetOperator())

	if value, ok := constraint.GetValueOk(); ok {
		data.Value = types.StringValue(*value)
	}

	if desc, ok := constraint.GetDescriptionOk(); ok {
		data.Description = types.StringValue(*desc)
	}

	if createdAt, ok := constraint.GetCreatedAtOk(); ok {
		data.CreatedAt = types.StringValue(createdAt.String())
	}

	if updatedAt, ok := constraint.GetUpdatedAtOk(); ok {
		data.UpdatedAt = types.StringValue(updatedAt.String())
	}

	tflog.Trace(ctx, "created a constraint resource")
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *ConstraintResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data ConstraintResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Note: Constraints don't have a direct Get endpoint
	// A production implementation might want to list all constraints and find the matching one
	tflog.Trace(ctx, "read a constraint resource")
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *ConstraintResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data ConstraintResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	updateReq := *flipt.NewUpdateConstraintRequest(data.Type.ValueString(), data.Property.ValueString(), data.Operator.ValueString())

	if !data.Value.IsNull() {
		value := data.Value.ValueString()
		updateReq.Value = &value
	}

	if !data.Description.IsNull() {
		desc := data.Description.ValueString()
		updateReq.Description = &desc
	}

	constraint, httpResp, err := r.client.ConstraintsServiceAPI.UpdateConstraint(ctx, data.NamespaceKey.ValueString(), data.SegmentKey.ValueString(), data.ID.ValueString()).UpdateConstraintRequest(updateReq).Execute()
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to update constraint, got error: %s", err))
		return
	}

	if httpResp.StatusCode != 200 {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to update constraint, got status: %d", httpResp.StatusCode))
		return
	}

	data.Type = types.StringValue(constraint.GetType())
	data.Property = types.StringValue(constraint.GetProperty())
	data.Operator = types.StringValue(constraint.GetOperator())

	if value, ok := constraint.GetValueOk(); ok {
		data.Value = types.StringValue(*value)
	}

	if desc, ok := constraint.GetDescriptionOk(); ok {
		data.Description = types.StringValue(*desc)
	}

	if updatedAt, ok := constraint.GetUpdatedAtOk(); ok {
		data.UpdatedAt = types.StringValue(updatedAt.String())
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *ConstraintResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data ConstraintResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	httpResp, err := r.client.ConstraintsServiceAPI.DeleteConstraint(ctx, data.NamespaceKey.ValueString(), data.SegmentKey.ValueString(), data.ID.ValueString()).Execute()
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to delete constraint, got error: %s", err))
		return
	}

	if httpResp.StatusCode != 204 && httpResp.StatusCode != 200 {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to delete constraint, got status: %d", httpResp.StatusCode))
		return
	}

	tflog.Trace(ctx, "deleted a constraint resource")
}

func (r *ConstraintResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
