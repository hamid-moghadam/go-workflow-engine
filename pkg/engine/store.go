package engine

import (
	"context"
	"errors"
	"time"
)

// Common store errors
var (
	ErrInstanceNotFound   = errors.New("workflow instance not found")
	ErrTransitionNotFound = errors.New("transition not found")
	ErrDuplicateInstance  = errors.New("workflow instance already exists")
)

// StoreFilter provides filtering criteria for listing workflow instances
type StoreFilter struct {
	WorkflowType string
	UserID       *int64
	CurrentStep  string
	CurrentState string
	IsFinished   *bool
	CreatedAfter *time.Time
	CreatedBefore *time.Time
	Limit        int
	Offset       int
}

// Store defines the interface for workflow persistence
type Store interface {
	CreateInstance(ctx context.Context, instance *WorkflowInstance) error
	GetInstance(ctx context.Context, userID int64, workflowType string) (*WorkflowInstance, error)
	GetInstanceByID(ctx context.Context, id int64) (*WorkflowInstance, error)
	UpdateInstance(ctx context.Context, instance *WorkflowInstance) error
	CreateTransition(ctx context.Context, transition *Transition) error
	UpdateTransition(ctx context.Context, transition *Transition) error
	GetTransition(ctx context.Context, instanceID int64, stepName, actionName string) (*Transition, error)
	GetTransitionByID(ctx context.Context, id int64) (*Transition, error)
	CreateTransitionHistory(ctx context.Context, history *TransitionHistory) error
	ListInstances(ctx context.Context, filter StoreFilter) ([]WorkflowInstance, error)
	ListTransitions(ctx context.Context, instanceID int64) ([]Transition, error)
	ListTransitionHistory(ctx context.Context, transitionID int64) ([]TransitionHistory, error)
	Transaction(ctx context.Context, fn func(tx Store) error) error
	Close() error
}
