package echo

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/labstack/echo/v4"
	"github.com/hamid-moghadam/go-workflow-engine/pkg/engine"
)

type CreateStepRequest struct {
	ActionName string                 `json:"action_name"`
	Input      map[string]interface{} `json:"input,omitempty"`
}

type CreateStepResponse struct {
	InstanceID   int64  `json:"instance_id"`
	WorkflowType string `json:"workflow_type"`
	CurrentStep  string `json:"current_step"`
	CurrentState string `json:"current_state"`
	Success      bool   `json:"success"`
	Message      string `json:"message,omitempty"`
}

type GetStepRequest struct {
	WorkflowType string `param:"type"`
	StepName     string `param:"step_name"`
	ActionName   string `query:"action_name"`
}

type GetStepResponse struct {
	StepName     string                 `json:"step_name"`
	StepTitle    string                 `json:"step_title,omitempty"`
	CurrentState string                 `json:"current_state"`
	Actions      []ActionInfo           `json:"actions,omitempty"`
	StaticData   map[string]interface{} `json:"static_data,omitempty"`
}

type ActionInfo struct {
	Name       string `json:"name"`
	NextStep   string `json:"next_step,omitempty"`
	NewState   string `json:"new_state,omitempty"`
}

type GetWorkflowResponse struct {
	InstanceID   int64  `json:"instance_id"`
	WorkflowType string `json:"workflow_type"`
	CurrentStep  string `json:"current_step"`
	CurrentState string `json:"current_state"`
	IsFinished   bool   `json:"is_finished"`
	CreatedAt    string `json:"created_at"`
	UpdatedAt    string `json:"updated_at"`
}

type ListWorkflowsRequest struct {
	WorkflowType string `query:"workflow_type"`
	UserID       int64  `query:"user_id"`
	CurrentStep  string `query:"current_step"`
	CurrentState string `query:"current_state"`
	IsFinished   *bool  `query:"is_finished"`
	Limit        int    `query:"limit"`
	Offset       int    `query:"offset"`
}

type ListWorkflowsResponse struct {
	Workflows []WorkflowSummary `json:"workflows"`
	Total     int               `json:"total"`
}

type WorkflowSummary struct {
	InstanceID   int64  `json:"instance_id"`
	WorkflowType string `json:"workflow_type"`
	CurrentStep  string `json:"current_step"`
	CurrentState string `json:"current_state"`
	UserID       int64  `json:"user_id"`
	IsFinished   bool   `json:"is_finished"`
	CreatedAt    string `json:"created_at"`
}

type TransitStepRequest struct {
	ActionName string                 `json:"action_name"`
	Input      map[string]interface{} `json:"input,omitempty"`
}

type GetTransitionHistoryResponse struct {
	TransitionID int64          `json:"transition_id"`
	History      []HistoryEntry `json:"history"`
}

type HistoryEntry struct {
	ID        int64       `json:"id"`
	FieldName string      `json:"field_name"`
	OldValue  interface{} `json:"old_value,omitempty"`
	NewValue  interface{} `json:"new_value,omitempty"`
	CreatedAt string      `json:"created_at"`
}

// CreateStepHandler handles PUT /workflows/:type/steps/:step_name
// Creates or processes a step transition for the authenticated user
func CreateStepHandler(c echo.Context) error {
	svc, err := MustGetService(c)
	if err != nil {
		return Error(c, http.StatusInternalServerError, "service not available", err)
	}

	userID, err := GetUserIDWithError(c)
	if err != nil {
		return Error(c, http.StatusUnauthorized, "authentication required", err)
	}

	var req CreateStepRequest
	if err := c.Bind(&req); err != nil {
		return Error(c, http.StatusBadRequest, "invalid request body", err)
	}

	workflowType := c.Param("type")
	stepName := c.Param("step_name")

	if workflowType == "" || stepName == "" {
		return Error(c, http.StatusBadRequest, "workflow type and step name are required", nil)
	}

	ctx := c.Request().Context()
	instance, err := svc.GetInstance(ctx, userID, workflowType)
	if err != nil {
		instance, err = svc.CreateInstance(ctx, workflowType, userID, nil)
		if err != nil {
			return Error(c, http.StatusInternalServerError, "failed to create workflow instance", err)
		}
	}

	if err := svc.TransitionStep(ctx, instance.ID, req.ActionName, req.Input); err != nil {
		return Error(c, http.StatusBadRequest, "transition failed", err)
	}

	instance, err = svc.GetInstanceByID(ctx, instance.ID)
	if err != nil {
		return Error(c, http.StatusInternalServerError, "failed to refresh instance", err)
	}

	resp := CreateStepResponse{
		InstanceID:   instance.ID,
		WorkflowType: instance.WorkflowType,
		CurrentStep:  instance.CurrentStep,
		CurrentState: instance.CurrentState,
		Success:      true,
		Message:      "step transition completed successfully",
	}

	return JSON(c, http.StatusOK, resp)
}

