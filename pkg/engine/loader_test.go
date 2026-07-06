package engine

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestLoadFromFileValid tests loading a valid JSON file
func TestLoadFromFileValid(t *testing.T) {
	// Create a temporary directory
	tmpDir := t.TempDir()

	// Create a valid workflow JSON file
	jsonContent := `{
		"workflow_type": "test_workflow",
		"initial_step_name": "step1",
		"initial_state": "pending",
		"steps": [
			{
				"name": "step1",
				"title": "First Step",
				"order": 1,
				"actions": [
					{
						"name": "NEXT",
						"next_step": "step2",
						"new_state": "processing"
					}
				]
			},
			{
				"name": "step2",
				"title": "Second Step",
				"order": 2,
				"actions": []
			}
		]
	}`

	filePath := filepath.Join(tmpDir, "test_workflow.json")
	err := os.WriteFile(filePath, []byte(jsonContent), 0644)
	require.NoError(t, err)

	// Test loading
	workflow, err := LoadWorkflowFromFile(filePath)
	require.NoError(t, err)
	assert.NotNil(t, workflow)
	assert.Equal(t, "test_workflow", workflow.WorkflowType)
	assert.Equal(t, "step1", workflow.InitialStepName)
	assert.Equal(t, "pending", workflow.InitialState)
	assert.Len(t, workflow.Steps, 2)

	// Verify steps
	step1, ok := workflow.Steps["step1"]
	require.True(t, ok)
	assert.Equal(t, "First Step", step1.Title)
	assert.Equal(t, 1, step1.Order)
	require.Len(t, step1.Actions, 1)
	assert.Equal(t, "NEXT", step1.Actions[0].Name)
	assert.Equal(t, "step2", step1.Actions[0].NextStep)
	assert.Equal(t, "processing", step1.Actions[0].NewState)
}

// TestLoadFromFileInvalidJSON tests loading invalid JSON
func TestLoadFromFileInvalidJSON(t *testing.T) {
	tmpDir := t.TempDir()

	// Create an invalid JSON file
	filePath := filepath.Join(tmpDir, "invalid.json")
	err := os.WriteFile(filePath, []byte(`{invalid json}`), 0644)
	require.NoError(t, err)

	workflow, err := LoadWorkflowFromFile(filePath)
	assert.Error(t, err)
	assert.Nil(t, workflow)
	assert.Contains(t, err.Error(), "failed to parse")
}

// TestLoadFromFileMissingFile tests loading a non-existent file
func TestLoadFromFileMissingFile(t *testing.T) {
	workflow, err := LoadWorkflowFromFile("/nonexistent/path/workflow.json")
	assert.Error(t, err)
	assert.Nil(t, workflow)
	assert.Contains(t, err.Error(), "failed to read")
}

// TestLoadWorkflowFromString tests loading from a string
func TestLoadWorkflowFromString(t *testing.T) {
	jsonContent := `{
		"workflow_type": "string_test",
		"initial_step_name": "start",
		"initial_state": "initial",
		"steps": [
			{
				"name": "start",
				"title": "Start Step",
				"order": 0,
				"actions": [
					{
						"name": "COMPLETE",
						"new_state": "completed"
					}
				]
			}
		]
	}`

	workflow, err := LoadWorkflowFromString(jsonContent)
	require.NoError(t, err)
	assert.NotNil(t, workflow)
	assert.Equal(t, "string_test", workflow.WorkflowType)
	assert.Equal(t, "start", workflow.InitialStepName)
}

// TestInitWorkflowsFromDir tests loading multiple workflows from a directory
func TestInitWorkflowsFromDir(t *testing.T) {
	tmpDir := t.TempDir()

	// Create first workflow file
	workflow1 := `{
		"workflow_type": "workflow1",
		"initial_step_name": "step1",
		"initial_state": "pending",
		"steps": [
			{
				"name": "step1",
				"title": "Step 1",
				"order": 1,
				"actions": []
			}
		]
	}`
	err := os.WriteFile(filepath.Join(tmpDir, "workflow1.json"), []byte(workflow1), 0644)
	require.NoError(t, err)

	// Create second workflow file
	workflow2 := `{
		"workflow_type": "workflow2",
		"initial_step_name": "begin",
		"initial_state": "new",
		"steps": [
			{
				"name": "begin",
				"title": "Begin",
				"order": 1,
				"actions": []
			}
		]
	}`
	err = os.WriteFile(filepath.Join(tmpDir, "workflow2.json"), []byte(workflow2), 0644)
	require.NoError(t, err)

	// Create a non-JSON file (should be ignored)
	err = os.WriteFile(filepath.Join(tmpDir, "readme.txt"), []byte("readme"), 0644)
	require.NoError(t, err)

	// Clear any previously registered workflows
	workflowRegistry = make(map[string]WorkflowDefinition)

	// Load all workflows from directory
	err = InitWorkflowsFromDir(tmpDir)
	require.NoError(t, err)

	// Verify both workflows were registered
	wf1, err := GetWorkflow("workflow1")
	require.NoError(t, err)
	assert.Equal(t, "workflow1", wf1.GetType())

	wf2, err := GetWorkflow("workflow2")
	require.NoError(t, err)
	assert.Equal(t, "workflow2", wf2.GetType())
}

