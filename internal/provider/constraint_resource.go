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

var _ resource.Resource = &ConstraintResource{}
var _ resource.ResourceWithImportState = &ConstraintResource{}

func NewConstraintResource() resource.Resource {
	return &ConstraintResource{}
}

type ConstraintResource struct {
	httpClient *http.Client
	endpoint   string
}

type ConstraintResourceModel struct {
	NamespaceKey   types.String `tfsdk:"namespace_key"`
	EnvironmentKey types.String `tfsdk:"environment_key"`
	SegmentKey     types.String `tfsdk:"segment_key"`
	Property       types.String `tfsdk:"property"`
	Type           types.String `tfsdk:"type"`
	Operator       types.String `tfsdk:"operator"`
	Value          types.String `tfsdk:"value"`
	Description    types.String `tfsdk:"description"`
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
			"environment_key": schema.StringAttribute{
				MarkdownDescription: "Environment key (defaults to 'default' if not specified)",
				Optional:            true,
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
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
			"property": schema.StringAttribute{
				MarkdownDescription: "Property name for the constraint (unique identifier)",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"type": schema.StringAttribute{
				MarkdownDescription: "Constraint type (e.g., STRING_COMPARISON_TYPE)",
				Required:            true,
			},
			"operator": schema.StringAttribute{
				MarkdownDescription: "Comparison operator (e.g., eq, suffix, prefix)",
				Required:            true,
			},
			"value": schema.StringAttribute{
				MarkdownDescription: "Value to compare against",
				Required:            true,
			},
			"description": schema.StringAttribute{
				MarkdownDescription: "Description of the constraint",
				Optional:            true,
			},
		},
	}
}

