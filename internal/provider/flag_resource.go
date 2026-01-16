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
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

var _ resource.Resource = &FlagResource{}
var _ resource.ResourceWithImportState = &FlagResource{}

func NewFlagResource() resource.Resource {
	return &FlagResource{}
}

type FlagResource struct {
	config *FliptProviderConfig
}

type FlagResourceModel struct {
	NamespaceKey   types.String `tfsdk:"namespace_key"`
	EnvironmentKey types.String `tfsdk:"environment_key"`
	Key            types.String `tfsdk:"key"`
	Name           types.String `tfsdk:"name"`
	Description    types.String `tfsdk:"description"`
	Enabled        types.Bool   `tfsdk:"enabled"`
	Type           types.String `tfsdk:"type"`
	Metadata       types.Map    `tfsdk:"metadata"`
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
				Default:             booldefault.StaticBool(false),
			},
			"type": schema.StringAttribute{
				MarkdownDescription: "Type of the flag (VARIANT_FLAG_TYPE or BOOLEAN_FLAG_TYPE)",
				Optional:            true,
				Computed:            true,
				Default:             stringdefault.StaticString("VARIANT_FLAG_TYPE"),
			},
			"metadata": schema.MapAttribute{
				MarkdownDescription: "Metadata key-value pairs for the flag",
				Optional:            true,
				ElementType:         types.StringType,
			},
		},
	}
}