// TestInitWorkflowsFromDirEmpty tests loading from an empty directory
func TestInitWorkflowsFromDirEmpty(t *testing.T) {
	tmpDir := t.TempDir()

	err := InitWorkflowsFromDir(tmpDir)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no workflow files found")
}

// TestInitWorkflowsFromDirNonExistent tests loading from a non-existent directory
func TestInitWorkflowsFromDirNonExistent(t *testing.T) {
	err := InitWorkflowsFromDir("/nonexistent/directory")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to read")
}

// TestInitWorkflowsFromDirInvalidJSON tests handling of invalid JSON in directory
func TestInitWorkflowsFromDirInvalidJSON(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a valid workflow
	validWorkflow := `{
		"workflow_type": "valid",
		"initial_step_name": "step1",
		"initial_state": "pending",
		"steps": [
			{
				"name": "step1",
				"title": "Step 1",
				"order": 1,
				"actions": []
			}
		]
	}`
	err := os.WriteFile(filepath.Join(tmpDir, "valid.json"), []byte(validWorkflow), 0644)
	require.NoError(t, err)

	// Create an invalid workflow
	err = os.WriteFile(filepath.Join(tmpDir, "invalid.json"), []byte(`{invalid}`), 0644)
	require.NoError(t, err)

	// Clear registry
	workflowRegistry = make(map[string]WorkflowDefinition)

	// Should return error for invalid JSON
	err = InitWorkflowsFromDir(tmpDir)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to load workflow")
}

// TestValidateWorkflowDefinition tests workflow validation
func TestValidateWorkflowDefinition(t *testing.T) {
	tests := []struct {
		name      string
		workflow  *DynamicWorkflow
		wantError string
	}{
		{
			name: "valid workflow",
			workflow: &DynamicWorkflow{
				WorkflowType:    "test",
				InitialStepName: "step1",
				Steps: map[string]*StepDefinition{
					"step1": {
						Name:    "step1",
						Title:   "Step 1",
						Actions: []Action{{Name: "NEXT", NextStep: "step2"}},
					},
					"step2": {
						Name:    "step2",
						Title:   "Step 2",
						Actions: []Action{},
					},
				},
			},
			wantError: "",
		},
		{
			name: "missing workflow type",
			workflow: &DynamicWorkflow{
				WorkflowType:    "",
				InitialStepName: "step1",
				Steps: map[string]*StepDefinition{
					"step1": {Name: "step1", Actions: []Action{}},
				},
			},
			wantError: "workflow type cannot be empty",
		},
		{
			name: "missing initial step name",
			workflow: &DynamicWorkflow{
				WorkflowType:    "test",
				InitialStepName: "",
				Steps: map[string]*StepDefinition{
					"step1": {Name: "step1", Actions: []Action{}},
				},
			},
			wantError: "initial step name cannot be empty",
		},
		{
			name: "no steps",
			workflow: &DynamicWorkflow{
				WorkflowType:    "test",
				InitialStepName: "step1",
				Steps:           map[string]*StepDefinition{},
			},
			wantError: "workflow must have at least one step",
		},
		{
			name: "non-existent initial step",
			workflow: &DynamicWorkflow{
				WorkflowType:    "test",
				InitialStepName: "nonexistent",
				Steps: map[string]*StepDefinition{
					"step1": {Name: "step1", Actions: []Action{}},
				},
			},
			wantError: "initial step 'nonexistent' does not exist",
		},
		{
			name: "step with empty name",
			workflow: &DynamicWorkflow{
				WorkflowType:    "test",
				InitialStepName: "step1",
				Steps: map[string]*StepDefinition{
					"step1": {Name: "", Actions: []Action{}},
				},
			},
			wantError: "step at index has empty name",
		},
		{
			name: "step key mismatch",
			workflow: &DynamicWorkflow{
				WorkflowType:    "test",
				InitialStepName: "step1",
				Steps: map[string]*StepDefinition{
					"step1": {Name: "different", Actions: []Action{}},
				},
			},
			wantError: "step key 'step1' does not match step name 'different'",
		},
		{
			name: "action with empty name",
			workflow: &DynamicWorkflow{
				WorkflowType:    "test",
				InitialStepName: "step1",
				Steps: map[string]*StepDefinition{
					"step1": {
						Name:    "step1",
						Actions: []Action{{Name: ""}},
					},
				},
			},
			wantError: "action at index 0 in step 'step1' has empty name",
		},
		{
			name: "action with invalid next step",
			workflow: &DynamicWorkflow{
				WorkflowType:    "test",
				InitialStepName: "step1",
				Steps: map[string]*StepDefinition{
					"step1": {
						Name:    "step1",
						Actions: []Action{{Name: "NEXT", NextStep: "nonexistent"}},
					},
				},
			},
			wantError: "references non-existent next step 'nonexistent'",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateWorkflowDefinition(tt.workflow)
			if tt.wantError == "" {
				assert.NoError(t, err)
			} else {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantError)
			}
		})
	}
}

