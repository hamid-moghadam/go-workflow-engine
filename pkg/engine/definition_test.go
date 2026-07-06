package engine

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestDynamicWorkflowImplementsInterface verifies that DynamicWorkflow implements WorkflowDefinition
func TestDynamicWorkflowImplementsInterface(t *testing.T) {
	workflow := &DynamicWorkflow{
		WorkflowType:    "test",
		InitialStepName: "step1",
		InitialState:    "initial",
		Steps: map[string]*StepDefinition{
			"step1": {
				Name:  "step1",
				Title: "Step 1",
				Order: 1,
				Actions: []Action{
					{Name: "action1", NextStep: "step2"},
				},
			},
			"step2": {
				Name:  "step2",
				Title: "Step 2",
				Order: 2,
				Actions: []Action{
					{Name: "action2"},
				},
			},
		},
	}

	// Verify it implements the interface
	var _ WorkflowDefinition = workflow

	// Test GetType
	assert.Equal(t, "test", workflow.GetType())

	// Test GetInitialStepName
	assert.Equal(t, "step1", workflow.GetInitialStepName())

	// Test GetInitialState
	assert.Equal(t, "initial", workflow.GetInitialState())
}

// TestGetStepSuccess tests successful step retrieval
func TestGetStepSuccess(t *testing.T) {
	workflow := &DynamicWorkflow{
		WorkflowType:    "test",
		InitialStepName: "step1",
		Steps: map[string]*StepDefinition{
			"step1": {
				Name:  "step1",
				Title: "Step 1",
				Order: 1,
				Actions: []Action{
					{Name: "action1"},
				},
			},
		},
	}

	step, err := workflow.GetStep("step1")
	require.NoError(t, err)
	assert.Equal(t, "step1", step.Name)
	assert.Equal(t, "Step 1", step.Title)
	assert.Equal(t, 1, step.Order)
}

// TestGetStepError tests error cases for GetStep
func TestGetStepError(t *testing.T) {
	workflow := &DynamicWorkflow{
		WorkflowType:    "test",
		InitialStepName: "step1",
		Steps: map[string]*StepDefinition{
			"step1": {
				Name:  "step1",
				Title: "Step 1",
			},
		},
	}

	// Test non-existent step
	step, err := workflow.GetStep("nonexistent")
	assert.Error(t, err)
	assert.Nil(t, step)
	assert.Contains(t, err.Error(), "not found")
	assert.Contains(t, err.Error(), "nonexistent")
}

// TestGetTransitionActionSuccess tests successful action retrieval
func TestGetTransitionActionSuccess(t *testing.T) {
	workflow := &DynamicWorkflow{
		WorkflowType:    "test",
		InitialStepName: "step1",
		Steps: map[string]*StepDefinition{
			"step1": {
				Name:  "step1",
				Title: "Step 1",
				Actions: []Action{
					{Name: "SUBMIT", NextStep: "step2", NewState: "Submitted"},
					{Name: "CANCEL", NextStep: "", NewState: "Cancelled"},
				},
			},
		},
	}

	action, err := workflow.GetTransitionAction("step1", "SUBMIT")
	require.NoError(t, err)
	assert.Equal(t, "SUBMIT", action.Name)
	assert.Equal(t, "step2", action.NextStep)
	assert.Equal(t, "Submitted", action.NewState)

	// Test second action
	action2, err := workflow.GetTransitionAction("step1", "CANCEL")
	require.NoError(t, err)
	assert.Equal(t, "CANCEL", action2.Name)
}

// TestGetTransitionActionError tests error cases for GetTransitionAction
func TestGetTransitionActionError(t *testing.T) {
	workflow := &DynamicWorkflow{
		WorkflowType:    "test",
		InitialStepName: "step1",
		Steps: map[string]*StepDefinition{
			"step1": {
				Name:    "step1",
				Title:   "Step 1",
				Actions: []Action{
					{Name: "SUBMIT"},
				},
			},
		},
	}

	// Test non-existent step
	action, err := workflow.GetTransitionAction("nonexistent", "SUBMIT")
	assert.Error(t, err)
	assert.Nil(t, action)
	assert.Contains(t, err.Error(), "step")
	assert.Contains(t, err.Error(), "not found")

	// Test non-existent action on existing step
	action, err = workflow.GetTransitionAction("step1", "NONEXISTENT")
	assert.Error(t, err)
	assert.Nil(t, action)
	assert.Contains(t, err.Error(), "action")
	assert.Contains(t, err.Error(), "not found")
}

