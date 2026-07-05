package gormstore

import (
	"encoding/json"
	"time"

	"github.com/hamid-moghadam/go-workflow-engine/pkg/engine"
)

// WorkflowInstance represents a running instance of a workflow in the database
type WorkflowInstance struct {
	ID           int64           `gorm:"primaryKey;column:id" json:"id"`
	WorkflowType string          `gorm:"column:workflow_type;type:varchar(255);not null;index:idx_workflow_instances_type,index:idx_workflow_instances_user_type" json:"workflow_type"`
	CurrentStep  string          `gorm:"column:current_step;type:varchar(255);not null" json:"current_step"`
	CurrentState string          `gorm:"column:current_state;type:varchar(255);not null;index:idx_workflow_instances_state" json:"current_state"`
	UserID       int64           `gorm:"column:user_id;not null;index:idx_workflow_instances_user,index:idx_workflow_instances_user_type" json:"user_id"`
	CreatedAt    time.Time       `gorm:"column:created_at;autoCreateTime" json:"created_at"`
	UpdatedAt    time.Time       `gorm:"column:updated_at;autoUpdateTime" json:"updated_at"`
	FinishedAt   *time.Time      `gorm:"column:finished_at;index:idx_workflow_instances_finished" json:"finished_at,omitempty"`

	// Associations
	Transitions []Transition `gorm:"foreignKey:InstanceID;references:ID" json:"transitions,omitempty"`
}

// TableName returns the table name for WorkflowInstance
func (WorkflowInstance) TableName() string {
	return "workflow_instances"
}

// Transition represents a user's interaction with a specific workflow step
type Transition struct {
	ID         int64           `gorm:"primaryKey;column:id" json:"id"`
	InstanceID int64           `gorm:"column:instance_id;not null;index:idx_transitions_instance,index:idx_transitions_instance_step_action" json:"instance_id"`
	StepName   string          `gorm:"column:step_name;type:varchar(255);not null;index:idx_transitions_step,index:idx_transitions_instance_step_action" json:"step_name"`
	ActionName string          `gorm:"column:action_name;type:varchar(255);not null;index:idx_transitions_action,index:idx_transitions_instance_step_action" json:"action_name"`
	StateName  string          `gorm:"column:state_name;type:varchar(255);not null" json:"state_name"`
	InputData  JSONRawMessage  `gorm:"column:input_data;type:text" json:"input_data,omitempty"`
	UserID     int64           `gorm:"column:user_id;not null;index:idx_transitions_user" json:"user_id"`
	CreatedAt  time.Time       `gorm:"column:created_at;autoCreateTime" json:"created_at"`
	UpdatedAt  time.Time       `gorm:"column:updated_at;autoUpdateTime" json:"updated_at"`

	// Associations
	Instance   WorkflowInstance    `gorm:"foreignKey:InstanceID;references:ID" json:"instance,omitempty"`
	Histories  []TransitionHistory `gorm:"foreignKey:TransitionID;references:ID" json:"histories,omitempty"`
}

// TableName returns the table name for Transition
func (Transition) TableName() string {
	return "workflow_transitions"
}

// TransitionHistory tracks changes made during transitions
type TransitionHistory struct {
	ID           int64           `gorm:"primaryKey;column:id" json:"id"`
	TransitionID int64           `gorm:"column:transition_id;not null;index:idx_history_transition" json:"transition_id"`
	FieldName    string          `gorm:"column:field_name;type:varchar(255);not null" json:"field_name"`
	OldValue     JSONRawMessage  `gorm:"column:old_value;type:text" json:"old_value,omitempty"`
	NewValue     JSONRawMessage  `gorm:"column:new_value;type:text" json:"new_value,omitempty"`
	CreatedAt    time.Time       `gorm:"column:created_at;autoCreateTime" json:"created_at"`

	// Associations
	Transition Transition `gorm:"foreignKey:TransitionID;references:ID" json:"transition,omitempty"`
}

// TableName returns the table name for TransitionHistory
func (TransitionHistory) TableName() string {
	return "transition_history"
}

// ToEngineInstance converts a GORM WorkflowInstance to an engine.WorkflowInstance
func (wi *WorkflowInstance) ToEngineInstance() *engine.WorkflowInstance {
	return &engine.WorkflowInstance{
		ID:           wi.ID,
		WorkflowType: wi.WorkflowType,
		CurrentStep:  wi.CurrentStep,
		CurrentState: wi.CurrentState,
		UserID:       wi.UserID,
		CreatedAt:    wi.CreatedAt,
		UpdatedAt:    wi.UpdatedAt,
		FinishedAt:   wi.FinishedAt,
	}
}

// FromEngineInstance creates a GORM WorkflowInstance from an engine.WorkflowInstance
func WorkflowInstanceFromEngine(ei *engine.WorkflowInstance) *WorkflowInstance {
	return &WorkflowInstance{
		ID:           ei.ID,
		WorkflowType: ei.WorkflowType,
		CurrentStep:  ei.CurrentStep,
		CurrentState: ei.CurrentState,
		UserID:       ei.UserID,
		CreatedAt:    ei.CreatedAt,
		UpdatedAt:    ei.UpdatedAt,
		FinishedAt:   ei.FinishedAt,
	}
}

// ToEngineTransition converts a GORM Transition to an engine.Transition
func (t *Transition) ToEngineTransition() *engine.Transition {
	return &engine.Transition{
		ID:         t.ID,
		InstanceID: t.InstanceID,
		StepName:   t.StepName,
		ActionName: t.ActionName,
		StateName:  t.StateName,
		InputData:  json.RawMessage(t.InputData),
		UserID:     t.UserID,
		CreatedAt:  t.CreatedAt,
		UpdatedAt:  t.UpdatedAt,
	}
}

// FromEngineTransition creates a GORM Transition from an engine.Transition
func TransitionFromEngine(et *engine.Transition) *Transition {
	inputData := JSONRawMessage(et.InputData)
	if len(inputData) == 0 {
		inputData = JSONRawMessage(`{}`)
	}
	return &Transition{
		ID:         et.ID,
		InstanceID: et.InstanceID,
		StepName:   et.StepName,
		ActionName: et.ActionName,
		StateName:  et.StateName,
		InputData:  inputData,
		UserID:     et.UserID,
		CreatedAt:  et.CreatedAt,
		UpdatedAt:  et.UpdatedAt,
	}
}

// ToEngineHistory converts a GORM TransitionHistory to an engine.TransitionHistory
func (th *TransitionHistory) ToEngineHistory() *engine.TransitionHistory {
	return &engine.TransitionHistory{
		ID:           th.ID,
		TransitionID: th.TransitionID,
		FieldName:    th.FieldName,
		OldValue:     json.RawMessage(th.OldValue),
		NewValue:     json.RawMessage(th.NewValue),
		CreatedAt:    th.CreatedAt,
	}
}

// FromEngineHistory creates a GORM TransitionHistory from an engine.TransitionHistory
func TransitionHistoryFromEngine(eth *engine.TransitionHistory) *TransitionHistory {
	oldVal := JSONRawMessage(eth.OldValue)
	newVal := JSONRawMessage(eth.NewValue)
	if len(oldVal) == 0 {
		oldVal = JSONRawMessage(`{}`)
	}
	if len(newVal) == 0 {
		newVal = JSONRawMessage(`{}`)
	}
	return &TransitionHistory{
		ID:           eth.ID,
		TransitionID: eth.TransitionID,
		FieldName:    eth.FieldName,
		OldValue:     oldVal,
		NewValue:     newVal,
		CreatedAt:    eth.CreatedAt,
	}
}
