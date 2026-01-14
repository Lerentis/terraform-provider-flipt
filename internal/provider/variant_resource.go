// Copyright (c) terraform-flipt contributors
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

var _ resource.Resource = &VariantResource{}
var _ resource.ResourceWithImportState = &VariantResource{}

func NewVariantResource() resource.Resource {
	return &VariantResource{}
}

type VariantResource struct {
	httpClient *http.Client
	endpoint   string
}

type VariantResourceModel struct {
	NamespaceKey   types.String `tfsdk:"namespace_key"`
	EnvironmentKey types.String `tfsdk:"environment_key"`
	FlagKey        types.String `tfsdk:"flag_key"`
	Key            types.String `tfsdk:"key"`
	Name           types.String `tfsdk:"name"`
	Description    types.String `tfsdk:"description"`
	Attachment     types.String `tfsdk:"attachment"`
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
			"environment_key": schema.StringAttribute{
				MarkdownDescription: "Environment key (defaults to 'default' if not specified)",
				Optional:            true,
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
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
			"key": schema.StringAttribute{
				MarkdownDescription: "Unique key for the variant",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "Display name of the variant",
				Optional:            true,
			},
			"description": schema.StringAttribute{
				MarkdownDescription: "Description of the variant",
				Optional:            true,
			},
			"attachment": schema.StringAttribute{
				MarkdownDescription: "JSON attachment data for the variant",
				Optional:            true,
			},
		},
	}
}

func (r *VariantResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

	r.httpClient = providerConfig.HTTPClient
	r.endpoint = providerConfig.Endpoint
}

