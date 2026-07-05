package engine

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// LoadWorkflowFromFile loads a workflow definition from a JSON file
func LoadWorkflowFromFile(filePath string) (*DynamicWorkflow, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read workflow file '%s': %w", filePath, err)
	}

	workflow, err := ParseWorkflowFromJSON(data)
	if err != nil {
		return nil, fmt.Errorf("failed to parse workflow file '%s': %w", filePath, err)
	}

	return workflow, nil
}

// LoadWorkflowFromReader loads a workflow definition from an io.Reader
func LoadWorkflowFromReader(r io.Reader) (*DynamicWorkflow, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("failed to read workflow data: %w", err)
	}

	workflow, err := ParseWorkflowFromJSON(data)
	if err != nil {
		return nil, fmt.Errorf("failed to parse workflow data: %w", err)
	}

	return workflow, nil
}

// InitWorkflowsFromDir loads and registers all workflow definitions from a directory
// It scans the directory for .json files and attempts to parse them as workflows
func InitWorkflowsFromDir(dirPath string) error {
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return fmt.Errorf("failed to read workflows directory '%s': %w", dirPath, err)
	}

	var loadedCount int
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		if !strings.HasSuffix(strings.ToLower(entry.Name()), ".json") {
			continue
		}

		filePath := filepath.Join(dirPath, entry.Name())
		workflow, err := LoadWorkflowFromFile(filePath)
		if err != nil {
			return fmt.Errorf("failed to load workflow from '%s': %w", filePath, err)
		}

		RegisterWorkflow(workflow)
		loadedCount++
	}

	if loadedCount == 0 {
		return fmt.Errorf("no workflow files found in directory '%s'", dirPath)
	}

	return nil
}

// LoadAndRegisterWorkflow loads a single workflow file and registers it
func LoadAndRegisterWorkflow(filePath string) error {
	workflow, err := LoadWorkflowFromFile(filePath)
	if err != nil {
		return err
	}

	RegisterWorkflow(workflow)
	return nil
}

// WorkflowLoader provides a configurable way to load workflows
type WorkflowLoader struct {
	basePath string
}

// NewWorkflowLoader creates a new workflow loader with a base path
func NewWorkflowLoader(basePath string) *WorkflowLoader {
	return &WorkflowLoader{
		basePath: basePath,
	}
}

// LoadFromFile loads a workflow from a relative path (relative to basePath)
func (wl *WorkflowLoader) LoadFromFile(relativePath string) (*DynamicWorkflow, error) {
	fullPath := filepath.Join(wl.basePath, relativePath)
	return LoadWorkflowFromFile(fullPath)
}

// LoadAll loads all workflows from the basePath directory
func (wl *WorkflowLoader) LoadAll() error {
	return InitWorkflowsFromDir(wl.basePath)
}

// SetBasePath updates the base path for loading workflows
func (wl *WorkflowLoader) SetBasePath(basePath string) {
	wl.basePath = basePath
}

// GetBasePath returns the current base path
func (wl *WorkflowLoader) GetBasePath() string {
	return wl.basePath
}

// ValidateWorkflowDefinition validates that a workflow definition is complete and valid
func ValidateWorkflowDefinition(workflow *DynamicWorkflow) error {
	if workflow.WorkflowType == "" {
		return fmt.Errorf("workflow type cannot be empty")
	}

	if workflow.InitialStepName == "" {
		return fmt.Errorf("initial step name cannot be empty")
	}

	if len(workflow.Steps) == 0 {
		return fmt.Errorf("workflow must have at least one step")
	}

	// Validate initial step exists
	if _, err := workflow.GetStep(workflow.InitialStepName); err != nil {
		return fmt.Errorf("initial step '%s' does not exist: %w", workflow.InitialStepName, err)
	}

	// Validate each step's actions reference valid next steps
	for stepName, step := range workflow.Steps {
		if step.Name == "" {
			return fmt.Errorf("step at index has empty name in workflow '%s'", workflow.WorkflowType)
		}

		if step.Name != stepName {
			return fmt.Errorf("step key '%s' does not match step name '%s'", stepName, step.Name)
		}

		for i, action := range step.Actions {
			if action.Name == "" {
				return fmt.Errorf("action at index %d in step '%s' has empty name", i, stepName)
			}

			// If there's a next step, verify it exists
			if action.NextStep != "" {
				if _, err := workflow.GetStep(action.NextStep); err != nil {
					return fmt.Errorf("action '%s' in step '%s' references non-existent next step '%s': %w",
						action.Name, stepName, action.NextStep, err)
				}
			}
		}
	}

	return nil
}

// MustLoadWorkflowFromFile loads a workflow from file or panics on error
// Useful for initialization during application startup
func MustLoadWorkflowFromFile(filePath string) *DynamicWorkflow {
	workflow, err := LoadWorkflowFromFile(filePath)
	if err != nil {
		panic(fmt.Sprintf("failed to load workflow from '%s': %v", filePath, err))
	}
	return workflow
}

// ParseWorkflowFromJSON is exported from definition.go but also useful here
// This allows users to parse workflows from strings or other sources
func LoadWorkflowFromString(jsonStr string) (*DynamicWorkflow, error) {
	return ParseWorkflowFromJSON([]byte(jsonStr))
}

// ToJSON serializes a workflow definition to JSON
func (w *DynamicWorkflow) ToJSON() ([]byte, error) {
	return json.MarshalIndent(w, "", "  ")
}

// ToJSONString serializes a workflow definition to a JSON string
func (w *DynamicWorkflow) ToJSONString() (string, error) {
	bytes, err := w.ToJSON()
	if err != nil {
		return "", err
	}
	return string(bytes), nil
}
