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

var _ resource.Resource = &FlagResource{}
var _ resource.ResourceWithImportState = &FlagResource{}

func NewFlagResource() resource.Resource {
	return &FlagResource{}
}

type FlagResource struct {
	client *flipt.APIClient
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
				Required:            true,
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

func (r *FlagResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data FlagResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	flagType := "VARIANT_FLAG_TYPE"
	if !data.Type.IsNull() {
		flagType = data.Type.ValueString()
	}

	createReq := *flipt.NewCreateFlagRequest(data.Key.ValueString(), data.Name.ValueString(), flagType)
	enabled := data.Enabled.ValueBool()
	createReq.Enabled = &enabled

	if !data.Description.IsNull() {
		desc := data.Description.ValueString()
		createReq.Description = &desc
	}

	flag, httpResp, err := r.client.FlagsServiceAPI.CreateFlag(ctx, data.NamespaceKey.ValueString()).CreateFlagRequest(createReq).Execute()
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to create flag, got error: %s", err))
		return
	}

	if httpResp.StatusCode != 200 && httpResp.StatusCode != 201 {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to create flag, got status: %d", httpResp.StatusCode))
		return
	}

	data.Key = types.StringValue(flag.GetKey())
	data.Name = types.StringValue(flag.GetName())
	data.Enabled = types.BoolValue(flag.GetEnabled())

	if desc, ok := flag.GetDescriptionOk(); ok {
		data.Description = types.StringValue(*desc)
	}

	if flagType, ok := flag.GetTypeOk(); ok {
		data.Type = types.StringValue(*flagType)
	}

	if createdAt, ok := flag.GetCreatedAtOk(); ok {
		data.CreatedAt = types.StringValue(createdAt.String())
	}

	if updatedAt, ok := flag.GetUpdatedAtOk(); ok {
		data.UpdatedAt = types.StringValue(updatedAt.String())
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

	flag, httpResp, err := r.client.FlagsServiceAPI.GetFlag(ctx, data.NamespaceKey.ValueString(), data.Key.ValueString()).Execute()
	if err != nil {
		if httpResp != nil && httpResp.StatusCode == 404 {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to read flag, got error: %s", err))
		return
	}

	data.Key = types.StringValue(flag.GetKey())
	data.Name = types.StringValue(flag.GetName())
	data.Enabled = types.BoolValue(flag.GetEnabled())

	if desc, ok := flag.GetDescriptionOk(); ok {
		data.Description = types.StringValue(*desc)
	} else {
		data.Description = types.StringNull()
	}

	if flagType, ok := flag.GetTypeOk(); ok {
		data.Type = types.StringValue(*flagType)
	}

	if createdAt, ok := flag.GetCreatedAtOk(); ok {
		data.CreatedAt = types.StringValue(createdAt.String())
	}

	if updatedAt, ok := flag.GetUpdatedAtOk(); ok {
		data.UpdatedAt = types.StringValue(updatedAt.String())
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *FlagResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data FlagResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	updateReq := *flipt.NewUpdateFlagRequest(data.Name.ValueString())
	enabled := data.Enabled.ValueBool()
	updateReq.Enabled = &enabled

	if !data.Description.IsNull() {
		desc := data.Description.ValueString()
		updateReq.Description = &desc
	}

	flag, httpResp, err := r.client.FlagsServiceAPI.UpdateFlag(ctx, data.NamespaceKey.ValueString(), data.Key.ValueString()).UpdateFlagRequest(updateReq).Execute()
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to update flag, got error: %s", err))
		return
	}

	if httpResp.StatusCode != 200 {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to update flag, got status: %d", httpResp.StatusCode))
		return
	}

	data.Name = types.StringValue(flag.GetName())
	data.Enabled = types.BoolValue(flag.GetEnabled())

	if desc, ok := flag.GetDescriptionOk(); ok {
		data.Description = types.StringValue(*desc)
	}

	if updatedAt, ok := flag.GetUpdatedAtOk(); ok {
		data.UpdatedAt = types.StringValue(updatedAt.String())
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *FlagResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data FlagResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	httpResp, err := r.client.FlagsServiceAPI.DeleteFlag(ctx, data.NamespaceKey.ValueString(), data.Key.ValueString()).Execute()
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to delete flag, got error: %s", err))
		return
	}

	if httpResp.StatusCode != 204 && httpResp.StatusCode != 200 {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to delete flag, got status: %d", httpResp.StatusCode))
		return
	}

	tflog.Trace(ctx, "deleted a flag resource")
}

func (r *FlagResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("key"), req, resp)
}