// TestWorkflowLoader tests the WorkflowLoader struct
func TestWorkflowLoader(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a workflow file
	jsonContent := `{
		"workflow_type": "loader_test",
		"initial_step_name": "start",
		"initial_state": "initial",
		"steps": [
			{
				"name": "start",
				"title": "Start",
				"order": 1,
				"actions": []
			}
		]
	}`
	err := os.WriteFile(filepath.Join(tmpDir, "test.json"), []byte(jsonContent), 0644)
	require.NoError(t, err)

	// Test creating loader
	loader := NewWorkflowLoader(tmpDir)
	assert.NotNil(t, loader)
	assert.Equal(t, tmpDir, loader.GetBasePath())

	// Test loading from relative path
	workflow, err := loader.LoadFromFile("test.json")
	require.NoError(t, err)
	assert.Equal(t, "loader_test", workflow.WorkflowType)

	// Test setting new base path
	newTmpDir := t.TempDir()
	loader.SetBasePath(newTmpDir)
	assert.Equal(t, newTmpDir, loader.GetBasePath())
}

// TestMustLoadWorkflowFromFile tests the must-load function
func TestMustLoadWorkflowFromFile(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a valid workflow
	jsonContent := `{
		"workflow_type": "must_load_test",
		"initial_step_name": "start",
		"initial_state": "initial",
		"steps": [
			{
				"name": "start",
				"title": "Start",
				"order": 1,
				"actions": []
			}
		]
	}`
	filePath := filepath.Join(tmpDir, "valid.json")
	err := os.WriteFile(filePath, []byte(jsonContent), 0644)
	require.NoError(t, err)

	// Test successful load (should not panic)
	workflow := MustLoadWorkflowFromFile(filePath)
	assert.Equal(t, "must_load_test", workflow.WorkflowType)

	// Test panic on invalid file
	assert.Panics(t, func() {
		MustLoadWorkflowFromFile("/nonexistent/file.json")
	})
}

// TestLoadWorkflowFromReader tests loading from an io.Reader
func TestLoadWorkflowFromReader(t *testing.T) {
	jsonContent := `{
		"workflow_type": "reader_test",
		"initial_step_name": "start",
		"initial_state": "initial",
		"steps": [
			{
				"name": "start",
				"title": "Start",
				"order": 1,
				"actions": []
			}
		]
	}`

	reader := strings.NewReader(jsonContent)
	workflow, err := LoadWorkflowFromReader(reader)
	require.NoError(t, err)
	assert.Equal(t, "reader_test", workflow.WorkflowType)

	// Test with invalid JSON
	invalidReader := strings.NewReader(`{invalid}`)
	workflow, err = LoadWorkflowFromReader(invalidReader)
	assert.Error(t, err)
	assert.Nil(t, workflow)
}

// TestLoadAndRegisterWorkflow tests loading and registering a single workflow
func TestLoadAndRegisterWorkflow(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a valid workflow file
	jsonContent := `{
		"workflow_type": "register_test",
		"initial_step_name": "start",
		"initial_state": "initial",
		"steps": [
			{
				"name": "start",
				"title": "Start",
				"order": 1,
				"actions": []
			}
		]
	}`
	filePath := filepath.Join(tmpDir, "test.json")
	err := os.WriteFile(filePath, []byte(jsonContent), 0644)
	require.NoError(t, err)

	// Clear registry before test
	workflowRegistry = make(map[string]WorkflowDefinition)

	// Test loading and registering
	err = LoadAndRegisterWorkflow(filePath)
	require.NoError(t, err)

	// Verify workflow was registered
	wf, err := GetWorkflow("register_test")
	require.NoError(t, err)
	assert.Equal(t, "register_test", wf.GetType())

	// Test with invalid file path
	err = LoadAndRegisterWorkflow("/nonexistent/file.json")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to read")
}

