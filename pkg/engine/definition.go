package engine

import (
	"encoding/json"
	"fmt"
)

// WorkflowDefinition defines the interface for workflow definitions
type WorkflowDefinition interface {
	GetType() string
	GetInitialStepName() string
	GetInitialState() string
	GetStep(stepName string) (*StepDefinition, error)
	GetTransitionAction(stepName, actionName string) (*Action, error)
}

// DynamicWorkflow implements WorkflowDefinition with JSON-based configuration
type DynamicWorkflow struct {
	WorkflowType     string                      `json:"workflow_type"`
	InitialStepName  string                      `json:"initial_step_name"`
	InitialState     string                      `json:"initial_state"`
	Steps            map[string]*StepDefinition  `json:"steps"`
}

// GetType returns the workflow type identifier
func (w *DynamicWorkflow) GetType() string {
	return w.WorkflowType
}

// GetInitialStepName returns the starting step name
func (w *DynamicWorkflow) GetInitialStepName() string {
	return w.InitialStepName
}

// GetInitialState returns the initial workflow state
func (w *DynamicWorkflow) GetInitialState() string {
	return w.InitialState
}

// GetStep retrieves a step definition by name
func (w *DynamicWorkflow) GetStep(stepName string) (*StepDefinition, error) {
	step, exists := w.Steps[stepName]
	if !exists {
		return nil, fmt.Errorf("step '%s' not found in workflow '%s'", stepName, w.WorkflowType)
	}
	return step, nil
}

// GetTransitionAction retrieves an action for a given step and action name
func (w *DynamicWorkflow) GetTransitionAction(stepName, actionName string) (*Action, error) {
	step, err := w.GetStep(stepName)
	if err != nil {
		return nil, err
	}

	for i := range step.Actions {
		if step.Actions[i].Name == actionName {
			return &step.Actions[i], nil
		}
	}

	return nil, fmt.Errorf("action '%s' not found in step '%s'", actionName, stepName)
}

// StepDefinition defines a single step in a workflow
type StepDefinition struct {
	Name       string                 `json:"name"`
	Title      string                 `json:"title"`
	Order      int                    `json:"order"`
	StaticData map[string]interface{} `json:"static_data,omitempty"`
	Actions    []Action               `json:"actions"`
}

// Action defines a possible action within a step
type Action struct {
	Name            string `json:"name"`
	NextStep        string `json:"next_step,omitempty"`
	NewState        string `json:"new_state,omitempty"`
	ValidationFunc  string `json:"validation_func,omitempty"`
}

// workflowRegistry stores workflow definitions by type
var workflowRegistry = make(map[string]WorkflowDefinition)

// RegisterWorkflow registers a workflow definition
func RegisterWorkflow(def WorkflowDefinition) {
	workflowRegistry[def.GetType()] = def
}

// GetWorkflow retrieves a workflow definition by type
func GetWorkflow(workflowType string) (WorkflowDefinition, error) {
	def, exists := workflowRegistry[workflowType]
	if !exists {
		return nil, fmt.Errorf("workflow type '%s' not registered", workflowType)
	}
	return def, nil
}

// ListWorkflowTypes returns all registered workflow type names
func ListWorkflowTypes() []string {
	types := make([]string, 0, len(workflowRegistry))
	for t := range workflowRegistry {
		types = append(types, t)
	}
	return types
}

// UnregisterWorkflow removes a workflow definition from the registry
// Primarily used for testing cleanup
func UnregisterWorkflow(workflowType string) {
	delete(workflowRegistry, workflowType)
}

// workflowJSONConfig is used for parsing JSON workflow definitions
type workflowJSONConfig struct {
	WorkflowType    string                       `json:"workflow_type"`
	InitialStepName string                       `json:"initial_step_name"`
	InitialState    string                       `json:"initial_state"`
	Steps           []stepJSONConfig             `json:"steps"`
}

type stepJSONConfig struct {
	Name       string                 `json:"name"`
	Title      string                 `json:"title"`
	Order      int                    `json:"order"`
	StaticData map[string]interface{} `json:"static_data,omitempty"`
	Actions    []actionJSONConfig     `json:"actions"`
}

type actionJSONConfig struct {
	Name           string `json:"name"`
	NextStep       string `json:"next_step,omitempty"`
	NewState       string `json:"new_state,omitempty"`
	ValidationFunc string `json:"validation_func,omitempty"`
}

// ParseWorkflowFromJSON parses a JSON workflow definition
func ParseWorkflowFromJSON(data []byte) (*DynamicWorkflow, error) {
	var config workflowJSONConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse workflow JSON: %w", err)
	}

	workflow := &DynamicWorkflow{
		WorkflowType:    config.WorkflowType,
		InitialStepName: config.InitialStepName,
		InitialState:    config.InitialState,
		Steps:           make(map[string]*StepDefinition),
	}

	for i := range config.Steps {
		stepConfig := &config.Steps[i]
		step := &StepDefinition{
			Name:       stepConfig.Name,
			Title:      stepConfig.Title,
			Order:      stepConfig.Order,
			StaticData: stepConfig.StaticData,
			Actions:    make([]Action, len(stepConfig.Actions)),
		}

		for j := range stepConfig.Actions {
			actionConfig := &stepConfig.Actions[j]
			step.Actions[j] = Action{
				Name:           actionConfig.Name,
				NextStep:       actionConfig.NextStep,
				NewState:       actionConfig.NewState,
				ValidationFunc: actionConfig.ValidationFunc,
			}
		}

		workflow.Steps[step.Name] = step
	}

	return workflow, nil
}