func (r *FlagResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *FlagResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data FlagResourceModel
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

	tflog.Debug(ctx, "Creating flag", map[string]interface{}{
		"environment_key": envKey,
		"namespace_key":   data.NamespaceKey.ValueString(),
		"key":             data.Key.ValueString(),
		"name":            data.Name.ValueString(),
	})

	// Build flag payload
	flagPayload := map[string]interface{}{
		"@type":   "flipt.core.Flag",
		"key":     data.Key.ValueString(),
		"name":    data.Name.ValueString(),
		"type":    data.Type.ValueString(),
		"enabled": data.Enabled.ValueBool(),
	}

	if !data.Description.IsNull() && !data.Description.IsUnknown() {
		flagPayload["description"] = data.Description.ValueString()
	}

	// Add metadata if provided
	if !data.Metadata.IsNull() && !data.Metadata.IsUnknown() {
		metadataMap := make(map[string]string)
		diags := data.Metadata.ElementsAs(ctx, &metadataMap, false)
		if diags.HasError() {
			resp.Diagnostics.Append(diags...)
			return
		}
		if len(metadataMap) > 0 {
			// Convert to map[string]interface{} for JSON marshaling
			metadata := make(map[string]interface{})
			for k, v := range metadataMap {
				metadata[k] = v
			}
			flagPayload["metadata"] = metadata
		}
	}

	// Wrap in resources API format
	createReq := map[string]interface{}{
		"key":     data.Key.ValueString(),
		"payload": flagPayload,
	}

	reqBody, err := json.Marshal(createReq)
	if err != nil {
		resp.Diagnostics.AddError("Serialization Error", fmt.Sprintf("Unable to marshal request: %s", err))
		return
	}

	url := fmt.Sprintf("%s/api/v2/environments/%s/namespaces/%s/resources", r.config.Endpoint, envKey, data.NamespaceKey.ValueString())
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(reqBody))
	if err != nil {
		resp.Diagnostics.AddError("Request Error", fmt.Sprintf("Unable to create request: %s", err))
		return
	}
	httpReq.Header.Set("Content-Type", "application/json")
	r.config.AddAuthHeader(httpReq)

	httpResp, err := r.config.HTTPClient.Do(httpReq)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to create flag, got error: %s", err))
		return
	}
	defer httpResp.Body.Close()

	body, err := io.ReadAll(httpResp.Body)
	if err != nil {
		resp.Diagnostics.AddError("Read Error", fmt.Sprintf("Unable to read response: %s", err))
		return
	}

	if httpResp.StatusCode != http.StatusOK && httpResp.StatusCode != http.StatusCreated {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to create flag, status: %d, body: %s", httpResp.StatusCode, string(body)))
		return
	}

	// Parse response with correct structure: {"resource": {"namespaceKey": "...", "key": "...", "payload": {...}}}
	var response struct {
		Resource struct {
			NamespaceKey string `json:"namespaceKey"`
			Key          string `json:"key"`
			Payload      struct {
				Type        string                 `json:"type"`
				Key         string                 `json:"key"`
				Name        string                 `json:"name"`
				Description string                 `json:"description"`
				Enabled     bool                   `json:"enabled"`
				Metadata    map[string]interface{} `json:"metadata"`
			} `json:"payload"`
		} `json:"resource"`
		Revision string `json:"revision"`
	}

	if err := json.Unmarshal(body, &response); err != nil {
		resp.Diagnostics.AddError("Parse Error", fmt.Sprintf("Unable to parse response: %s, body: %s", err, string(body)))
		return
	}

	flag := response.Resource.Payload

	// Set optional and computed fields from response
	if flag.Description != "" {
		data.Description = types.StringValue(flag.Description)
	}

	data.Enabled = types.BoolValue(flag.Enabled)
	data.Type = types.StringValue(flag.Type)

	// Set metadata if present in response
	if len(flag.Metadata) > 0 {
		metadataMap := make(map[string]string)
		for k, v := range flag.Metadata {
			// Convert interface{} to string for storage
			metadataMap[k] = fmt.Sprintf("%v", v)
		}
		metadataValue, diags := types.MapValueFrom(ctx, types.StringType, metadataMap)
		if diags.HasError() {
			resp.Diagnostics.Append(diags...)
			return
		}
		data.Metadata = metadataValue
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

	// Default to "default" environment if not specified
	envKey := "default"
	if !data.EnvironmentKey.IsNull() && !data.EnvironmentKey.IsUnknown() {
		envKey = data.EnvironmentKey.ValueString()
	}

	tflog.Debug(ctx, "Reading flag", map[string]interface{}{
		"environment_key": envKey,
		"namespace_key":   data.NamespaceKey.ValueString(),
		"key":             data.Key.ValueString(),
	})

	// GET URL includes flipt.core.Flag prefix
	url := fmt.Sprintf("%s/api/v2/environments/%s/namespaces/%s/resources/flipt.core.Flag/%s", r.config.Endpoint, envKey, data.NamespaceKey.ValueString(), data.Key.ValueString())
	httpReq, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		resp.Diagnostics.AddError("Request Error", fmt.Sprintf("Unable to create request: %s", err))
		return
	}
	r.config.AddAuthHeader(httpReq)

	httpResp, err := r.config.HTTPClient.Do(httpReq)
	if err != nil {
		resp.State.RemoveResource(ctx)
		return
	}
	defer httpResp.Body.Close()

	if httpResp.StatusCode == http.StatusNotFound {
		resp.State.RemoveResource(ctx)
		return
	}

	body, err := io.ReadAll(httpResp.Body)
	if err != nil {
		resp.Diagnostics.AddError("Read Error", fmt.Sprintf("Unable to read response: %s", err))
		return
	}

	if httpResp.StatusCode != http.StatusOK {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to read flag, status: %d, body: %s", httpResp.StatusCode, string(body)))
		return
	}

	// Parse response with correct structure
	var response struct {
		Resource struct {
			NamespaceKey string `json:"namespaceKey"`
			Key          string `json:"key"`
			Payload      struct {
				Type        string                 `json:"type"`
				Key         string                 `json:"key"`
				Name        string                 `json:"name"`
				Description string                 `json:"description"`
				Enabled     bool                   `json:"enabled"`
				Metadata    map[string]interface{} `json:"metadata"`
			} `json:"payload"`
		} `json:"resource"`
		Revision string `json:"revision"`
	}

	if err := json.Unmarshal(body, &response); err != nil {
		resp.Diagnostics.AddError("Parse Error", fmt.Sprintf("Unable to parse response: %s", err))
		return
	}

	flag := response.Resource.Payload

	// Don't overwrite Required fields (namespace_key, key, name) - preserve from state
	// Only update Optional and Computed fields
	if flag.Description != "" {
		data.Description = types.StringValue(flag.Description)
	} else {
		data.Description = types.StringNull()
	}

	data.Enabled = types.BoolValue(flag.Enabled)
	data.Type = types.StringValue(flag.Type)

	// Update metadata
	if len(flag.Metadata) > 0 {
		metadataMap := make(map[string]string)
		for k, v := range flag.Metadata {
			metadataMap[k] = fmt.Sprintf("%v", v)
		}
		metadataValue, diags := types.MapValueFrom(ctx, types.StringType, metadataMap)
		if diags.HasError() {
			resp.Diagnostics.Append(diags...)
			return
		}
		data.Metadata = metadataValue
	} else {
		data.Metadata = types.MapNull(types.StringType)
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *FlagResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data FlagResourceModel
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

	tflog.Debug(ctx, "Updating flag", map[string]interface{}{
		"environment_key": envKey,
		"namespace_key":   data.NamespaceKey.ValueString(),
		"key":             data.Key.ValueString(),
		"name":            data.Name.ValueString(),
	})

	// Build flag payload
	flagPayload := map[string]interface{}{
		"@type":   "flipt.core.Flag",
		"key":     data.Key.ValueString(),
		"name":    data.Name.ValueString(),
		"type":    data.Type.ValueString(),
		"enabled": data.Enabled.ValueBool(),
	}

	if !data.Description.IsNull() && !data.Description.IsUnknown() {
		flagPayload["description"] = data.Description.ValueString()
	}

	// Add metadata if provided
	if !data.Metadata.IsNull() && !data.Metadata.IsUnknown() {
		metadataMap := make(map[string]string)
		diags := data.Metadata.ElementsAs(ctx, &metadataMap, false)
		if diags.HasError() {
			resp.Diagnostics.Append(diags...)
			return
		}
		if len(metadataMap) > 0 {
			metadata := make(map[string]interface{})
			for k, v := range metadataMap {
				metadata[k] = v
			}
			flagPayload["metadata"] = metadata
		}
	}

	// Wrap in resources API format
	updateReq := map[string]interface{}{
		"key":     data.Key.ValueString(),
		"payload": flagPayload,
	}

	reqBody, err := json.Marshal(updateReq)
	if err != nil {
		resp.Diagnostics.AddError("Serialization Error", fmt.Sprintf("Unable to marshal request: %s", err))
		return
	}

	// PUT URL doesn't include the flipt.core.Flag prefix
	url := fmt.Sprintf("%s/api/v2/environments/%s/namespaces/%s/resources", r.config.Endpoint, envKey, data.NamespaceKey.ValueString())
	httpReq, err := http.NewRequestWithContext(ctx, "PUT", url, bytes.NewReader(reqBody))
	if err != nil {
		resp.Diagnostics.AddError("Request Error", fmt.Sprintf("Unable to create request: %s", err))
		return
	}
	httpReq.Header.Set("Content-Type", "application/json")
	r.config.AddAuthHeader(httpReq)

	httpResp, err := r.config.HTTPClient.Do(httpReq)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to update flag, got error: %s", err))
		return
	}
	defer httpResp.Body.Close()

	body, err := io.ReadAll(httpResp.Body)
	if err != nil {
		resp.Diagnostics.AddError("Read Error", fmt.Sprintf("Unable to read response: %s", err))
		return
	}

	if httpResp.StatusCode != http.StatusOK {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to update flag, status: %d, body: %s", httpResp.StatusCode, string(body)))
		return
	}

	// Parse response
	var response struct {
		Resource struct {
			NamespaceKey string `json:"namespaceKey"`
			Key          string `json:"key"`
			Payload      struct {
				Type        string                 `json:"type"`
				Key         string                 `json:"key"`
				Name        string                 `json:"name"`
				Description string                 `json:"description"`
				Enabled     bool                   `json:"enabled"`
				Metadata    map[string]interface{} `json:"metadata"`
			} `json:"payload"`
		} `json:"resource"`
		Revision string `json:"revision"`
	}

	if err := json.Unmarshal(body, &response); err != nil {
		resp.Diagnostics.AddError("Parse Error", fmt.Sprintf("Unable to parse response: %s", err))
		return
	}

	flag := response.Resource.Payload

	// Update optional and computed fields
	if flag.Description != "" {
		data.Description = types.StringValue(flag.Description)
	} else {
		data.Description = types.StringNull()
	}

	data.Enabled = types.BoolValue(flag.Enabled)
	data.Type = types.StringValue(flag.Type)

	// Update metadata
	if len(flag.Metadata) > 0 {
		metadataMap := make(map[string]string)
		for k, v := range flag.Metadata {
			metadataMap[k] = fmt.Sprintf("%v", v)
		}
		metadataValue, diags := types.MapValueFrom(ctx, types.StringType, metadataMap)
		if diags.HasError() {
			resp.Diagnostics.Append(diags...)
			return
		}
		data.Metadata = metadataValue
	} else {
		data.Metadata = types.MapNull(types.StringType)
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *FlagResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data FlagResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if data.Key.IsNull() || data.Key.ValueString() == "" {
		resp.Diagnostics.AddError("Missing Flag Key",
			"The flag key is required for deletion but was not found in the state.")
		return
	}

	// Default to "default" environment if not specified
	envKey := "default"
	if !data.EnvironmentKey.IsNull() && !data.EnvironmentKey.IsUnknown() {
		envKey = data.EnvironmentKey.ValueString()
	}

	tflog.Debug(ctx, "Deleting flag", map[string]interface{}{
		"environment_key": envKey,
		"namespace_key":   data.NamespaceKey.ValueString(),
		"key":             data.Key.ValueString(),
	})

	// DELETE URL includes flipt.core.Flag prefix
	url := fmt.Sprintf("%s/api/v2/environments/%s/namespaces/%s/resources/flipt.core.Flag/%s", r.config.Endpoint, envKey, data.NamespaceKey.ValueString(), data.Key.ValueString())
	httpReq, err := http.NewRequestWithContext(ctx, "DELETE", url, nil)
	if err != nil {
		resp.Diagnostics.AddError("Request Error", fmt.Sprintf("Unable to create request: %s", err))
		return
	}
	r.config.AddAuthHeader(httpReq)

	httpResp, err := r.config.HTTPClient.Do(httpReq)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to delete flag, got error: %s", err))
		return
	}
	defer httpResp.Body.Close()

	if httpResp.StatusCode == http.StatusNotFound {
		return
	}

	if httpResp.StatusCode != http.StatusOK && httpResp.StatusCode != http.StatusNoContent {
		body, _ := io.ReadAll(httpResp.Body)
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to delete flag, status: %d, body: %s", httpResp.StatusCode, string(body)))
		return
	}

	tflog.Trace(ctx, "deleted a flag resource")
}

func (r *FlagResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("key"), req, resp)
}
