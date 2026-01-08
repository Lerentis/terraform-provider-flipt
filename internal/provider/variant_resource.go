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

var _ resource.Resource = &VariantResource{}
var _ resource.ResourceWithImportState = &VariantResource{}

func NewVariantResource() resource.Resource {
	return &VariantResource{}
}

type VariantResource struct {
	client *sdk.SDK
}

type VariantResourceModel struct {
	NamespaceKey types.String `tfsdk:"namespace_key"`
	FlagKey      types.String `tfsdk:"flag_key"`
	ID           types.String `tfsdk:"id"`
	Key          types.String `tfsdk:"key"`
	Name         types.String `tfsdk:"name"`
	Description  types.String `tfsdk:"description"`
	Attachment   types.String `tfsdk:"attachment"`
	CreatedAt    types.String `tfsdk:"created_at"`
	UpdatedAt    types.String `tfsdk:"updated_at"`
}

func (r *VariantResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_variant"
}

func (r *VariantResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Flipt variant resource (belongs to a flag)",

		Attributes: map[string]schema.Attribute{
			"namespace_key": schema.StringAttribute{
				MarkdownDescription: "Namespace key where the flag belongs",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"flag_key": schema.StringAttribute{
				MarkdownDescription: "Flag key that this variant belongs to",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"id": schema.StringAttribute{
				MarkdownDescription: "Unique identifier for the variant",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"key": schema.StringAttribute{
				MarkdownDescription: "Unique key for the variant",
				Required:            true,
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "Display name of the variant",
				Required:            true,
			},
			"description": schema.StringAttribute{
				MarkdownDescription: "Description of the variant",
				Optional:            true,
			},
			"attachment": schema.StringAttribute{
				MarkdownDescription: "JSON attachment data for the variant",
				Optional:            true,
			},
			"created_at": schema.StringAttribute{
				MarkdownDescription: "Timestamp when the variant was created",
				Computed:            true,
			},
			"updated_at": schema.StringAttribute{
				MarkdownDescription: "Timestamp when the variant was last updated",
				Computed:            true,
			},
		},
	}
}

func (r *VariantResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *VariantResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data VariantResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	createReq := &flipt.CreateVariantRequest{
		NamespaceKey: data.NamespaceKey.ValueString(),
		FlagKey:      data.FlagKey.ValueString(),
		Key:          data.Key.ValueString(),
	}

	if !data.Name.IsNull() {
		createReq.Name = data.Name.ValueString()
	}

	if !data.Description.IsNull() {
		createReq.Description = data.Description.ValueString()
	}

	if !data.Attachment.IsNull() {
		createReq.Attachment = data.Attachment.ValueString()
	}

	variant, err := r.client.Flipt().CreateVariant(ctx, createReq)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to create variant, got error: %s", err))
		return
	}

	data.ID = types.StringValue(variant.Id)
	data.Key = types.StringValue(variant.Key)

	if variant.Name != "" {
		data.Name = types.StringValue(variant.Name)
	}

	if variant.Description != "" {
		data.Description = types.StringValue(variant.Description)
	}

	if variant.Attachment != "" {
		data.Attachment = types.StringValue(variant.Attachment)
	}

	if variant.CreatedAt != nil {
		data.CreatedAt = types.StringValue(variant.CreatedAt.AsTime().String())
	}

	if variant.UpdatedAt != nil {
		data.UpdatedAt = types.StringValue(variant.UpdatedAt.AsTime().String())
	}

	tflog.Trace(ctx, "created a variant resource")
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *VariantResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data VariantResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	flag, err := r.client.Flipt().GetFlag(ctx, &flipt.GetFlagRequest{
		NamespaceKey: data.NamespaceKey.ValueString(),
		Key:          data.FlagKey.ValueString(),
	})
	if err != nil {
		resp.State.RemoveResource(ctx)
		return
	}

	// Find the variant in the flag's variants list
	var foundVariant *flipt.Variant
	for _, v := range flag.Variants {
		if v.Id == data.ID.ValueString() {
			foundVariant = v
			break
		}
	}

	if foundVariant == nil {
		resp.State.RemoveResource(ctx)
		return
	}

	data.Key = types.StringValue(foundVariant.Key)

	if foundVariant.Name != "" {
		data.Name = types.StringValue(foundVariant.Name)
	} else {
		data.Name = types.StringNull()
	}

	if foundVariant.Description != "" {
		data.Description = types.StringValue(foundVariant.Description)
	} else {
		data.Description = types.StringNull()
	}

	if foundVariant.Attachment != "" {
		data.Attachment = types.StringValue(foundVariant.Attachment)
	} else {
		data.Attachment = types.StringNull()
	}

	if foundVariant.CreatedAt != nil {
		data.CreatedAt = types.StringValue(foundVariant.CreatedAt.AsTime().String())
	}

	if foundVariant.UpdatedAt != nil {
		data.UpdatedAt = types.StringValue(foundVariant.UpdatedAt.AsTime().String())
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *VariantResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data VariantResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	updateReq := &flipt.UpdateVariantRequest{
		NamespaceKey: data.NamespaceKey.ValueString(),
		FlagKey:      data.FlagKey.ValueString(),
		Id:           data.ID.ValueString(),
		Key:          data.Key.ValueString(),
	}

	if !data.Name.IsNull() {
		updateReq.Name = data.Name.ValueString()
	}

	if !data.Description.IsNull() {
		updateReq.Description = data.Description.ValueString()
	}

	if !data.Attachment.IsNull() {
		updateReq.Attachment = data.Attachment.ValueString()
	}

	variant, err := r.client.Flipt().UpdateVariant(ctx, updateReq)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to update variant, got error: %s", err))
		return
	}

	data.Key = types.StringValue(variant.Key)

	if variant.Name != "" {
		data.Name = types.StringValue(variant.Name)
	}

	if variant.Description != "" {
		data.Description = types.StringValue(variant.Description)
	}

	if variant.Attachment != "" {
		data.Attachment = types.StringValue(variant.Attachment)
	}

	if variant.UpdatedAt != nil {
		data.UpdatedAt = types.StringValue(variant.UpdatedAt.AsTime().String())
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *VariantResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data VariantResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	err := r.client.Flipt().DeleteVariant(ctx, &flipt.DeleteVariantRequest{
		NamespaceKey: data.NamespaceKey.ValueString(),
		FlagKey:      data.FlagKey.ValueString(),
		Id:           data.ID.ValueString(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to delete variant, got error: %s", err))
		return
	}

	tflog.Trace(ctx, "deleted a variant resource")
}

func (r *VariantResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
