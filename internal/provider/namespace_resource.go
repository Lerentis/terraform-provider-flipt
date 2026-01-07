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

// Ensure provider defined types fully satisfy framework interfaces.
var _ resource.Resource = &NamespaceResource{}
var _ resource.ResourceWithImportState = &NamespaceResource{}

func NewNamespaceResource() resource.Resource {
	return &NamespaceResource{}
}

// NamespaceResource defines the resource implementation.
type NamespaceResource struct {
	client *flipt.APIClient
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

	client, ok := req.ProviderData.(*flipt.APIClient)

	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *flipt.APIClient, got: %T. Please report this issue to the provider developers.", req.ProviderData),
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
	createReq := *flipt.NewCreateNamespaceRequest(data.Key.ValueString(), data.Name.ValueString())

	if !data.Description.IsNull() {
		desc := data.Description.ValueString()
		createReq.Description = &desc
	}

	namespace, httpResp, err := r.client.NamespacesServiceAPI.CreateNamespace(ctx).CreateNamespaceRequest(createReq).Execute()
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to create namespace, got error: %s", err))
		return
	}

	if httpResp.StatusCode != 200 && httpResp.StatusCode != 201 {
		resp.Diagnostics.AddError(
			"API Error",
			fmt.Sprintf("Unable to create namespace, got status: %d", httpResp.StatusCode),
		)
		return
	}
	data.Key = types.StringValue(namespace.GetKey())
	data.Name = types.StringValue(namespace.GetName())

	if desc, ok := namespace.GetDescriptionOk(); ok {
		data.Description = types.StringValue(*desc)
	}

	if protected, ok := namespace.GetProtectedOk(); ok {
		data.Protected = types.BoolValue(*protected)
	}

	if createdAt, ok := namespace.GetCreatedAtOk(); ok {
		data.CreatedAt = types.StringValue(createdAt.String())
	}

	if updatedAt, ok := namespace.GetUpdatedAtOk(); ok {
		data.UpdatedAt = types.StringValue(updatedAt.String())
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
	namespace, httpResp, err := r.client.NamespacesServiceAPI.GetNamespace(ctx, data.Key.ValueString()).Execute()
	if err != nil {
		if httpResp != nil && httpResp.StatusCode == 404 {
			// Namespace no longer exists
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to read namespace, got error: %s", err))
		return
	}

	if httpResp.StatusCode != 200 {
		resp.Diagnostics.AddError(
			"API Error",
			fmt.Sprintf("Unable to read namespace, got status: %d", httpResp.StatusCode),
		)
		return
	}
	data.Key = types.StringValue(namespace.GetKey())
	data.Name = types.StringValue(namespace.GetName())

	if desc, ok := namespace.GetDescriptionOk(); ok {
		data.Description = types.StringValue(*desc)
	} else {
		data.Description = types.StringNull()
	}

	if protected, ok := namespace.GetProtectedOk(); ok {
		data.Protected = types.BoolValue(*protected)
	}

	if createdAt, ok := namespace.GetCreatedAtOk(); ok {
		data.CreatedAt = types.StringValue(createdAt.String())
	}

	if updatedAt, ok := namespace.GetUpdatedAtOk(); ok {
		data.UpdatedAt = types.StringValue(updatedAt.String())
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
	updateReq := *flipt.NewUpdateNamespaceRequest(data.Name.ValueString())

	if !data.Description.IsNull() {
		desc := data.Description.ValueString()
		updateReq.Description = &desc
	}

	namespace, httpResp, err := r.client.NamespacesServiceAPI.UpdateNamespace(ctx, data.Key.ValueString()).UpdateNamespaceRequest(updateReq).Execute()
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to update namespace, got error: %s", err))
		return
	}

	if httpResp.StatusCode != 200 {
		resp.Diagnostics.AddError(
			"API Error",
			fmt.Sprintf("Unable to update namespace, got status: %d", httpResp.StatusCode),
		)
		return
	}
	data.Name = types.StringValue(namespace.GetName())

	if desc, ok := namespace.GetDescriptionOk(); ok {
		data.Description = types.StringValue(*desc)
	}

	if protected, ok := namespace.GetProtectedOk(); ok {
		data.Protected = types.BoolValue(*protected)
	}

	if updatedAt, ok := namespace.GetUpdatedAtOk(); ok {
		data.UpdatedAt = types.StringValue(updatedAt.String())
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
	httpResp, err := r.client.NamespacesServiceAPI.DeleteNamespace(ctx, data.Key.ValueString()).Execute()
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to delete namespace, got error: %s", err))
		return
	}

	if httpResp.StatusCode != 204 && httpResp.StatusCode != 200 {
		resp.Diagnostics.AddError(
			"API Error",
			fmt.Sprintf("Unable to delete namespace, got status: %d", httpResp.StatusCode),
		)
		return
	}

	tflog.Trace(ctx, "deleted a namespace resource")
}

func (r *NamespaceResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("key"), req, resp)
}