// TestWorkflowLoaderSetBasePath tests the SetBasePath and GetBasePath methods
func TestWorkflowLoaderSetBasePath(t *testing.T) {
	tmpDir := t.TempDir()

	loader := NewWorkflowLoader(tmpDir)
	assert.Equal(t, tmpDir, loader.GetBasePath())

	// Create a workflow file
	jsonContent := `{
		"workflow_type": "basepath_test",
		"initial_step_name": "start",
		"initial_state": "initial",
		"steps": [
			{
				"name": "start",
				"title": "Start",
				"order": 1,
				"actions": []
			}
		]
	}`
	err := os.WriteFile(filepath.Join(tmpDir, "test.json"), []byte(jsonContent), 0644)
	require.NoError(t, err)

	// Test loading with initial base path
	workflow, err := loader.LoadFromFile("test.json")
	require.NoError(t, err)
	assert.Equal(t, "basepath_test", workflow.WorkflowType)

	// Update base path
	newTmpDir := t.TempDir()
	err = os.WriteFile(filepath.Join(newTmpDir, "new_test.json"), []byte(jsonContent), 0644)
	require.NoError(t, err)

	loader.SetBasePath(newTmpDir)
	assert.Equal(t, newTmpDir, loader.GetBasePath())

	// Test loading with new base path
	workflow, err = loader.LoadFromFile("new_test.json")
	require.NoError(t, err)
	assert.NotNil(t, workflow)
}

// TestWorkflowLoaderLoadAll tests the LoadAll method
func TestWorkflowLoaderLoadAll(t *testing.T) {
	tmpDir := t.TempDir()

	// Create first workflow file
	workflow1 := `{
		"workflow_type": "loader_workflow1",
		"initial_step_name": "step1",
		"initial_state": "pending",
		"steps": [
			{
				"name": "step1",
				"title": "Step 1",
				"order": 1,
				"actions": []
			}
		]
	}`
	err := os.WriteFile(filepath.Join(tmpDir, "workflow1.json"), []byte(workflow1), 0644)
	require.NoError(t, err)

	// Create second workflow file
	workflow2 := `{
		"workflow_type": "loader_workflow2",
		"initial_step_name": "begin",
		"initial_state": "new",
		"steps": [
			{
				"name": "begin",
				"title": "Begin",
				"order": 1,
				"actions": []
			}
		]
	}`
	err = os.WriteFile(filepath.Join(tmpDir, "workflow2.json"), []byte(workflow2), 0644)
	require.NoError(t, err)

	// Clear registry
	workflowRegistry = make(map[string]WorkflowDefinition)

	// Test LoadAll
	loader := NewWorkflowLoader(tmpDir)
	err = loader.LoadAll()
	require.NoError(t, err)

	// Verify both workflows were registered
	wf1, err := GetWorkflow("loader_workflow1")
	require.NoError(t, err)
	assert.Equal(t, "loader_workflow1", wf1.GetType())

	wf2, err := GetWorkflow("loader_workflow2")
	require.NoError(t, err)
	assert.Equal(t, "loader_workflow2", wf2.GetType())
}

// TestValidateWorkflowDefinitionCircularReference tests detection of circular step references
func TestValidateWorkflowDefinitionCircularReference(t *testing.T) {
	// Test circular reference: step1 -> step2 -> step1
	workflow := &DynamicWorkflow{
		WorkflowType:    "circular_test",
		InitialStepName: "step1",
		Steps: map[string]*StepDefinition{
			"step1": {
				Name: "step1",
				Actions: []Action{{
					Name:     "NEXT",
					NextStep: "step2",
				}},
			},
			"step2": {
				Name: "step2",
				Actions: []Action{{
					Name:     "BACK",
					NextStep: "step1",
				}},
			},
		},
	}

	// The current validator doesn't check for circular references,
	// but it should validate that all referenced steps exist
	err := ValidateWorkflowDefinition(workflow)
	assert.NoError(t, err) // Both steps exist, so this passes
}

