// Copyright (c) terraform-provider-flipt contributors
// SPDX-License-Identifier: MIT

package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ resource.Resource = &NamespaceResource{}
var _ resource.ResourceWithImportState = &NamespaceResource{}

func NewNamespaceResource() resource.Resource {
	return &NamespaceResource{}
}

// NamespaceResource defines the resource implementation.
type NamespaceResource struct {
	config *FliptProviderConfig
}

// NamespaceResourceModel describes the resource data model.
type NamespaceResourceModel struct {
	EnvironmentKey types.String `tfsdk:"environment_key"`
	Key            types.String `tfsdk:"key"`
	Name           types.String `tfsdk:"name"`
	Description    types.String `tfsdk:"description"`
	Protected      types.Bool   `tfsdk:"protected"`
}

func (r *NamespaceResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_namespace"
}

func (r *NamespaceResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Flipt namespace resource",

		Attributes: map[string]schema.Attribute{
			"environment_key": schema.StringAttribute{
				MarkdownDescription: "Environment key (defaults to 'default')",
				Optional:            true,
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
					stringplanmodifier.RequiresReplace(),
				},
			},
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
		},
	}
}

func (r *NamespaceResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	// Prevent panic if the provider has not been configured.
	if req.ProviderData == nil {
		return
	}

	providerConfig, ok := req.ProviderData.(*FliptProviderConfig)

	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *FliptProviderConfig, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)

		return
	}

	r.config = providerConfig
}

