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

var _ resource.Resource = &FlagResource{}
var _ resource.ResourceWithImportState = &FlagResource{}

func NewFlagResource() resource.Resource {
	return &FlagResource{}
}

type FlagResource struct {
	client *sdk.SDK
}

type FlagResourceModel struct {
	NamespaceKey types.String `tfsdk:"namespace_key"`
	Key          types.String `tfsdk:"key"`
	Name         types.String `tfsdk:"name"`
	Description  types.String `tfsdk:"description"`
	Enabled      types.Bool   `tfsdk:"enabled"`
	Type         types.String `tfsdk:"type"`
	CreatedAt    types.String `tfsdk:"created_at"`
	UpdatedAt    types.String `tfsdk:"updated_at"`
}

func (r *FlagResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_flag"
}

func (r *FlagResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Flipt flag resource",

		Attributes: map[string]schema.Attribute{
			"namespace_key": schema.StringAttribute{
				MarkdownDescription: "Namespace key where the flag belongs",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"key": schema.StringAttribute{
				MarkdownDescription: "Unique key for the flag",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "Display name of the flag",
				Required:            true,
			},
			"description": schema.StringAttribute{
				MarkdownDescription: "Description of the flag",
				Optional:            true,
			},
			"enabled": schema.BoolAttribute{
				MarkdownDescription: "Whether the flag is enabled",
				Optional:            true,
				Computed:            true,
			},
			"type": schema.StringAttribute{
				MarkdownDescription: "Type of the flag (VARIANT_FLAG_TYPE or BOOLEAN_FLAG_TYPE)",
				Optional:            true,
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

func (r *FlagResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *FlagResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data FlagResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	flagType := flipt.FlagType_VARIANT_FLAG_TYPE
	if !data.Type.IsNull() && !data.Type.IsUnknown() {
		if data.Type.ValueString() == "BOOLEAN_FLAG_TYPE" {
			flagType = flipt.FlagType_BOOLEAN_FLAG_TYPE
		}
	}

	createReq := &flipt.CreateFlagRequest{
		NamespaceKey: data.NamespaceKey.ValueString(),
		Key:          data.Key.ValueString(),
		Name:         data.Name.ValueString(),
		Type:         flagType,
		Enabled:      data.Enabled.ValueBool(),
	}

	if !data.Description.IsNull() {
		createReq.Description = data.Description.ValueString()
	}

	flag, err := r.client.Flipt().CreateFlag(ctx, createReq)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to create flag, got error: %s", err))
		return
	}

	data.Key = types.StringValue(flag.Key)
	data.Name = types.StringValue(flag.Name)
	data.Enabled = types.BoolValue(flag.Enabled)

	if flag.Description != "" {
		data.Description = types.StringValue(flag.Description)
	}

	data.Type = types.StringValue(flag.Type.String())

	if flag.CreatedAt != nil {
		data.CreatedAt = types.StringValue(flag.CreatedAt.AsTime().String())
	}

	if flag.UpdatedAt != nil {
		data.UpdatedAt = types.StringValue(flag.UpdatedAt.AsTime().String())
	}

	tflog.Trace(ctx, "created a flag resource")
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *FlagResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data FlagResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	flag, err := r.client.Flipt().GetFlag(ctx, &flipt.GetFlagRequest{
		NamespaceKey: data.NamespaceKey.ValueString(),
		Key:          data.Key.ValueString(),
	})
	if err != nil {
		resp.State.RemoveResource(ctx)
		return
	}

	data.Key = types.StringValue(flag.Key)
	data.Name = types.StringValue(flag.Name)
	data.Enabled = types.BoolValue(flag.Enabled)

	if flag.Description != "" {
		data.Description = types.StringValue(flag.Description)
	} else {
		data.Description = types.StringNull()
	}

	data.Type = types.StringValue(flag.Type.String())

	if flag.CreatedAt != nil {
		data.CreatedAt = types.StringValue(flag.CreatedAt.AsTime().String())
	}

	if flag.UpdatedAt != nil {
		data.UpdatedAt = types.StringValue(flag.UpdatedAt.AsTime().String())
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *FlagResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data FlagResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	updateReq := &flipt.UpdateFlagRequest{
		NamespaceKey: data.NamespaceKey.ValueString(),
		Key:          data.Key.ValueString(),
		Name:         data.Name.ValueString(),
		Enabled:      data.Enabled.ValueBool(),
	}

	if !data.Description.IsNull() {
		updateReq.Description = data.Description.ValueString()
	}

	flag, err := r.client.Flipt().UpdateFlag(ctx, updateReq)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to update flag, got error: %s", err))
		return
	}

	data.Name = types.StringValue(flag.Name)
	data.Enabled = types.BoolValue(flag.Enabled)

	if flag.Description != "" {
		data.Description = types.StringValue(flag.Description)
	}

	data.Type = types.StringValue(flag.Type.String())

	if flag.UpdatedAt != nil {
		data.UpdatedAt = types.StringValue(flag.UpdatedAt.AsTime().String())
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *FlagResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data FlagResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	err := r.client.Flipt().DeleteFlag(ctx, &flipt.DeleteFlagRequest{
		NamespaceKey: data.NamespaceKey.ValueString(),
		Key:          data.Key.ValueString(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to delete flag, got error: %s", err))
		return
	}

	tflog.Trace(ctx, "deleted a flag resource")
}

func (r *FlagResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("key"), req, resp)
}