// TestParseWorkflowFromJSONValid tests parsing valid JSON workflow definitions
func TestParseWorkflowFromJSONValid(t *testing.T) {
	jsonData := `{
		"workflow_type": "approval",
		"initial_step_name": "submit",
		"initial_state": "Pending",
		"steps": [
			{
				"name": "submit",
				"title": "Submit Request",
				"order": 1,
				"static_data": {"description": "Submit your request"},
				"actions": [
					{
						"name": "SUBMIT",
						"next_step": "review",
						"new_state": "Submitted",
						"validation_func": "validateSubmit"
					}
				]
			},
			{
				"name": "review",
				"title": "Manager Review",
				"order": 2,
				"actions": [
					{
						"name": "APPROVE",
						"next_step": "done",
						"new_state": "Approved"
					},
					{
						"name": "REJECT",
						"next_step": "submit",
						"new_state": "Rejected"
					}
				]
			}
		]
	}`

	workflow, err := ParseWorkflowFromJSON([]byte(jsonData))
	require.NoError(t, err)

	// Verify parsed workflow
	assert.Equal(t, "approval", workflow.WorkflowType)
	assert.Equal(t, "submit", workflow.InitialStepName)
	assert.Equal(t, "Pending", workflow.InitialState)
	assert.Len(t, workflow.Steps, 2)

	// Verify first step
	submitStep, ok := workflow.Steps["submit"]
	require.True(t, ok)
	assert.Equal(t, "submit", submitStep.Name)
	assert.Equal(t, "Submit Request", submitStep.Title)
	assert.Equal(t, 1, submitStep.Order)
	assert.Equal(t, "Submit your request", submitStep.StaticData["description"])
	require.Len(t, submitStep.Actions, 1)
	assert.Equal(t, "SUBMIT", submitStep.Actions[0].Name)
	assert.Equal(t, "review", submitStep.Actions[0].NextStep)
	assert.Equal(t, "Submitted", submitStep.Actions[0].NewState)
	assert.Equal(t, "validateSubmit", submitStep.Actions[0].ValidationFunc)

	// Verify second step
	reviewStep, ok := workflow.Steps["review"]
	require.True(t, ok)
	assert.Equal(t, "review", reviewStep.Name)
	require.Len(t, reviewStep.Actions, 2)
}

// TestParseWorkflowFromJSONInvalid tests parsing invalid JSON
func TestParseWorkflowFromJSONInvalid(t *testing.T) {
	tests := []struct {
		name     string
		jsonData string
		wantErr  string
	}{
		{
			name:     "invalid JSON syntax",
			jsonData: `{invalid json}`,
			wantErr:  "failed to parse",
		},
		{
			name:     "empty JSON",
			jsonData: ``,
			wantErr:  "failed to parse",
		},
		{
			name:     "invalid JSON structure",
			jsonData: `{"initial_step_name": "step1", "steps": "not an array"}`,
			wantErr:  "failed to parse",
		},
		{
			name:     "invalid steps type",
			jsonData: `{"workflow_type": "test", "steps": "not an array"}`,
			wantErr:  "failed to parse",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			workflow, err := ParseWorkflowFromJSON([]byte(tt.jsonData))
			assert.Error(t, err)
			assert.Nil(t, workflow)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

// TestWorkflowDefinitionInterface ensures all interface methods work correctly
func TestWorkflowDefinitionInterface(t *testing.T) {
	workflow := &DynamicWorkflow{
		WorkflowType:    "interface-test",
		InitialStepName: "start",
		InitialState:    "Initial",
		Steps: map[string]*StepDefinition{
			"start": {
				Name:  "start",
				Title: "Start Step",
				Order: 0,
				Actions: []Action{
					{Name: "PROCEED", NextStep: "end"},
				},
			},
			"end": {
				Name:    "end",
				Title:   "End Step",
				Order:   1,
				Actions: []Action{},
			},
		},
	}

	// Test the interface methods
	def, ok := interface{}(workflow).(WorkflowDefinition)
	require.True(t, ok)

	assert.Equal(t, "interface-test", def.GetType())
	assert.Equal(t, "start", def.GetInitialStepName())
	assert.Equal(t, "Initial", def.GetInitialState())

	// Test GetStep
	step, err := def.GetStep("start")
	require.NoError(t, err)
	assert.Equal(t, "Start Step", step.Title)

	// Test GetTransitionAction
	action, err := def.GetTransitionAction("start", "PROCEED")
	require.NoError(t, err)
	assert.Equal(t, "PROCEED", action.Name)
	assert.Equal(t, "end", action.NextStep)
}