func (r *NamespaceResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data NamespaceResourceModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Default to "default" environment if not specified
	envKey := "default"
	if !data.EnvironmentKey.IsNull() && !data.EnvironmentKey.IsUnknown() {
		envKey = data.EnvironmentKey.ValueString()
	}
	data.EnvironmentKey = types.StringValue(envKey)

	tflog.Debug(ctx, "Creating namespace", map[string]interface{}{
		"environment_key": envKey,
		"key":             data.Key.ValueString(),
		"name":            data.Name.ValueString(),
	})

	// Create the namespace using manual HTTP request
	createReq := map[string]interface{}{
		"key":  data.Key.ValueString(),
		"name": data.Name.ValueString(),
	}

	if !data.Description.IsNull() {
		createReq["description"] = data.Description.ValueString()
	}

	reqBody, err := json.Marshal(createReq)
	if err != nil {
		resp.Diagnostics.AddError("Serialization Error", fmt.Sprintf("Unable to marshal request: %s", err))
		return
	}

	url := fmt.Sprintf("%s/api/v2/environments/%s/namespaces", r.config.Endpoint, envKey)
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(reqBody))
	if err != nil {
		resp.Diagnostics.AddError("Request Error", fmt.Sprintf("Unable to create request: %s", err))
		return
	}
	httpReq.Header.Set("Content-Type", "application/json")

	tflog.Debug(ctx, "Making HTTP request", map[string]interface{}{
		"method":          "POST",
		"url":             url,
		"environment_key": envKey,
		"key":             data.Key.ValueString(),
	})

	r.config.AddAuthHeader(httpReq)
	httpResp, err := r.config.HTTPClient.Do(httpReq)
	if err != nil {
		tflog.Error(ctx, "Failed to create namespace", map[string]interface{}{
			"error":           err.Error(),
			"environment_key": envKey,
			"key":             data.Key.ValueString(),
		})
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to create namespace, got error: %s", err))
		return
	}
	defer httpResp.Body.Close()

	// Read the response body first so we can log it
	body, err := io.ReadAll(httpResp.Body)
	if err != nil {
		resp.Diagnostics.AddError("Read Error", fmt.Sprintf("Unable to read response: %s", err))
		return
	}

	tflog.Debug(ctx, "Received create response", map[string]interface{}{
		"status_code":     httpResp.StatusCode,
		"response_body":   string(body),
		"environment_key": envKey,
		"key":             data.Key.ValueString(),
	})

	if httpResp.StatusCode != http.StatusOK && httpResp.StatusCode != http.StatusCreated {
		tflog.Error(ctx, "Failed to create namespace", map[string]interface{}{
			"status_code":     httpResp.StatusCode,
			"response_body":   string(body),
			"environment_key": envKey,
			"key":             data.Key.ValueString(),
		})
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to create namespace, status: %d, body: %s", httpResp.StatusCode, string(body)))
		return
	}

	var response struct {
		Namespace struct {
			Key         string `json:"key"`
			Name        string `json:"name"`
			Description string `json:"description"`
			Protected   bool   `json:"protected"`
		} `json:"namespace"`
		Revision string `json:"revision"`
	}

	if err := json.Unmarshal(body, &response); err != nil {
		resp.Diagnostics.AddError("Parse Error", fmt.Sprintf("Unable to parse response: %s, body: %s", err, string(body)))
		return
	}

	namespace := response.Namespace

	tflog.Debug(ctx, "Parsed namespace from response", map[string]interface{}{
		"key":         namespace.Key,
		"name":        namespace.Name,
		"description": namespace.Description,
		"protected":   namespace.Protected,
	})

	// Don't overwrite Required fields (key, name) - they should already be set from the plan
	// Only set Optional fields if returned
	if namespace.Description != "" {
		data.Description = types.StringValue(namespace.Description)
	}

	// Always set Computed fields
	data.Protected = types.BoolValue(namespace.Protected)

	tflog.Debug(ctx, "Saving state after create", map[string]interface{}{
		"key":       data.Key.ValueString(),
		"name":      data.Name.ValueString(),
		"protected": data.Protected.ValueBool(),
	})

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

	// Default to "default" environment if not specified
	envKey := "default"
	if !data.EnvironmentKey.IsNull() && !data.EnvironmentKey.IsUnknown() {
		envKey = data.EnvironmentKey.ValueString()
	}

	tflog.Debug(ctx, "Reading namespace", map[string]interface{}{
		"environment_key": envKey,
		"key":             data.Key.ValueString(),
	})

	// Get the namespace from Flipt
	url := fmt.Sprintf("%s/api/v2/environments/%s/namespaces/%s", r.config.Endpoint, envKey, data.Key.ValueString())
	httpReq, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		resp.Diagnostics.AddError("Request Error", fmt.Sprintf("Unable to create request: %s", err))
		return
	}

	r.config.AddAuthHeader(httpReq)
	httpResp, err := r.config.HTTPClient.Do(httpReq)
	if err != nil {
		tflog.Warn(ctx, "Namespace not found, removing from state", map[string]interface{}{
			"error":           err.Error(),
			"environment_key": envKey,
			"key":             data.Key.ValueString(),
		})
		resp.State.RemoveResource(ctx)
		return
	}
	defer httpResp.Body.Close()

	if httpResp.StatusCode == http.StatusNotFound {
		tflog.Warn(ctx, "Namespace not found, removing from state", map[string]interface{}{
			"environment_key": envKey,
			"key":             data.Key.ValueString(),
		})
		resp.State.RemoveResource(ctx)
		return
	}

	if httpResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(httpResp.Body)
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to read namespace, status: %d, body: %s", httpResp.StatusCode, string(body)))
		return
	}

	var response struct {
		Namespace struct {
			Key         string `json:"key"`
			Name        string `json:"name"`
			Description string `json:"description"`
			Protected   bool   `json:"protected"`
		} `json:"namespace"`
		Revision string `json:"revision"`
	}

	if err := json.NewDecoder(httpResp.Body).Decode(&response); err != nil {
		resp.Diagnostics.AddError("Parse Error", fmt.Sprintf("Unable to parse response: %s", err))
		return
	}

	namespace := response.Namespace

	// Don't overwrite Required fields (key, name) - they should remain as they are in state
	// Only update Optional and Computed fields
	if namespace.Description != "" {
		data.Description = types.StringValue(namespace.Description)
	} else {
		data.Description = types.StringNull()
	}

	data.Protected = types.BoolValue(namespace.Protected)

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

	// Default to "default" environment if not specified
	envKey := "default"
	if !data.EnvironmentKey.IsNull() && !data.EnvironmentKey.IsUnknown() {
		envKey = data.EnvironmentKey.ValueString()
	}

	tflog.Debug(ctx, "Updating namespace", map[string]interface{}{
		"environment_key": envKey,
		"key":             data.Key.ValueString(),
		"name":            data.Name.ValueString(),
	})

	// Update the namespace
	updateReq := map[string]interface{}{
		"key":  data.Key.ValueString(),
		"name": data.Name.ValueString(),
	}

	if !data.Description.IsNull() {
		updateReq["description"] = data.Description.ValueString()
	}

	reqBody, err := json.Marshal(updateReq)
	if err != nil {
		resp.Diagnostics.AddError("Serialization Error", fmt.Sprintf("Unable to marshal request: %s", err))
		return
	}

	url := fmt.Sprintf("%s/api/v2/environments/%s/namespaces", r.config.Endpoint, envKey)
	httpReq, err := http.NewRequestWithContext(ctx, "PUT", url, bytes.NewReader(reqBody))
	if err != nil {
		resp.Diagnostics.AddError("Request Error", fmt.Sprintf("Unable to create request: %s", err))
		return
	}
	httpReq.Header.Set("Content-Type", "application/json")

	r.config.AddAuthHeader(httpReq)
	httpResp, err := r.config.HTTPClient.Do(httpReq)
	if err != nil {
		tflog.Error(ctx, "Failed to update namespace", map[string]interface{}{
			"error":           err.Error(),
			"environment_key": envKey,
			"key":             data.Key.ValueString(),
		})
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to update namespace, got error: %s", err))
		return
	}
	defer httpResp.Body.Close()

	if httpResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(httpResp.Body)
		tflog.Error(ctx, "Failed to update namespace", map[string]interface{}{
			"status_code":     httpResp.StatusCode,
			"response_body":   string(body),
			"environment_key": envKey,
			"key":             data.Key.ValueString(),
		})
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to update namespace, status: %d, body: %s", httpResp.StatusCode, string(body)))
		return
	}

	var response struct {
		Namespace struct {
			Key         string `json:"key"`
			Name        string `json:"name"`
			Description string `json:"description"`
			Protected   bool   `json:"protected"`
		} `json:"namespace"`
		Revision string `json:"revision"`
	}

	if err := json.NewDecoder(httpResp.Body).Decode(&response); err != nil {
		resp.Diagnostics.AddError("Parse Error", fmt.Sprintf("Unable to parse response: %s", err))
		return
	}

	namespace := response.Namespace

	// Don't overwrite Required fields (key, name) - use values from plan
	// Only update Optional and Computed fields
	if namespace.Description != "" {
		data.Description = types.StringValue(namespace.Description)
	} else {
		data.Description = types.StringNull()
	}

	data.Protected = types.BoolValue(namespace.Protected)

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

	tflog.Debug(ctx, "State data retrieved", map[string]interface{}{
		"key_is_null":    data.Key.IsNull(),
		"key_is_unknown": data.Key.IsUnknown(),
		"key_value":      data.Key.ValueString(),
		"raw_state":      fmt.Sprintf("%+v", data),
	})

	// Validate that key is present
	if data.Key.IsNull() || data.Key.ValueString() == "" {
		resp.Diagnostics.AddError("Missing Namespace Key",
			"The namespace key is required for deletion but was not found in the state. This may indicate a state corruption issue.")
		return
	}

	// Default to "default" environment if not specified
	envKey := "default"
	if !data.EnvironmentKey.IsNull() && !data.EnvironmentKey.IsUnknown() {
		envKey = data.EnvironmentKey.ValueString()
	}

	tflog.Debug(ctx, "Deleting namespace", map[string]interface{}{
		"environment_key": envKey,
		"key":             data.Key.ValueString(),
	})

	// Delete the namespace
	url := fmt.Sprintf("%s/api/v2/environments/%s/namespaces/%s", r.config.Endpoint, envKey, data.Key.ValueString())

	tflog.Debug(ctx, "Making DELETE request", map[string]interface{}{
		"method":          "DELETE",
		"url":             url,
		"endpoint":        r.config.Endpoint,
		"environment_key": envKey,
		"key":             data.Key.ValueString(),
	})

	httpReq, err := http.NewRequestWithContext(ctx, "DELETE", url, nil)
	if err != nil {
		resp.Diagnostics.AddError("Request Error", fmt.Sprintf("Unable to create request: %s", err))
		return
	}

	tflog.Debug(ctx, "HTTP Request details", map[string]interface{}{
		"method": httpReq.Method,
		"url":    httpReq.URL.String(),
		"host":   httpReq.Host,
		"header": fmt.Sprintf("%v", httpReq.Header),
	})

	r.config.AddAuthHeader(httpReq)
	httpResp, err := r.config.HTTPClient.Do(httpReq)
	if err != nil {
		tflog.Error(ctx, "Failed to delete namespace", map[string]interface{}{
			"error":           err.Error(),
			"environment_key": envKey,
			"key":             data.Key.ValueString(),
		})
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to delete namespace, got error: %s", err))
		return
	}
	defer httpResp.Body.Close()

	tflog.Debug(ctx, "Received DELETE response", map[string]interface{}{
		"status_code":     httpResp.StatusCode,
		"environment_key": envKey,
		"key":             data.Key.ValueString(),
	})

	// If namespace is already gone (404), consider it a success
	if httpResp.StatusCode == http.StatusNotFound {
		tflog.Debug(ctx, "Namespace already deleted", map[string]interface{}{
			"environment_key": envKey,
			"key":             data.Key.ValueString(),
		})
		return
	}

	if httpResp.StatusCode != http.StatusOK && httpResp.StatusCode != http.StatusNoContent {
		body, _ := io.ReadAll(httpResp.Body)
		tflog.Error(ctx, "Failed to delete namespace", map[string]interface{}{
			"status_code":     httpResp.StatusCode,
			"response_body":   string(body),
			"environment_key": envKey,
			"key":             data.Key.ValueString(),
			"url":             url,
		})

		// If namespace is protected or has resources, provide a helpful message
		if httpResp.StatusCode == http.StatusNotImplemented || httpResp.StatusCode == http.StatusMethodNotAllowed {
			resp.Diagnostics.AddError("Namespace Cannot Be Deleted",
				fmt.Sprintf("Unable to delete namespace '%s'. The namespace may be protected or contain resources that must be deleted first. Status: %d, Response: %s",
					data.Key.ValueString(), httpResp.StatusCode, string(body)))
		} else {
			resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to delete namespace, status: %d, body: %s", httpResp.StatusCode, string(body)))
		}
		return
	}

	tflog.Trace(ctx, "deleted a namespace resource")
}

func (r *NamespaceResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("key"), req, resp)
}
