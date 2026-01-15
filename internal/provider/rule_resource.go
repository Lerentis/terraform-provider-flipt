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

	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

var _ resource.Resource = &RuleResource{}
var _ resource.ResourceWithImportState = &RuleResource{}

type RuleResource struct {
	httpClient *http.Client
	endpoint   string
}

func NewRuleResource() resource.Resource {
	return &RuleResource{}
}

type RuleResourceModel struct {
	NamespaceKey    types.String `tfsdk:"namespace_key"`
	EnvironmentKey  types.String `tfsdk:"environment_key"`
	FlagKey         types.String `tfsdk:"flag_key"`
	ID              types.String `tfsdk:"id"`
	SegmentKeys     types.List   `tfsdk:"segment_keys"`
	SegmentOperator types.String `tfsdk:"segment_operator"`
	Rank            types.Int64  `tfsdk:"rank"`
}

func (r *RuleResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_rule"
}

func (r *RuleResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Flipt rule resource (belongs to a flag)",

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
				MarkdownDescription: "Flag key that this rule belongs to",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"id": schema.StringAttribute{
				MarkdownDescription: "Unique identifier for the rule (auto-generated)",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"segment_keys": schema.ListAttribute{
				ElementType:         types.StringType,
				MarkdownDescription: "List of segment keys to evaluate for this rule",
				Required:            true,
			},
			"segment_operator": schema.StringAttribute{
				MarkdownDescription: "Operator for combining segments (OR_SEGMENT_OPERATOR or AND_SEGMENT_OPERATOR)",
				Optional:            true,
				Computed:            true,
			},
			"rank": schema.Int64Attribute{
				MarkdownDescription: "Rank/order of the rule (lower ranks are evaluated first)",
				Optional:            true,
				Computed:            true,
			},
		},
	}
}