// GetStepHandler handles GET /workflows/:type/steps/:step_name?action_name=xxx
// Retrieves step data including available actions and static data
func GetStepHandler(c echo.Context) error {
	svc, err := MustGetService(c)
	if err != nil {
		return Error(c, http.StatusInternalServerError, "service not available", err)
	}

	userID, err := GetUserIDWithError(c)
	if err != nil {
		return Error(c, http.StatusUnauthorized, "authentication required", err)
	}

	var req GetStepRequest
	if err := c.Bind(&req); err != nil {
		return Error(c, http.StatusBadRequest, "invalid request parameters", err)
	}

	workflowType := c.Param("type")
	stepName := c.Param("step_name")

	if workflowType == "" || stepName == "" {
		return Error(c, http.StatusBadRequest, "workflow type and step name are required", nil)
	}

	ctx := c.Request().Context()
	instance, err := svc.GetInstance(ctx, userID, workflowType)
	if err != nil {
		return Error(c, http.StatusNotFound, "no active workflow found", err)
	}

	def, err := svc.GetWorkflowDefinition(workflowType)
	if err != nil {
		return Error(c, http.StatusInternalServerError, "workflow definition not found", err)
	}

	step, err := def.GetStep(instance.CurrentStep)
	if err != nil {
		return Error(c, http.StatusInternalServerError, "step not found", err)
	}

	actionInfos := make([]ActionInfo, 0, len(step.Actions))
	for _, action := range step.Actions {
		actionInfos = append(actionInfos, ActionInfo{
			Name:     action.Name,
			NextStep: action.NextStep,
			NewState: action.NewState,
		})
	}

	resp := GetStepResponse{
		StepName:     step.Name,
		StepTitle:    step.Title,
		CurrentState: instance.CurrentState,
		Actions:      actionInfos,
		StaticData:   step.StaticData,
	}

	return JSON(c, http.StatusOK, resp)
}

// GetWorkflowHandler handles GET /workflows/:type
// Retrieves the current workflow instance for the authenticated user
func GetWorkflowHandler(c echo.Context) error {
	svc, err := MustGetService(c)
	if err != nil {
		return Error(c, http.StatusInternalServerError, "service not available", err)
	}

	userID, err := GetUserIDWithError(c)
	if err != nil {
		return Error(c, http.StatusUnauthorized, "authentication required", err)
	}

	workflowType := c.Param("type")
	if workflowType == "" {
		return Error(c, http.StatusBadRequest, "workflow type is required", nil)
	}

	ctx := c.Request().Context()
	instance, err := svc.GetInstance(ctx, userID, workflowType)
	if err != nil {
		return Error(c, http.StatusNotFound, "no active workflow found", err)
	}

	resp := GetWorkflowResponse{
		InstanceID:   instance.ID,
		WorkflowType: instance.WorkflowType,
		CurrentStep:  instance.CurrentStep,
		CurrentState: instance.CurrentState,
		IsFinished:   instance.IsFinished(),
		CreatedAt:    instance.CreatedAt.Format("2006-01-02T15:04:05Z"),
		UpdatedAt:    instance.UpdatedAt.Format("2006-01-02T15:04:05Z"),
	}

	return JSON(c, http.StatusOK, resp)
}

// ListWorkflowsHandler handles GET /admin/workflows
// Lists all workflow instances with filtering (admin only)
func ListWorkflowsHandler(c echo.Context) error {
	svc, err := MustGetService(c)
	if err != nil {
		return Error(c, http.StatusInternalServerError, "service not available", err)
	}

	var req ListWorkflowsRequest
	if err := c.Bind(&req); err != nil {
		return Error(c, http.StatusBadRequest, "invalid query parameters", err)
	}

	if req.Limit <= 0 {
		req.Limit = 100
	}

	filter := engine.InstanceFilter{
		WorkflowType: req.WorkflowType,
		CurrentStep:  req.CurrentStep,
		CurrentState: req.CurrentState,
		Limit:        req.Limit,
		Offset:       req.Offset,
	}

	if req.UserID > 0 {
		filter.UserID = &req.UserID
	}
	if req.IsFinished != nil {
		filter.IsFinished = req.IsFinished
	}

	ctx := c.Request().Context()
	instances, err := svc.ListInstances(ctx, filter)
	if err != nil {
		return Error(c, http.StatusInternalServerError, "failed to list workflows", err)
	}

	summaries := make([]WorkflowSummary, 0, len(instances))
	for _, inst := range instances {
		summaries = append(summaries, WorkflowSummary{
			InstanceID:   inst.ID,
			WorkflowType: inst.WorkflowType,
			CurrentStep:  inst.CurrentStep,
			CurrentState: inst.CurrentState,
			UserID:       inst.UserID,
			IsFinished:   inst.IsFinished(),
			CreatedAt:    inst.CreatedAt.Format("2006-01-02T15:04:05Z"),
		})
	}

	resp := ListWorkflowsResponse{
		Workflows: summaries,
		Total:     len(summaries),
	}

	return JSON(c, http.StatusOK, resp)
}