func (r *ConstraintResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *ConstraintResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data ConstraintResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Determine environment key (default to "default" if not specified)
	envKey := "default"
	if !data.EnvironmentKey.IsNull() && !data.EnvironmentKey.IsUnknown() {
		envKey = data.EnvironmentKey.ValueString()
	}

	tflog.Debug(ctx, "Creating constraint", map[string]interface{}{
		"environment_key": envKey,
		"namespace_key":   data.NamespaceKey.ValueString(),
		"segment_key":     data.SegmentKey.ValueString(),
		"property":        data.Property.ValueString(),
	})

	// First, get the current segment to read existing constraints
	segmentURL := fmt.Sprintf("%s/api/v2/environments/%s/namespaces/%s/resources/flipt.core.Segment/%s",
		r.endpoint, envKey, data.NamespaceKey.ValueString(), data.SegmentKey.ValueString())

	httpReq, err := http.NewRequestWithContext(ctx, "GET", segmentURL, nil)
	if err != nil {
		resp.Diagnostics.AddError("Request Error", fmt.Sprintf("Unable to create request: %s", err))
		return
	}

	httpResp, err := r.httpClient.Do(httpReq)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to read segment, got error: %s", err))
		return
	}
	defer httpResp.Body.Close()

	if httpResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(httpResp.Body)
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to read segment, status: %d, body: %s", httpResp.StatusCode, string(body)))
		return
	}

	var segmentResponse struct {
		Resource struct {
			Payload struct {
				Key         string                   `json:"key"`
				Name        string                   `json:"name"`
				Description string                   `json:"description"`
				MatchType   string                   `json:"matchType"`
				Constraints []map[string]interface{} `json:"constraints"`
			} `json:"payload"`
		} `json:"resource"`
	}

	body, _ := io.ReadAll(httpResp.Body)
	if err := json.Unmarshal(body, &segmentResponse); err != nil {
		resp.Diagnostics.AddError("Parse Error", fmt.Sprintf("Unable to parse segment response: %s", err))
		return
	}

	// Build new constraint
	newConstraint := map[string]interface{}{
		"property": data.Property.ValueString(),
		"type":     data.Type.ValueString(),
		"operator": data.Operator.ValueString(),
		"value":    data.Value.ValueString(),
	}

	if !data.Description.IsNull() && !data.Description.IsUnknown() {
		newConstraint["description"] = data.Description.ValueString()
	} else {
		newConstraint["description"] = ""
	}

	// Add new constraint to existing constraints
	existingConstraints := segmentResponse.Resource.Payload.Constraints
	if existingConstraints == nil {
		existingConstraints = []map[string]interface{}{}
	}
	allConstraints := append(existingConstraints, newConstraint)

	// Update the segment with all constraints (including the new one)
	segmentPayload := map[string]interface{}{
		"@type":       "flipt.core.Segment",
		"key":         segmentResponse.Resource.Payload.Key,
		"name":        segmentResponse.Resource.Payload.Name,
		"description": segmentResponse.Resource.Payload.Description,
		"matchType":   segmentResponse.Resource.Payload.MatchType,
		"constraints": allConstraints,
	}

	updateReq := map[string]interface{}{
		"key":     data.SegmentKey.ValueString(),
		"payload": segmentPayload,
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
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to create constraint, got error: %s", err))
		return
	}
	defer httpResp.Body.Close()

	body, _ = io.ReadAll(httpResp.Body)
	if httpResp.StatusCode != http.StatusOK {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to create constraint, status: %d, body: %s", httpResp.StatusCode, string(body)))
		return
	}

	// Parse response to confirm constraint was created
	if err := json.Unmarshal(body, &segmentResponse); err != nil {
		resp.Diagnostics.AddError("Parse Error", fmt.Sprintf("Unable to parse response: %s", err))
		return
	}

	// State is already set from plan
	tflog.Trace(ctx, "created a constraint resource")
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *ConstraintResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data ConstraintResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Determine environment key (default to "default" if not specified)
	envKey := "default"
	if !data.EnvironmentKey.IsNull() && !data.EnvironmentKey.IsUnknown() {
		envKey = data.EnvironmentKey.ValueString()
	}

	tflog.Debug(ctx, "Reading constraint", map[string]interface{}{
		"environment_key": envKey,
		"namespace_key":   data.NamespaceKey.ValueString(),
		"segment_key":     data.SegmentKey.ValueString(),
		"property":        data.Property.ValueString(),
	})

	// Get the segment to read its constraints
	url := fmt.Sprintf("%s/api/v2/environments/%s/namespaces/%s/resources/flipt.core.Segment/%s",
		r.endpoint, envKey, data.NamespaceKey.ValueString(), data.SegmentKey.ValueString())

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
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to read segment, status: %d, body: %s", httpResp.StatusCode, string(body)))
		return
	}

	var segmentResponse struct {
		Resource struct {
			Payload struct {
				Constraints []struct {
					Property    string `json:"property"`
					Type        string `json:"type"`
					Operator    string `json:"operator"`
					Value       string `json:"value"`
					Description string `json:"description"`
				} `json:"constraints"`
			} `json:"payload"`
		} `json:"resource"`
	}

	if err := json.Unmarshal(body, &segmentResponse); err != nil {
		resp.Diagnostics.AddError("Parse Error", fmt.Sprintf("Unable to parse response: %s", err))
		return
	}

	// Find the constraint by property
	var found bool
	for _, c := range segmentResponse.Resource.Payload.Constraints {
		if c.Property == data.Property.ValueString() {
			found = true

			data.Type = types.StringValue(c.Type)
			data.Operator = types.StringValue(c.Operator)
			data.Value = types.StringValue(c.Value)

			if c.Description != "" {
				data.Description = types.StringValue(c.Description)
			} else {
				data.Description = types.StringNull()
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

func (r *ConstraintResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data ConstraintResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Determine environment key (default to "default" if not specified)
	envKey := "default"
	if !data.EnvironmentKey.IsNull() && !data.EnvironmentKey.IsUnknown() {
		envKey = data.EnvironmentKey.ValueString()
	}

	tflog.Debug(ctx, "Updating constraint", map[string]interface{}{
		"environment_key": envKey,
		"namespace_key":   data.NamespaceKey.ValueString(),
		"segment_key":     data.SegmentKey.ValueString(),
		"property":        data.Property.ValueString(),
	})

	// Get the current segment to read existing constraints
	segmentURL := fmt.Sprintf("%s/api/v2/environments/%s/namespaces/%s/resources/flipt.core.Segment/%s",
		r.endpoint, envKey, data.NamespaceKey.ValueString(), data.SegmentKey.ValueString())

	httpReq, err := http.NewRequestWithContext(ctx, "GET", segmentURL, nil)
	if err != nil {
		resp.Diagnostics.AddError("Request Error", fmt.Sprintf("Unable to create request: %s", err))
		return
	}

	httpResp, err := r.httpClient.Do(httpReq)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to read segment, got error: %s", err))
		return
	}
	defer httpResp.Body.Close()

	if httpResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(httpResp.Body)
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to read segment, status: %d, body: %s", httpResp.StatusCode, string(body)))
		return
	}

	var segmentResponse struct {
		Resource struct {
			Payload struct {
				Key         string                   `json:"key"`
				Name        string                   `json:"name"`
				Description string                   `json:"description"`
				MatchType   string                   `json:"matchType"`
				Constraints []map[string]interface{} `json:"constraints"`
			} `json:"payload"`
		} `json:"resource"`
	}

	body, _ := io.ReadAll(httpResp.Body)
	if err := json.Unmarshal(body, &segmentResponse); err != nil {
		resp.Diagnostics.AddError("Parse Error", fmt.Sprintf("Unable to parse segment response: %s", err))
		return
	}

	// Find and update the constraint in the constraints array
	var found bool
	existingConstraints := segmentResponse.Resource.Payload.Constraints
	if existingConstraints == nil {
		existingConstraints = []map[string]interface{}{}
	}

	for i, c := range existingConstraints {
		if prop, ok := c["property"].(string); ok && prop == data.Property.ValueString() {
			found = true
			// Update the constraint
			existingConstraints[i] = map[string]interface{}{
				"property": data.Property.ValueString(),
				"type":     data.Type.ValueString(),
				"operator": data.Operator.ValueString(),
				"value":    data.Value.ValueString(),
			}

			if !data.Description.IsNull() && !data.Description.IsUnknown() {
				existingConstraints[i]["description"] = data.Description.ValueString()
			} else {
				existingConstraints[i]["description"] = ""
			}
			break
		}
	}

	if !found {
		resp.Diagnostics.AddError("Not Found", fmt.Sprintf("Constraint with property %s not found in segment", data.Property.ValueString()))
		return
	}

	// Update the segment with all constraints (including the modified one)
	segmentPayload := map[string]interface{}{
		"@type":       "flipt.core.Segment",
		"key":         segmentResponse.Resource.Payload.Key,
		"name":        segmentResponse.Resource.Payload.Name,
		"description": segmentResponse.Resource.Payload.Description,
		"matchType":   segmentResponse.Resource.Payload.MatchType,
		"constraints": existingConstraints,
	}

	updateReq := map[string]interface{}{
		"key":     data.SegmentKey.ValueString(),
		"payload": segmentPayload,
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
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to update constraint, got error: %s", err))
		return
	}
	defer httpResp.Body.Close()

	body, _ = io.ReadAll(httpResp.Body)
	if httpResp.StatusCode != http.StatusOK {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to update constraint, status: %d, body: %s", httpResp.StatusCode, string(body)))
		return
	}

	// State is already set from plan
	tflog.Trace(ctx, "updated a constraint resource")
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *ConstraintResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data ConstraintResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Determine environment key (default to "default" if not specified)
	envKey := "default"
	if !data.EnvironmentKey.IsNull() && !data.EnvironmentKey.IsUnknown() {
		envKey = data.EnvironmentKey.ValueString()
	}

	tflog.Debug(ctx, "Deleting constraint", map[string]interface{}{
		"environment_key": envKey,
		"namespace_key":   data.NamespaceKey.ValueString(),
		"segment_key":     data.SegmentKey.ValueString(),
		"property":        data.Property.ValueString(),
	})

	// Get the current segment to read existing constraints
	segmentURL := fmt.Sprintf("%s/api/v2/environments/%s/namespaces/%s/resources/flipt.core.Segment/%s",
		r.endpoint, envKey, data.NamespaceKey.ValueString(), data.SegmentKey.ValueString())

	httpReq, err := http.NewRequestWithContext(ctx, "GET", segmentURL, nil)
	if err != nil {
		resp.Diagnostics.AddError("Request Error", fmt.Sprintf("Unable to create request: %s", err))
		return
	}

	httpResp, err := r.httpClient.Do(httpReq)
	if err != nil {
		// If segment doesn't exist, constraint is already gone
		return
	}
	defer httpResp.Body.Close()

	if httpResp.StatusCode == http.StatusNotFound {
		// Segment doesn't exist, constraint is already gone
		return
	}

	if httpResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(httpResp.Body)
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to read segment, status: %d, body: %s", httpResp.StatusCode, string(body)))
		return
	}

	var segmentResponse struct {
		Resource struct {
			Payload struct {
				Key         string                   `json:"key"`
				Name        string                   `json:"name"`
				Description string                   `json:"description"`
				MatchType   string                   `json:"matchType"`
				Constraints []map[string]interface{} `json:"constraints"`
			} `json:"payload"`
		} `json:"resource"`
	}

	body, _ := io.ReadAll(httpResp.Body)
	if err := json.Unmarshal(body, &segmentResponse); err != nil {
		resp.Diagnostics.AddError("Parse Error", fmt.Sprintf("Unable to parse segment response: %s", err))
		return
	}

	// Remove the constraint from the constraints array
	existingConstraints := segmentResponse.Resource.Payload.Constraints
	if existingConstraints == nil {
		// No constraints, already deleted
		return
	}

	var updatedConstraints []map[string]interface{}
	for _, c := range existingConstraints {
		if prop, ok := c["property"].(string); ok && prop != data.Property.ValueString() {
			updatedConstraints = append(updatedConstraints, c)
		}
	}

	// Update the segment without the deleted constraint
	segmentPayload := map[string]interface{}{
		"@type":       "flipt.core.Segment",
		"key":         segmentResponse.Resource.Payload.Key,
		"name":        segmentResponse.Resource.Payload.Name,
		"description": segmentResponse.Resource.Payload.Description,
		"matchType":   segmentResponse.Resource.Payload.MatchType,
		"constraints": updatedConstraints,
	}

	updateReq := map[string]interface{}{
		"key":     data.SegmentKey.ValueString(),
		"payload": segmentPayload,
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
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to delete constraint, got error: %s", err))
		return
	}
	defer httpResp.Body.Close()

	body, _ = io.ReadAll(httpResp.Body)
	if httpResp.StatusCode != http.StatusOK {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to delete constraint, status: %d, body: %s", httpResp.StatusCode, string(body)))
		return
	}

	tflog.Trace(ctx, "deleted a constraint resource")
}

func (r *ConstraintResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