func (r *RuleResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *RuleResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data RuleResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Determine environment key (default to "default" if not specified)
	envKey := "default"
	if !data.EnvironmentKey.IsNull() && !data.EnvironmentKey.IsUnknown() {
		envKey = data.EnvironmentKey.ValueString()
	}

	tflog.Debug(ctx, "Creating rule", map[string]interface{}{
		"environment_key": envKey,
		"namespace_key":   data.NamespaceKey.ValueString(),
		"flag_key":        data.FlagKey.ValueString(),
	})

	// First, get the current flag to read existing rules
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
				Type           string                   `json:"type"`
				Key            string                   `json:"key"`
				Name           string                   `json:"name"`
				Description    string                   `json:"description"`
				Enabled        bool                     `json:"enabled"`
				Variants       []map[string]interface{} `json:"variants"`
				Rules          []map[string]interface{} `json:"rules"`
				DefaultVariant string                   `json:"defaultVariant"`
				Metadata       map[string]interface{}   `json:"metadata"`
			} `json:"payload"`
		} `json:"resource"`
	}

	body, _ := io.ReadAll(httpResp.Body)
	if err := json.Unmarshal(body, &flagResponse); err != nil {
		resp.Diagnostics.AddError("Parse Error", fmt.Sprintf("Unable to parse flag response: %s", err))
		return
	}

	// Extract segment keys from plan
	var segmentKeys []string
	resp.Diagnostics.Append(data.SegmentKeys.ElementsAs(ctx, &segmentKeys, false)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Generate ID for new rule
	ruleID := uuid.New().String()

	// Set defaults
	segmentOperator := "OR_SEGMENT_OPERATOR"
	if !data.SegmentOperator.IsNull() && !data.SegmentOperator.IsUnknown() {
		segmentOperator = data.SegmentOperator.ValueString()
	}

	rank := int64(0)
	if !data.Rank.IsNull() && !data.Rank.IsUnknown() {
		rank = data.Rank.ValueInt64()
	} else {
		// Auto-assign rank as next available
		rank = int64(len(flagResponse.Resource.Payload.Rules))
	}

	// Build new rule
	newRule := map[string]interface{}{
		"id":              ruleID,
		"segments":        segmentKeys,
		"segmentOperator": segmentOperator,
		"rank":            rank,
		"distributions":   []interface{}{}, // Empty distributions array
	}

	// Add new rule to existing rules
	existingRules := flagResponse.Resource.Payload.Rules
	if existingRules == nil {
		existingRules = []map[string]interface{}{}
	}
	allRules := append(existingRules, newRule)

	// Update the flag with all rules (including the new one)
	flagPayload := map[string]interface{}{
		"@type":          "flipt.core.Flag",
		"key":            flagResponse.Resource.Payload.Key,
		"name":           flagResponse.Resource.Payload.Name,
		"description":    flagResponse.Resource.Payload.Description,
		"type":           flagResponse.Resource.Payload.Type,
		"enabled":        flagResponse.Resource.Payload.Enabled,
		"variants":       flagResponse.Resource.Payload.Variants,
		"rules":          allRules,
		"defaultVariant": flagResponse.Resource.Payload.DefaultVariant,
		"metadata":       flagResponse.Resource.Payload.Metadata,
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
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to create rule, got error: %s", err))
		return
	}
	defer httpResp.Body.Close()

	body, _ = io.ReadAll(httpResp.Body)
	if httpResp.StatusCode != http.StatusOK {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to create rule, status: %d, body: %s", httpResp.StatusCode, string(body)))
		return
	}

	// Set computed values
	data.EnvironmentKey = types.StringValue(envKey)
	// Generate a stable ID based on flag_key and rank (rank is more stable than operator)
	ruleID = fmt.Sprintf("%s/%d", data.FlagKey.ValueString(), rank)
	data.ID = types.StringValue(ruleID)
	data.SegmentOperator = types.StringValue(segmentOperator)
	data.Rank = types.Int64Value(rank)

	tflog.Trace(ctx, "created a rule resource")
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *RuleResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data RuleResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Determine environment key (default to "default" if not specified)
	envKey := "default"
	if !data.EnvironmentKey.IsNull() && !data.EnvironmentKey.IsUnknown() {
		envKey = data.EnvironmentKey.ValueString()
	}

	tflog.Debug(ctx, "Reading rule", map[string]interface{}{
		"environment_key": envKey,
		"namespace_key":   data.NamespaceKey.ValueString(),
		"flag_key":        data.FlagKey.ValueString(),
		"rule_id":         data.ID.ValueString(),
	})

	// Get the flag to read its rules
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
				Rules []struct {
					ID              string   `json:"id"`
					Segments        []string `json:"segments"`
					SegmentOperator string   `json:"segmentOperator"`
					Rank            int64    `json:"rank"`
				} `json:"rules"`
			} `json:"payload"`
		} `json:"resource"`
	}

	if err := json.Unmarshal(body, &flagResponse); err != nil {
		resp.Diagnostics.AddError("Parse Error", fmt.Sprintf("Unable to parse response: %s", err))
		return
	}

	tflog.Debug(ctx, "Flag response received", map[string]interface{}{
		"rules_count":    len(flagResponse.Resource.Payload.Rules),
		"looking_for_id": data.ID.ValueString(),
	})

	// Find the rule by matching segments, operator, and rank since Flipt doesn't preserve rule IDs
	var found bool
	for _, rule := range flagResponse.Resource.Payload.Rules {
		tflog.Debug(ctx, "Checking rule", map[string]interface{}{
			"rule_id":           rule.ID,
			"rule_segments":     rule.Segments,
			"rule_operator":     rule.SegmentOperator,
			"rule_rank":         rule.Rank,
			"expected_operator": data.SegmentOperator.ValueString(),
			"expected_rank":     data.Rank.ValueInt64(),
		})

		// Match by segments, operator, and rank since Flipt doesn't preserve IDs
		var expectedSegments []string
		resp.Diagnostics.Append(data.SegmentKeys.ElementsAs(ctx, &expectedSegments, false)...)
		if resp.Diagnostics.HasError() {
			return
		}

		// Check if segments match
		segmentsMatch := len(rule.Segments) == len(expectedSegments)
		if segmentsMatch {
			for i, seg := range rule.Segments {
				if i >= len(expectedSegments) || seg != expectedSegments[i] {
					segmentsMatch = false
					break
				}
			}
		}

		if segmentsMatch &&
			rule.SegmentOperator == data.SegmentOperator.ValueString() &&
			rule.Rank == data.Rank.ValueInt64() {
			found = true

			// Convert segments to types.List
			segmentsList, diags := types.ListValueFrom(ctx, types.StringType, rule.Segments)
			resp.Diagnostics.Append(diags...)
			if resp.Diagnostics.HasError() {
				return
			}
			data.SegmentKeys = segmentsList

			data.SegmentOperator = types.StringValue(rule.SegmentOperator)
			data.Rank = types.Int64Value(rule.Rank)

			// Generate a stable ID based on rule attributes if not already set
			if data.ID.IsNull() || data.ID.ValueString() == "" {
				ruleID := fmt.Sprintf("%s/%d", data.FlagKey.ValueString(), rule.Rank)
				data.ID = types.StringValue(ruleID)
			}
			break
		}
	}

	if !found {
		tflog.Warn(ctx, "Rule not found in flag, removing from state", map[string]interface{}{
			"rule_id":  data.ID.ValueString(),
			"flag_key": data.FlagKey.ValueString(),
		})
		resp.State.RemoveResource(ctx)
		return
	}

	// Ensure EnvironmentKey is set in state
	data.EnvironmentKey = types.StringValue(envKey)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *RuleResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data RuleResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Get the current state to know which rule to update
	var state RuleResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Determine environment key (default to "default" if not specified)
	envKey := "default"
	if !data.EnvironmentKey.IsNull() && !data.EnvironmentKey.IsUnknown() {
		envKey = data.EnvironmentKey.ValueString()
	}

	tflog.Debug(ctx, "Updating rule", map[string]interface{}{
		"environment_key": envKey,
		"namespace_key":   data.NamespaceKey.ValueString(),
		"flag_key":        data.FlagKey.ValueString(),
		"old_state_id":    state.ID.ValueString(),
		"new_plan_values": fmt.Sprintf("operator=%s rank=%d", data.SegmentOperator.ValueString(), data.Rank.ValueInt64()),
	})

	// Get the current flag to read existing rules
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
				Type           string                   `json:"type"`
				Key            string                   `json:"key"`
				Name           string                   `json:"name"`
				Description    string                   `json:"description"`
				Enabled        bool                     `json:"enabled"`
				Variants       []map[string]interface{} `json:"variants"`
				Rules          []map[string]interface{} `json:"rules"`
				DefaultVariant string                   `json:"defaultVariant"`
				Metadata       map[string]interface{}   `json:"metadata"`
			} `json:"payload"`
		} `json:"resource"`
	}

	body, _ := io.ReadAll(httpResp.Body)
	if err := json.Unmarshal(body, &flagResponse); err != nil {
		resp.Diagnostics.AddError("Parse Error", fmt.Sprintf("Unable to parse flag response: %s", err))
		return
	}

	// Extract segment keys from plan
	var segmentKeys []string
	resp.Diagnostics.Append(data.SegmentKeys.ElementsAs(ctx, &segmentKeys, false)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Extract old segment keys from state to find the rule
	var oldSegmentKeys []string
	resp.Diagnostics.Append(state.SegmentKeys.ElementsAs(ctx, &oldSegmentKeys, false)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Find and update the rule in the rules array by matching old state values
	var found bool
	existingRules := flagResponse.Resource.Payload.Rules
	if existingRules == nil {
		existingRules = []map[string]interface{}{}
	}

	for i, rule := range existingRules {
		// Match by old state values (operator and rank) to find the rule to update
		ruleSegments, _ := rule["segments"].([]interface{})
		ruleOperator, _ := rule["segmentOperator"].(string)
		ruleRank, _ := rule["rank"].(float64)

		// Check if this rule matches the old state
		segmentsMatch := len(ruleSegments) == len(oldSegmentKeys)
		if segmentsMatch {
			for j, seg := range ruleSegments {
				if segStr, ok := seg.(string); ok && j < len(oldSegmentKeys) {
					if segStr != oldSegmentKeys[j] {
						segmentsMatch = false
						break
					}
				}
			}
		}

		if segmentsMatch &&
			ruleOperator == state.SegmentOperator.ValueString() &&
			int64(ruleRank) == state.Rank.ValueInt64() {
			found = true

			// Preserve distributions if they exist
			distributions := rule["distributions"]
			if distributions == nil {
				distributions = []interface{}{}
			}

			// Update the rule with new values
			existingRules[i] = map[string]interface{}{
				"segments":        segmentKeys,
				"segmentOperator": data.SegmentOperator.ValueString(),
				"rank":            data.Rank.ValueInt64(),
				"distributions":   distributions,
			}
			break
		}
	}

	if !found {
		resp.Diagnostics.AddError("Not Found", fmt.Sprintf("Rule with state ID %s not found in flag (operator=%s, rank=%d)",
			state.ID.ValueString(), state.SegmentOperator.ValueString(), state.Rank.ValueInt64()))
		return
	}

	// Update the flag with all rules (including the modified one)
	flagPayload := map[string]interface{}{
		"@type":          "flipt.core.Flag",
		"key":            flagResponse.Resource.Payload.Key,
		"name":           flagResponse.Resource.Payload.Name,
		"description":    flagResponse.Resource.Payload.Description,
		"type":           flagResponse.Resource.Payload.Type,
		"enabled":        flagResponse.Resource.Payload.Enabled,
		"variants":       flagResponse.Resource.Payload.Variants,
		"rules":          existingRules,
		"defaultVariant": flagResponse.Resource.Payload.DefaultVariant,
		"metadata":       flagResponse.Resource.Payload.Metadata,
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
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to update rule, got error: %s", err))
		return
	}
	defer httpResp.Body.Close()

	body, _ = io.ReadAll(httpResp.Body)
	if httpResp.StatusCode != http.StatusOK {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to update rule, status: %d, body: %s", httpResp.StatusCode, string(body)))
		return
	}

	// Ensure EnvironmentKey is set in state
	data.EnvironmentKey = types.StringValue(envKey)

	// ID remains stable based on flag_key and rank (don't change it)

	tflog.Trace(ctx, "updated a rule resource")
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *RuleResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data RuleResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Determine environment key (default to "default" if not specified)
	envKey := "default"
	if !data.EnvironmentKey.IsNull() && !data.EnvironmentKey.IsUnknown() {
		envKey = data.EnvironmentKey.ValueString()
	}

	tflog.Debug(ctx, "Deleting rule", map[string]interface{}{
		"environment_key": envKey,
		"namespace_key":   data.NamespaceKey.ValueString(),
		"flag_key":        data.FlagKey.ValueString(),
		"rule_id":         data.ID.ValueString(),
	})

	// Get the current flag to read existing rules
	flagURL := fmt.Sprintf("%s/api/v2/environments/%s/namespaces/%s/resources/flipt.core.Flag/%s",
		r.endpoint, envKey, data.NamespaceKey.ValueString(), data.FlagKey.ValueString())

	httpReq, err := http.NewRequestWithContext(ctx, "GET", flagURL, nil)
	if err != nil {
		resp.Diagnostics.AddError("Request Error", fmt.Sprintf("Unable to create request: %s", err))
		return
	}

	httpResp, err := r.httpClient.Do(httpReq)
	if err != nil {
		// If flag doesn't exist, rule is already gone
		return
	}
	defer httpResp.Body.Close()

	if httpResp.StatusCode == http.StatusNotFound {
		// Flag doesn't exist, rule is already gone
		return
	}

	if httpResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(httpResp.Body)
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to read flag, status: %d, body: %s", httpResp.StatusCode, string(body)))
		return
	}

	var flagResponse struct {
		Resource struct {
			Payload struct {
				Type           string                   `json:"type"`
				Key            string                   `json:"key"`
				Name           string                   `json:"name"`
				Description    string                   `json:"description"`
				Enabled        bool                     `json:"enabled"`
				Variants       []map[string]interface{} `json:"variants"`
				Rules          []map[string]interface{} `json:"rules"`
				DefaultVariant string                   `json:"defaultVariant"`
				Metadata       map[string]interface{}   `json:"metadata"`
			} `json:"payload"`
		} `json:"resource"`
	}

	body, _ := io.ReadAll(httpResp.Body)
	if err := json.Unmarshal(body, &flagResponse); err != nil {
		resp.Diagnostics.AddError("Parse Error", fmt.Sprintf("Unable to parse flag response: %s", err))
		return
	}

	// Remove the rule from the rules array
	existingRules := flagResponse.Resource.Payload.Rules
	if existingRules == nil {
		// No rules, already deleted
		return
	}

	var updatedRules []map[string]interface{}
	for _, rule := range existingRules {
		if id, ok := rule["id"].(string); ok && id != data.ID.ValueString() {
			updatedRules = append(updatedRules, rule)
		}
	}

	// Update the flag without the deleted rule
	flagPayload := map[string]interface{}{
		"@type":          "flipt.core.Flag",
		"key":            flagResponse.Resource.Payload.Key,
		"name":           flagResponse.Resource.Payload.Name,
		"description":    flagResponse.Resource.Payload.Description,
		"type":           flagResponse.Resource.Payload.Type,
		"enabled":        flagResponse.Resource.Payload.Enabled,
		"variants":       flagResponse.Resource.Payload.Variants,
		"rules":          updatedRules,
		"defaultVariant": flagResponse.Resource.Payload.DefaultVariant,
		"metadata":       flagResponse.Resource.Payload.Metadata,
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
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to delete rule, got error: %s", err))
		return
	}
	defer httpResp.Body.Close()

	body, _ = io.ReadAll(httpResp.Body)
	if httpResp.StatusCode != http.StatusOK {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to delete rule, status: %d, body: %s", httpResp.StatusCode, string(body)))
		return
	}

	tflog.Trace(ctx, "deleted a rule resource")
}

func (r *RuleResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
