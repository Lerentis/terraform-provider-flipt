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

var _ resource.Resource = &VariantResource{}
var _ resource.ResourceWithImportState = &VariantResource{}

func NewVariantResource() resource.Resource {
	return &VariantResource{}
}

type VariantResource struct {
	client *flipt.APIClient
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

func (r *VariantResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data VariantResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	createReq := *flipt.NewCreateVariantRequest(data.Key.ValueString())

	if !data.Name.IsNull() {
		name := data.Name.ValueString()
		createReq.Name = &name
	}

	if !data.Description.IsNull() {
		desc := data.Description.ValueString()
		createReq.Description = &desc
	}

	if !data.Attachment.IsNull() {
		attachment := data.Attachment.ValueString()
		createReq.Attachment = &attachment
	}

	variant, httpResp, err := r.client.VariantsServiceAPI.CreateVariant(ctx, data.NamespaceKey.ValueString(), data.FlagKey.ValueString()).CreateVariantRequest(createReq).Execute()
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to create variant, got error: %s", err))
		return
	}

	if httpResp.StatusCode != 200 && httpResp.StatusCode != 201 {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to create variant, got status: %d", httpResp.StatusCode))
		return
	}

	data.ID = types.StringValue(variant.GetId())
	data.Key = types.StringValue(variant.GetKey())

	if name, ok := variant.GetNameOk(); ok {
		data.Name = types.StringValue(*name)
	}

	if desc, ok := variant.GetDescriptionOk(); ok {
		data.Description = types.StringValue(*desc)
	}

	if attachment, ok := variant.GetAttachmentOk(); ok {
		data.Attachment = types.StringValue(*attachment)
	}

	if createdAt, ok := variant.GetCreatedAtOk(); ok {
		data.CreatedAt = types.StringValue(createdAt.String())
	}

	if updatedAt, ok := variant.GetUpdatedAtOk(); ok {
		data.UpdatedAt = types.StringValue(updatedAt.String())
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

	// Note: Variants don't have a direct Get endpoint, we need to list and find
	// For now, we'll just trust the state is accurate
	// A production implementation might want to list all variants and find the matching one

	tflog.Trace(ctx, "read a variant resource")
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *VariantResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data VariantResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	updateReq := *flipt.NewUpdateVariantRequest(data.Key.ValueString())

	if !data.Name.IsNull() {
		name := data.Name.ValueString()
		updateReq.Name = &name
	}

	if !data.Description.IsNull() {
		desc := data.Description.ValueString()
		updateReq.Description = &desc
	}

	if !data.Attachment.IsNull() {
		attachment := data.Attachment.ValueString()
		updateReq.Attachment = &attachment
	}

	variant, httpResp, err := r.client.VariantsServiceAPI.UpdateVariant(ctx, data.NamespaceKey.ValueString(), data.FlagKey.ValueString(), data.ID.ValueString()).UpdateVariantRequest(updateReq).Execute()
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to update variant, got error: %s", err))
		return
	}

	if httpResp.StatusCode != 200 {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to update variant, got status: %d", httpResp.StatusCode))
		return
	}

	data.Key = types.StringValue(variant.GetKey())

	if name, ok := variant.GetNameOk(); ok {
		data.Name = types.StringValue(*name)
	}

	if desc, ok := variant.GetDescriptionOk(); ok {
		data.Description = types.StringValue(*desc)
	}

	if attachment, ok := variant.GetAttachmentOk(); ok {
		data.Attachment = types.StringValue(*attachment)
	}

	if updatedAt, ok := variant.GetUpdatedAtOk(); ok {
		data.UpdatedAt = types.StringValue(updatedAt.String())
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *VariantResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data VariantResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	httpResp, err := r.client.VariantsServiceAPI.DeleteVariant(ctx, data.NamespaceKey.ValueString(), data.FlagKey.ValueString(), data.ID.ValueString()).Execute()
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to delete variant, got error: %s", err))
		return
	}

	if httpResp.StatusCode != 204 && httpResp.StatusCode != 200 {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to delete variant, got status: %d", httpResp.StatusCode))
		return
	}

	tflog.Trace(ctx, "deleted a variant resource")
}

func (r *VariantResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