func (r *VariantResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data VariantResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Determine environment key (default to "default" if not specified)
	envKey := "default"
	if !data.EnvironmentKey.IsNull() && !data.EnvironmentKey.IsUnknown() {
		envKey = data.EnvironmentKey.ValueString()
	}

	tflog.Debug(ctx, "Creating variant", map[string]interface{}{
		"environment_key": envKey,
		"namespace_key":   data.NamespaceKey.ValueString(),
		"flag_key":        data.FlagKey.ValueString(),
		"variant_key":     data.Key.ValueString(),
	})

	// First, get the current flag to read existing variants
	flagURL := fmt.Sprintf("%s/api/v2/environments/%s/namespaces/%s/resources/flipt.core.Flag/%s",
		r.endpoint, envKey, data.NamespaceKey.ValueString(), data.FlagKey.ValueString())

	httpReq, err := http.NewRequestWithContext(ctx, "GET", flagURL, nil)
	if err != nil {
		resp.Diagnostics.AddError("Request Error", fmt.Sprintf("Unable to create request: %s", err))
		return
	}

	httpResp, err := r.httpClient.Do(httpReq)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to read flag, got error: %s", err))
		return
	}
	defer httpResp.Body.Close()

	if httpResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(httpResp.Body)
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to read flag, status: %d, body: %s", httpResp.StatusCode, string(body)))
		return
	}

	var flagResponse struct {
		Resource struct {
			Payload struct {
				Type        string                   `json:"type"`
				Key         string                   `json:"key"`
				Name        string                   `json:"name"`
				Description string                   `json:"description"`
				Enabled     bool                     `json:"enabled"`
				Variants    []map[string]interface{} `json:"variants"`
				Metadata    map[string]interface{}   `json:"metadata"`
			} `json:"payload"`
		} `json:"resource"`
	}

	body, _ := io.ReadAll(httpResp.Body)
	if err := json.Unmarshal(body, &flagResponse); err != nil {
		resp.Diagnostics.AddError("Parse Error", fmt.Sprintf("Unable to parse flag response: %s", err))
		return
	}

	// Build new variant
	newVariant := map[string]interface{}{
		"key": data.Key.ValueString(),
	}

	if !data.Name.IsNull() && !data.Name.IsUnknown() {
		newVariant["name"] = data.Name.ValueString()
	} else {
		newVariant["name"] = ""
	}

	if !data.Description.IsNull() && !data.Description.IsUnknown() {
		newVariant["description"] = data.Description.ValueString()
	} else {
		newVariant["description"] = ""
	}

	if !data.Attachment.IsNull() && !data.Attachment.IsUnknown() {
		// Parse the attachment JSON string into a map
		var attachmentData map[string]interface{}
		if err := json.Unmarshal([]byte(data.Attachment.ValueString()), &attachmentData); err != nil {
			resp.Diagnostics.AddError("Invalid Attachment", fmt.Sprintf("Attachment must be valid JSON: %s", err))
			return
		}
		newVariant["attachment"] = attachmentData
	} else {
		newVariant["attachment"] = map[string]interface{}{}
	}

	// Add new variant to existing variants
	existingVariants := flagResponse.Resource.Payload.Variants
	if existingVariants == nil {
		existingVariants = []map[string]interface{}{}
	}
	allVariants := append(existingVariants, newVariant)

	// Update the flag with all variants (including the new one)
	flagPayload := map[string]interface{}{
		"@type":       "flipt.core.Flag",
		"key":         flagResponse.Resource.Payload.Key,
		"name":        flagResponse.Resource.Payload.Name,
		"description": flagResponse.Resource.Payload.Description,
		"type":        flagResponse.Resource.Payload.Type,
		"enabled":     flagResponse.Resource.Payload.Enabled,
		"variants":    allVariants,
		"rules":       []interface{}{},
		"metadata":    flagResponse.Resource.Payload.Metadata,
	}

	updateReq := map[string]interface{}{
		"key":     data.FlagKey.ValueString(),
		"payload": flagPayload,
	}

	reqBody, err := json.Marshal(updateReq)
	if err != nil {
		resp.Diagnostics.AddError("Serialization Error", fmt.Sprintf("Unable to marshal request: %s", err))
		return
	}

	updateURL := fmt.Sprintf("%s/api/v2/environments/%s/namespaces/%s/resources", r.endpoint, envKey, data.NamespaceKey.ValueString())
	httpReq, err = http.NewRequestWithContext(ctx, "PUT", updateURL, bytes.NewReader(reqBody))
	if err != nil {
		resp.Diagnostics.AddError("Request Error", fmt.Sprintf("Unable to create request: %s", err))
		return
	}
	httpReq.Header.Set("Content-Type", "application/json")

	httpResp, err = r.httpClient.Do(httpReq)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to create variant, got error: %s", err))
		return
	}
	defer httpResp.Body.Close()

	body, _ = io.ReadAll(httpResp.Body)
	if httpResp.StatusCode != http.StatusOK {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to create variant, status: %d, body: %s", httpResp.StatusCode, string(body)))
		return
	}

	// Parse response to confirm variant was created
	if err := json.Unmarshal(body, &flagResponse); err != nil {
		resp.Diagnostics.AddError("Parse Error", fmt.Sprintf("Unable to parse response: %s", err))
		return
	}

	// State is already set from plan, no need to update
	tflog.Trace(ctx, "created a variant resource")
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *VariantResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data VariantResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Determine environment key (default to "default" if not specified)
	envKey := "default"
	if !data.EnvironmentKey.IsNull() && !data.EnvironmentKey.IsUnknown() {
		envKey = data.EnvironmentKey.ValueString()
	}

	tflog.Debug(ctx, "Reading variant", map[string]interface{}{
		"environment_key": envKey,
		"namespace_key":   data.NamespaceKey.ValueString(),
		"flag_key":        data.FlagKey.ValueString(),
		"variant_key":     data.Key.ValueString(),
	})

	// Get the flag to read its variants
	url := fmt.Sprintf("%s/api/v2/environments/%s/namespaces/%s/resources/flipt.core.Flag/%s",
		r.endpoint, envKey, data.NamespaceKey.ValueString(), data.FlagKey.ValueString())

	httpReq, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		resp.Diagnostics.AddError("Request Error", fmt.Sprintf("Unable to create request: %s", err))
		return
	}

	httpResp, err := r.httpClient.Do(httpReq)
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

	var flagResponse struct {
		Resource struct {
			Payload struct {
				Variants []struct {
					Key         string                 `json:"key"`
					Name        string                 `json:"name"`
					Description string                 `json:"description"`
					Attachment  map[string]interface{} `json:"attachment"`
				} `json:"variants"`
			} `json:"payload"`
		} `json:"resource"`
	}

	if err := json.Unmarshal(body, &flagResponse); err != nil {
		resp.Diagnostics.AddError("Parse Error", fmt.Sprintf("Unable to parse response: %s", err))
		return
	}

	// Find the variant by key
	var found bool
	for _, v := range flagResponse.Resource.Payload.Variants {
		if v.Key == data.Key.ValueString() {
			found = true

			if v.Name != "" {
				data.Name = types.StringValue(v.Name)
			} else {
				data.Name = types.StringNull()
			}

			if v.Description != "" {
				data.Description = types.StringValue(v.Description)
			} else {
				data.Description = types.StringNull()
			}

			if len(v.Attachment) > 0 {
				attachmentJSON, err := json.Marshal(v.Attachment)
				if err == nil {
					data.Attachment = types.StringValue(string(attachmentJSON))
				} else {
					data.Attachment = types.StringNull()
				}
			} else {
				data.Attachment = types.StringNull()
			}
			break
		}
	}

	if !found {
		resp.State.RemoveResource(ctx)
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *VariantResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data VariantResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Determine environment key (default to "default" if not specified)
	envKey := "default"
	if !data.EnvironmentKey.IsNull() && !data.EnvironmentKey.IsUnknown() {
		envKey = data.EnvironmentKey.ValueString()
	}

	tflog.Debug(ctx, "Updating variant", map[string]interface{}{
		"environment_key": envKey,
		"namespace_key":   data.NamespaceKey.ValueString(),
		"flag_key":        data.FlagKey.ValueString(),
		"variant_key":     data.Key.ValueString(),
	})

	// Get the current flag to read existing variants
	flagURL := fmt.Sprintf("%s/api/v2/environments/%s/namespaces/%s/resources/flipt.core.Flag/%s",
		r.endpoint, envKey, data.NamespaceKey.ValueString(), data.FlagKey.ValueString())

	httpReq, err := http.NewRequestWithContext(ctx, "GET", flagURL, nil)
	if err != nil {
		resp.Diagnostics.AddError("Request Error", fmt.Sprintf("Unable to create request: %s", err))
		return
	}

	httpResp, err := r.httpClient.Do(httpReq)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to read flag, got error: %s", err))
		return
	}
	defer httpResp.Body.Close()

	body, _ := io.ReadAll(httpResp.Body)
	if httpResp.StatusCode != http.StatusOK {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to read flag, status: %d, body: %s", httpResp.StatusCode, string(body)))
		return
	}

	var flagResponse struct {
		Resource struct {
			Payload struct {
				Type        string                   `json:"type"`
				Key         string                   `json:"key"`
				Name        string                   `json:"name"`
				Description string                   `json:"description"`
				Enabled     bool                     `json:"enabled"`
				Variants    []map[string]interface{} `json:"variants"`
				Metadata    map[string]interface{}   `json:"metadata"`
			} `json:"payload"`
		} `json:"resource"`
	}

	if err := json.Unmarshal(body, &flagResponse); err != nil {
		resp.Diagnostics.AddError("Parse Error", fmt.Sprintf("Unable to parse flag response: %s", err))
		return
	}

	// Update the variant in the variants list
	updatedVariants := make([]map[string]interface{}, 0)
	found := false
	for _, v := range flagResponse.Resource.Payload.Variants {
		if vKey, ok := v["key"].(string); ok && vKey == data.Key.ValueString() {
			found = true
			updatedVariant := map[string]interface{}{
				"key": data.Key.ValueString(),
			}

			if !data.Name.IsNull() && !data.Name.IsUnknown() {
				updatedVariant["name"] = data.Name.ValueString()
			} else {
				updatedVariant["name"] = ""
			}

			if !data.Description.IsNull() && !data.Description.IsUnknown() {
				updatedVariant["description"] = data.Description.ValueString()
			} else {
				updatedVariant["description"] = ""
			}

			if !data.Attachment.IsNull() && !data.Attachment.IsUnknown() {
				var attachmentData map[string]interface{}
				if err := json.Unmarshal([]byte(data.Attachment.ValueString()), &attachmentData); err != nil {
					resp.Diagnostics.AddError("Invalid Attachment", fmt.Sprintf("Attachment must be valid JSON: %s", err))
					return
				}
				updatedVariant["attachment"] = attachmentData
			} else {
				updatedVariant["attachment"] = map[string]interface{}{}
			}

			updatedVariants = append(updatedVariants, updatedVariant)
		} else {
			updatedVariants = append(updatedVariants, v)
		}
	}

	if !found {
		resp.Diagnostics.AddError("Variant Not Found", fmt.Sprintf("Variant with key '%s' not found in flag", data.Key.ValueString()))
		return
	}

	// Update the flag with modified variants
	flagPayload := map[string]interface{}{
		"@type":       "flipt.core.Flag",
		"key":         flagResponse.Resource.Payload.Key,
		"name":        flagResponse.Resource.Payload.Name,
		"description": flagResponse.Resource.Payload.Description,
		"type":        flagResponse.Resource.Payload.Type,
		"enabled":     flagResponse.Resource.Payload.Enabled,
		"variants":    updatedVariants,
		"rules":       []interface{}{},
		"metadata":    flagResponse.Resource.Payload.Metadata,
	}

	updateReq := map[string]interface{}{
		"key":     data.FlagKey.ValueString(),
		"payload": flagPayload,
	}

	reqBody, err := json.Marshal(updateReq)
	if err != nil {
		resp.Diagnostics.AddError("Serialization Error", fmt.Sprintf("Unable to marshal request: %s", err))
		return
	}

	updateURL := fmt.Sprintf("%s/api/v2/environments/%s/namespaces/%s/resources", r.endpoint, envKey, data.NamespaceKey.ValueString())
	httpReq, err = http.NewRequestWithContext(ctx, "PUT", updateURL, bytes.NewReader(reqBody))
	if err != nil {
		resp.Diagnostics.AddError("Request Error", fmt.Sprintf("Unable to create request: %s", err))
		return
	}
	httpReq.Header.Set("Content-Type", "application/json")

	httpResp, err = r.httpClient.Do(httpReq)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to update variant, got error: %s", err))
		return
	}
	defer httpResp.Body.Close()

	body, _ = io.ReadAll(httpResp.Body)
	if httpResp.StatusCode != http.StatusOK {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to update variant, status: %d, body: %s", httpResp.StatusCode, string(body)))
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *VariantResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data VariantResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Determine environment key (default to "default" if not specified)
	envKey := "default"
	if !data.EnvironmentKey.IsNull() && !data.EnvironmentKey.IsUnknown() {
		envKey = data.EnvironmentKey.ValueString()
	}

	tflog.Debug(ctx, "Deleting variant", map[string]interface{}{
		"environment_key": envKey,
		"namespace_key":   data.NamespaceKey.ValueString(),
		"flag_key":        data.FlagKey.ValueString(),
		"variant_key":     data.Key.ValueString(),
	})

	// Get the current flag to read existing variants
	flagURL := fmt.Sprintf("%s/api/v2/environments/%s/namespaces/%s/resources/flipt.core.Flag/%s",
		r.endpoint, envKey, data.NamespaceKey.ValueString(), data.FlagKey.ValueString())

	httpReq, err := http.NewRequestWithContext(ctx, "GET", flagURL, nil)
	if err != nil {
		resp.Diagnostics.AddError("Request Error", fmt.Sprintf("Unable to create request: %s", err))
		return
	}

	httpResp, err := r.httpClient.Do(httpReq)
	if err != nil {
		// If flag doesn't exist, variant is already gone
		return
	}
	defer httpResp.Body.Close()

	if httpResp.StatusCode == http.StatusNotFound {
		// Flag doesn't exist, so variant is gone
		return
	}

	body, _ := io.ReadAll(httpResp.Body)
	if httpResp.StatusCode != http.StatusOK {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to read flag, status: %d, body: %s", httpResp.StatusCode, string(body)))
		return
	}

	var flagResponse struct {
		Resource struct {
			Payload struct {
				Type        string                   `json:"type"`
				Key         string                   `json:"key"`
				Name        string                   `json:"name"`
				Description string                   `json:"description"`
				Enabled     bool                     `json:"enabled"`
				Variants    []map[string]interface{} `json:"variants"`
				Metadata    map[string]interface{}   `json:"metadata"`
			} `json:"payload"`
		} `json:"resource"`
	}

	if err := json.Unmarshal(body, &flagResponse); err != nil {
		resp.Diagnostics.AddError("Parse Error", fmt.Sprintf("Unable to parse flag response: %s", err))
		return
	}

	// Remove the variant from the variants list
	remainingVariants := make([]map[string]interface{}, 0)
	for _, v := range flagResponse.Resource.Payload.Variants {
		if vKey, ok := v["key"].(string); !ok || vKey != data.Key.ValueString() {
			remainingVariants = append(remainingVariants, v)
		}
	}

	// Update the flag with remaining variants (excluding the deleted one)
	flagPayload := map[string]interface{}{
		"@type":       "flipt.core.Flag",
		"key":         flagResponse.Resource.Payload.Key,
		"name":        flagResponse.Resource.Payload.Name,
		"description": flagResponse.Resource.Payload.Description,
		"type":        flagResponse.Resource.Payload.Type,
		"enabled":     flagResponse.Resource.Payload.Enabled,
		"variants":    remainingVariants,
		"rules":       []interface{}{},
		"metadata":    flagResponse.Resource.Payload.Metadata,
	}

	updateReq := map[string]interface{}{
		"key":     data.FlagKey.ValueString(),
		"payload": flagPayload,
	}

	reqBody, err := json.Marshal(updateReq)
	if err != nil {
		resp.Diagnostics.AddError("Serialization Error", fmt.Sprintf("Unable to marshal request: %s", err))
		return
	}

	updateURL := fmt.Sprintf("%s/api/v2/environments/%s/namespaces/%s/resources", r.endpoint, envKey, data.NamespaceKey.ValueString())
	httpReq, err = http.NewRequestWithContext(ctx, "PUT", updateURL, bytes.NewReader(reqBody))
	if err != nil {
		resp.Diagnostics.AddError("Request Error", fmt.Sprintf("Unable to create request: %s", err))
		return
	}
	httpReq.Header.Set("Content-Type", "application/json")

	httpResp, err = r.httpClient.Do(httpReq)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to delete variant, got error: %s", err))
		return
	}
	defer httpResp.Body.Close()

	body, _ = io.ReadAll(httpResp.Body)
	if httpResp.StatusCode != http.StatusOK {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to delete variant, status: %d, body: %s", httpResp.StatusCode, string(body)))
		return
	}

	tflog.Trace(ctx, "deleted a variant resource")
}

func (r *VariantResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("key"), req, resp)
}
