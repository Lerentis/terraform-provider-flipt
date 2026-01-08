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
	flipt "go.flipt.io/flipt/rpc/flipt"
	sdk "go.flipt.io/flipt/sdk/go"
)

var _ resource.Resource = &ConstraintResource{}
var _ resource.ResourceWithImportState = &ConstraintResource{}

func NewConstraintResource() resource.Resource {
	return &ConstraintResource{}
}

type ConstraintResource struct {
	client *sdk.SDK
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

func (r *ConstraintResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data ConstraintResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	createReq := &flipt.CreateConstraintRequest{
		NamespaceKey: data.NamespaceKey.ValueString(),
		SegmentKey:   data.SegmentKey.ValueString(),
		Property:     data.Property.ValueString(),
		Operator:     data.Operator.ValueString(),
	}

	// Handle Type field - it's a ComparisonType enum
	if !data.Type.IsNull() {
		typeStr := data.Type.ValueString()
		if typeVal, ok := flipt.ComparisonType_value[typeStr]; ok {
			createReq.Type = flipt.ComparisonType(typeVal)
		}
	}

	if !data.Value.IsNull() {
		createReq.Value = data.Value.ValueString()
	}

	if !data.Description.IsNull() {
		createReq.Description = data.Description.ValueString()
	}

	constraint, err := r.client.Flipt().CreateConstraint(ctx, createReq)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to create constraint, got error: %s", err))
		return
	}

	data.ID = types.StringValue(constraint.Id)
	if constraint.Type != 0 {
		data.Type = types.StringValue(constraint.Type.String())
	}
	data.Property = types.StringValue(constraint.Property)
	data.Operator = types.StringValue(constraint.Operator)

	if constraint.Value != "" {
		data.Value = types.StringValue(constraint.Value)
	}

	if constraint.Description != "" {
		data.Description = types.StringValue(constraint.Description)
	}

	if constraint.CreatedAt != nil {
		data.CreatedAt = types.StringValue(constraint.CreatedAt.AsTime().String())
	}

	if constraint.UpdatedAt != nil {
		data.UpdatedAt = types.StringValue(constraint.UpdatedAt.AsTime().String())
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

	segment, err := r.client.Flipt().GetSegment(ctx, &flipt.GetSegmentRequest{
		NamespaceKey: data.NamespaceKey.ValueString(),
		Key:          data.SegmentKey.ValueString(),
	})
	if err != nil {
		resp.State.RemoveResource(ctx)
		return
	}

	// Find the constraint in the segment's constraints list
	var foundConstraint *flipt.Constraint
	for _, c := range segment.Constraints {
		if c.Id == data.ID.ValueString() {
			foundConstraint = c
			break
		}
	}

	if foundConstraint == nil {
		resp.State.RemoveResource(ctx)
		return
	}

	if foundConstraint.Type != 0 {
		data.Type = types.StringValue(foundConstraint.Type.String())
	}
	data.Property = types.StringValue(foundConstraint.Property)
	data.Operator = types.StringValue(foundConstraint.Operator)

	if foundConstraint.Value != "" {
		data.Value = types.StringValue(foundConstraint.Value)
	} else {
		data.Value = types.StringNull()
	}

	if foundConstraint.Description != "" {
		data.Description = types.StringValue(foundConstraint.Description)
	} else {
		data.Description = types.StringNull()
	}

	if foundConstraint.CreatedAt != nil {
		data.CreatedAt = types.StringValue(foundConstraint.CreatedAt.AsTime().String())
	}

	if foundConstraint.UpdatedAt != nil {
		data.UpdatedAt = types.StringValue(foundConstraint.UpdatedAt.AsTime().String())
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *ConstraintResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data ConstraintResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	updateReq := &flipt.UpdateConstraintRequest{
		NamespaceKey: data.NamespaceKey.ValueString(),
		SegmentKey:   data.SegmentKey.ValueString(),
		Id:           data.ID.ValueString(),
		Property:     data.Property.ValueString(),
		Operator:     data.Operator.ValueString(),
	}

	// Handle Type field - it's a ComparisonType enum
	if !data.Type.IsNull() {
		typeStr := data.Type.ValueString()
		if typeVal, ok := flipt.ComparisonType_value[typeStr]; ok {
			updateReq.Type = flipt.ComparisonType(typeVal)
		}
	}

	if !data.Value.IsNull() {
		updateReq.Value = data.Value.ValueString()
	}

	if !data.Description.IsNull() {
		updateReq.Description = data.Description.ValueString()
	}

	constraint, err := r.client.Flipt().UpdateConstraint(ctx, updateReq)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to update constraint, got error: %s", err))
		return
	}

	if constraint.Type != 0 {
		data.Type = types.StringValue(constraint.Type.String())
	}
	data.Property = types.StringValue(constraint.Property)
	data.Operator = types.StringValue(constraint.Operator)

	if constraint.Value != "" {
		data.Value = types.StringValue(constraint.Value)
	}

	if constraint.Description != "" {
		data.Description = types.StringValue(constraint.Description)
	}

	if constraint.UpdatedAt != nil {
		data.UpdatedAt = types.StringValue(constraint.UpdatedAt.AsTime().String())
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *ConstraintResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data ConstraintResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	err := r.client.Flipt().DeleteConstraint(ctx, &flipt.DeleteConstraintRequest{
		NamespaceKey: data.NamespaceKey.ValueString(),
		SegmentKey:   data.SegmentKey.ValueString(),
		Id:           data.ID.ValueString(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to delete constraint, got error: %s", err))
		return
	}

	tflog.Trace(ctx, "deleted a constraint resource")
}

func (r *ConstraintResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
