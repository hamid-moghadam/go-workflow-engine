package engine

import (
	"encoding/json"
	"time"
)

// WorkflowInstance represents a running instance of a workflow
type WorkflowInstance struct {
	ID           int64     `json:"id"`
	WorkflowType string    `json:"workflow_type"`
	CurrentStep  string    `json:"current_step"`
	CurrentState string    `json:"current_state"`
	UserID       int64     `json:"user_id"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
	FinishedAt   *time.Time `json:"finished_at,omitempty"`
}

// IsFinished returns true if the workflow instance has finished
func (wi *WorkflowInstance) IsFinished() bool {
	return wi.FinishedAt != nil
}

// Transition represents a user's interaction with a specific workflow step
type Transition struct {
	ID         int64           `json:"id"`
	InstanceID int64           `json:"instance_id"`
	StepName   string          `json:"step_name"`
	ActionName string          `json:"action_name"`
	StateName  string          `json:"state_name"`
	InputData  json.RawMessage `json:"input_data,omitempty"`
	UserID     int64           `json:"user_id"`
	CreatedAt  time.Time       `json:"created_at"`
	UpdatedAt  time.Time       `json:"updated_at"`
}

// SetInputData stores input data as JSON
func (t *Transition) SetInputData(data map[string]interface{}) error {
	if data == nil {
		t.InputData = nil
		return nil
	}
	bytes, err := json.Marshal(data)
	if err != nil {
		return err
	}
	t.InputData = bytes
	return nil
}

// GetInputData retrieves input data as a map
func (t *Transition) GetInputData() (map[string]interface{}, error) {
	if t.InputData == nil {
		return nil, nil
	}
	var data map[string]interface{}
	if err := json.Unmarshal(t.InputData, &data); err != nil {
		return nil, err
	}
	return data, nil
}

// TransitionHistory tracks changes made during transitions
type TransitionHistory struct {
	ID           int64           `json:"id"`
	TransitionID int64           `json:"transition_id"`
	FieldName    string          `json:"field_name"`
	OldValue     json.RawMessage `json:"old_value,omitempty"`
	NewValue     json.RawMessage `json:"new_value,omitempty"`
	CreatedAt    time.Time       `json:"created_at"`
}

// SetOldValue stores the old value as JSON
func (th *TransitionHistory) SetOldValue(value interface{}) error {
	if value == nil {
		th.OldValue = nil
		return nil
	}
	bytes, err := json.Marshal(value)
	if err != nil {
		return err
	}
	th.OldValue = bytes
	return nil
}

// SetNewValue stores the new value as JSON
func (th *TransitionHistory) SetNewValue(value interface{}) error {
	if value == nil {
		th.NewValue = nil
		return nil
	}
	bytes, err := json.Marshal(value)
	if err != nil {
		return err
	}
	th.NewValue = bytes
	return nil
}

// GetOldValue retrieves the old value
func (th *TransitionHistory) GetOldValue() (interface{}, error) {
	if th.OldValue == nil {
		return nil, nil
	}
	var value interface{}
	if err := json.Unmarshal(th.OldValue, &value); err != nil {
		return nil, err
	}
	return value, nil
}

// GetNewValue retrieves the new value
func (th *TransitionHistory) GetNewValue() (interface{}, error) {
	if th.NewValue == nil {
		return nil, nil
	}
	var value interface{}
	if err := json.Unmarshal(th.NewValue, &value); err != nil {
		return nil, err
	}
	return value, nil
}