// TransitStepHandler handles PUT /admin/workflows/:id/steps/:step_name
// Admin endpoint to trigger a step transition for any workflow
func TransitStepHandler(c echo.Context) error {
	svc, err := MustGetService(c)
	if err != nil {
		return Error(c, http.StatusInternalServerError, "service not available", err)
	}

	instanceIDStr := c.Param("id")
	instanceID, err := strconv.ParseInt(instanceIDStr, 10, 64)
	if err != nil {
		return Error(c, http.StatusBadRequest, "invalid workflow instance ID", err)
	}

	stepName := c.Param("step_name")
	if stepName == "" {
		return Error(c, http.StatusBadRequest, "step name is required", nil)
	}

	var req TransitStepRequest
	if err := c.Bind(&req); err != nil {
		return Error(c, http.StatusBadRequest, "invalid request body", err)
	}

	if req.ActionName == "" {
		return Error(c, http.StatusBadRequest, "action name is required", nil)
	}

	ctx := c.Request().Context()

	instance, err := svc.GetInstanceByID(ctx, instanceID)
	if err != nil {
		return Error(c, http.StatusNotFound, "workflow instance not found", err)
	}

	if stepName != instance.CurrentStep {
		return Error(c, http.StatusBadRequest,
			fmt.Sprintf("step '%s' does not match current step '%s'", stepName, instance.CurrentStep), nil)
	}

	if err := svc.TransitionStep(ctx, instanceID, req.ActionName, req.Input); err != nil {
		return Error(c, http.StatusBadRequest, "transition failed", err)
	}

	instance, err = svc.GetInstanceByID(ctx, instanceID)
	if err != nil {
		return Error(c, http.StatusInternalServerError, "failed to get updated instance", err)
	}

	resp := CreateStepResponse{
		InstanceID:   instance.ID,
		WorkflowType: instance.WorkflowType,
		CurrentStep:  instance.CurrentStep,
		CurrentState: instance.CurrentState,
		Success:      true,
		Message:      "admin step transition completed successfully",
	}

	return JSON(c, http.StatusOK, resp)
}

// GetTransitionHistoryHandler handles GET /admin/workflows/:id/transitions/:transition_id/history
// Retrieves the history for a specific transition (admin only)
func GetTransitionHistoryHandler(c echo.Context) error {
	svc, err := MustGetService(c)
	if err != nil {
		return Error(c, http.StatusInternalServerError, "service not available", err)
	}

	transitionIDStr := c.Param("transition_id")
	transitionID, err := strconv.ParseInt(transitionIDStr, 10, 64)
	if err != nil {
		return Error(c, http.StatusBadRequest, "invalid transition ID", err)
	}

	ctx := c.Request().Context()

	_, err = svc.GetTransitionByID(ctx, transitionID)
	if err != nil {
		return Error(c, http.StatusNotFound, "transition not found", err)
	}

	histories, err := svc.ListTransitionHistory(ctx, transitionID)
	if err != nil {
		return Error(c, http.StatusInternalServerError, "failed to get transition history", err)
	}

	entries := make([]HistoryEntry, 0, len(histories))
	for _, h := range histories {
		var oldValue, newValue interface{}
		if h.OldValue != nil {
			_ = json.Unmarshal(h.OldValue, &oldValue)
		}
		if h.NewValue != nil {
			_ = json.Unmarshal(h.NewValue, &newValue)
		}
		entries = append(entries, HistoryEntry{
			ID:        h.ID,
			FieldName: h.FieldName,
			OldValue:  oldValue,
			NewValue:  newValue,
			CreatedAt: h.CreatedAt.Format("2006-01-02T15:04:05Z"),
		})
	}

	resp := GetTransitionHistoryResponse{
		TransitionID: transitionID,
		History:      entries,
	}

	return JSON(c, http.StatusOK, resp)
}