// TestDynamicWorkflowToJSON tests serializing workflow to JSON
func TestDynamicWorkflowToJSON(t *testing.T) {
	workflow := &DynamicWorkflow{
		WorkflowType:    "serialization_test",
		InitialStepName: "step1",
		InitialState:    "pending",
		Steps: map[string]*StepDefinition{
			"step1": {
				Name:    "step1",
				Title:   "Step 1",
				Order:   1,
				Actions: []Action{{Name: "NEXT", NextStep: "step2"}},
			},
			"step2": {
				Name:    "step2",
				Title:   "Step 2",
				Order:   2,
				Actions: []Action{},
			},
		},
	}

	// Test ToJSON
	jsonBytes, err := workflow.ToJSON()
	require.NoError(t, err)
	assert.NotNil(t, jsonBytes)

	// Verify the JSON is valid by unmarshaling into a generic map
	var result map[string]interface{}
	err = json.Unmarshal(jsonBytes, &result)
	require.NoError(t, err)
	assert.Equal(t, "serialization_test", result["workflow_type"])
	assert.Equal(t, "step1", result["initial_step_name"])
	assert.Equal(t, "pending", result["initial_state"])

	// Verify steps are serialized as a map
	steps, ok := result["steps"].(map[string]interface{})
	require.True(t, ok)
	assert.Len(t, steps, 2)

	// Test ToJSONString
	jsonStr, err := workflow.ToJSONString()
	require.NoError(t, err)
	assert.NotEmpty(t, jsonStr)
	assert.Contains(t, jsonStr, "serialization_test")
	assert.Contains(t, jsonStr, "step1")
}

// BenchmarkLoadWorkflowFromFile benchmarks loading workflow from file
func BenchmarkLoadWorkflowFromFile(b *testing.B) {
	tmpDir := b.TempDir()

	jsonContent := `{
		"workflow_type": "benchmark_workflow",
		"initial_step_name": "start",
		"initial_state": "initial",
		"steps": [
			{
				"name": "start",
				"title": "Start",
				"order": 1,
				"actions": [
					{"name": "NEXT", "next_step": "step2", "new_state": "processing"}
				]
			},
			{
				"name": "step2",
				"title": "Step 2",
				"order": 2,
				"actions": [
					{"name": "COMPLETE", "next_step": "done", "new_state": "completed"}
				]
			},
			{
				"name": "done",
				"title": "Complete",
				"order": 3,
				"actions": []
			}
		]
	}`

	filePath := filepath.Join(tmpDir, "benchmark.json")
	err := os.WriteFile(filePath, []byte(jsonContent), 0644)
	require.NoError(b, err)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := LoadWorkflowFromFile(filePath)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkParseWorkflowFromJSON benchmarks parsing workflow JSON
func BenchmarkParseWorkflowFromJSON(b *testing.B) {
	jsonContent := []byte(`{
		"workflow_type": "benchmark_workflow",
		"initial_step_name": "start",
		"initial_state": "initial",
		"steps": [
			{
				"name": "start",
				"title": "Start",
				"order": 1,
				"actions": [
					{"name": "NEXT", "next_step": "step2", "new_state": "processing"}
				]
			},
			{
				"name": "step2",
				"title": "Step 2",
				"order": 2,
				"actions": [
					{"name": "COMPLETE", "next_step": "done", "new_state": "completed"}
				]
			},
			{
				"name": "done",
				"title": "Complete",
				"order": 3,
				"actions": []
			}
		]
	}`)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := ParseWorkflowFromJSON(jsonContent)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkValidateWorkflowDefinition benchmarks workflow validation
func BenchmarkValidateWorkflowDefinition(b *testing.B) {
	workflow := &DynamicWorkflow{
		WorkflowType:    "benchmark_validation",
		InitialStepName: "step1",
		InitialState:    "pending",
		Steps: map[string]*StepDefinition{
			"step1": {
				Name:    "step1",
				Title:   "Step 1",
				Order:   1,
				Actions: []Action{{Name: "NEXT", NextStep: "step2"}},
			},
			"step2": {
				Name:    "step2",
				Title:   "Step 2",
				Order:   2,
				Actions: []Action{{Name: "NEXT", NextStep: "step3"}},
			},
			"step3": {
				Name:    "step3",
				Title:   "Step 3",
				Order:   3,
				Actions: []Action{},
			},
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		err := ValidateWorkflowDefinition(workflow)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkToJSON benchmarks workflow serialization
func BenchmarkToJSON(b *testing.B) {
	workflow := &DynamicWorkflow{
		WorkflowType:    "benchmark_serialization",
		InitialStepName: "step1",
		InitialState:    "pending",
		Steps: map[string]*StepDefinition{
			"step1": {
				Name:    "step1",
				Title:   "Step 1",
				Order:   1,
				Actions: []Action{{Name: "NEXT", NextStep: "step2"}},
			},
			"step2": {
				Name:    "step2",
				Title:   "Step 2",
				Order:   2,
				Actions: []Action{},
			},
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := workflow.ToJSON()
		if err != nil {
			b.Fatal(err)
		}
	}
}

