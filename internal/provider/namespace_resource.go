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

// Ensure provider defined types fully satisfy framework interfaces.
var _ resource.Resource = &NamespaceResource{}
var _ resource.ResourceWithImportState = &NamespaceResource{}

func NewNamespaceResource() resource.Resource {
	return &NamespaceResource{}
}

// NamespaceResource defines the resource implementation.
type NamespaceResource struct {
	client *sdk.SDK
}

// NamespaceResourceModel describes the resource data model.
type NamespaceResourceModel struct {
	Key         types.String `tfsdk:"key"`
	Name        types.String `tfsdk:"name"`
	Description types.String `tfsdk:"description"`
	Protected   types.Bool   `tfsdk:"protected"`
	CreatedAt   types.String `tfsdk:"created_at"`
	UpdatedAt   types.String `tfsdk:"updated_at"`
}

func (r *NamespaceResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_namespace"
}

func (r *NamespaceResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Flipt namespace resource",

		Attributes: map[string]schema.Attribute{
			"key": schema.StringAttribute{
				MarkdownDescription: "Unique key for the namespace",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "Display name of the namespace",
				Required:            true,
			},
			"description": schema.StringAttribute{
				MarkdownDescription: "Description of the namespace",
				Optional:            true,
			},
			"protected": schema.BoolAttribute{
				MarkdownDescription: "Whether the namespace is protected",
				Optional:            true,
				Computed:            true,
			},
			"created_at": schema.StringAttribute{
				MarkdownDescription: "Timestamp when the namespace was created",
				Computed:            true,
			},
			"updated_at": schema.StringAttribute{
				MarkdownDescription: "Timestamp when the namespace was last updated",
				Computed:            true,
			},
		},
	}
}

func (r *NamespaceResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	// Prevent panic if the provider has not been configured.
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(*sdk.SDK)

	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *sdk.SDK, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)

		return
	}

	r.client = client
}

func (r *NamespaceResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data NamespaceResourceModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Create the namespace using the Flipt SDK
	createReq := &flipt.CreateNamespaceRequest{
		Key:  data.Key.ValueString(),
		Name: data.Name.ValueString(),
	}

	if !data.Description.IsNull() {
		createReq.Description = data.Description.ValueString()
	}

	namespace, err := r.client.Flipt().CreateNamespace(ctx, createReq)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to create namespace, got error: %s", err))
		return
	}

	data.Key = types.StringValue(namespace.Key)
	data.Name = types.StringValue(namespace.Name)

	if namespace.Description != "" {
		data.Description = types.StringValue(namespace.Description)
	}

	if namespace.Protected {
		data.Protected = types.BoolValue(namespace.Protected)
	}

	if namespace.CreatedAt != nil {
		data.CreatedAt = types.StringValue(namespace.CreatedAt.AsTime().String())
	}

	if namespace.UpdatedAt != nil {
		data.UpdatedAt = types.StringValue(namespace.UpdatedAt.AsTime().String())
	}

	tflog.Trace(ctx, "created a namespace resource")

	// Save data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *NamespaceResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data NamespaceResourceModel

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Get the namespace from Flipt
	namespace, err := r.client.Flipt().GetNamespace(ctx, &flipt.GetNamespaceRequest{
		Key: data.Key.ValueString(),
	})
	if err != nil {
		// Namespace no longer exists
		resp.State.RemoveResource(ctx)
		return
	}

	data.Key = types.StringValue(namespace.Key)
	data.Name = types.StringValue(namespace.Name)

	if namespace.Description != "" {
		data.Description = types.StringValue(namespace.Description)
	} else {
		data.Description = types.StringNull()
	}

	if namespace.Protected {
		data.Protected = types.BoolValue(namespace.Protected)
	}

	if namespace.CreatedAt != nil {
		data.CreatedAt = types.StringValue(namespace.CreatedAt.AsTime().String())
	}

	if namespace.UpdatedAt != nil {
		data.UpdatedAt = types.StringValue(namespace.UpdatedAt.AsTime().String())
	}

	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *NamespaceResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data NamespaceResourceModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Update the namespace using the Flipt SDK
	updateReq := &flipt.UpdateNamespaceRequest{
		Key:  data.Key.ValueString(),
		Name: data.Name.ValueString(),
	}

	if !data.Description.IsNull() {
		updateReq.Description = data.Description.ValueString()
	}

	namespace, err := r.client.Flipt().UpdateNamespace(ctx, updateReq)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to update namespace, got error: %s", err))
		return
	}

	data.Name = types.StringValue(namespace.Name)

	if namespace.Description != "" {
		data.Description = types.StringValue(namespace.Description)
	}

	if namespace.Protected {
		data.Protected = types.BoolValue(namespace.Protected)
	}

	if namespace.UpdatedAt != nil {
		data.UpdatedAt = types.StringValue(namespace.UpdatedAt.AsTime().String())
	}

	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *NamespaceResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data NamespaceResourceModel

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Delete the namespace using the Flipt SDK
	err := r.client.Flipt().DeleteNamespace(ctx, &flipt.DeleteNamespaceRequest{
		Key: data.Key.ValueString(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to delete namespace, got error: %s", err))
		return
	}

	tflog.Trace(ctx, "deleted a namespace resource")
}

func (r *NamespaceResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("key"), req, resp)
}
