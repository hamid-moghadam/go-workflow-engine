package context

import (
	"context"
	"errors"

	"github.com/rs/zerolog"
	"gorm.io/gorm"

	"github.com/hamid-moghadam/go-workflow-engine/pkg/engine"
)

// StepContext provides the execution context for workflow step handlers
// It gives access to database, logging, workflow service, and the current workflow state
type StepContext struct {
	// Database is the GORM database connection
	// Available for database operations within the step
	Database *gorm.DB

	// WorkflowInstance is the current workflow instance being processed
	WorkflowInstance *engine.WorkflowInstance

	// Logger provides structured logging for the step
	Logger zerolog.Logger

	// Service provides access to the workflow service for operations
	// like transitioning to other steps
	Service WorkflowService

	// BaseContext is the underlying context.Context for cancellation and timeouts
	BaseContext context.Context

	// CustomData allows users to store custom dependencies and data
	// Use this to attach application-specific objects (e.g., HTTP clients, config, etc.)
	CustomData map[string]interface{}
}

// WorkflowService defines the interface for workflow operations
// This is implemented by the workflow service and made available to step handlers
type WorkflowService interface {
	// TransitionStep executes a transition for the given instance and action
	TransitionStep(ctx *StepContext, instanceID int64, actionName string, inputData map[string]interface{}) error

	// GetInstance retrieves a workflow instance by ID
	GetInstance(instanceID int64) (*engine.WorkflowInstance, error)

	// CreateInstance creates a new workflow instance
	CreateInstance(workflowType string, userID int64, initialData map[string]interface{}) (*engine.WorkflowInstance, error)
}

// NewStepContext creates a new StepContext with the provided dependencies
func NewStepContext(db *gorm.DB, instance *engine.WorkflowInstance, logger zerolog.Logger, service WorkflowService) *StepContext {
	return &StepContext{
		Database:         db,
		WorkflowInstance: instance,
		Logger:           logger,
		Service:          service,
		BaseContext:      context.Background(),
		CustomData:       make(map[string]interface{}),
	}
}

// NewStepContextWithContext creates a StepContext with a custom base context
func NewStepContextWithContext(ctx context.Context, db *gorm.DB, instance *engine.WorkflowInstance, logger zerolog.Logger, service WorkflowService) *StepContext {
	return &StepContext{
		Database:         db,
		WorkflowInstance: instance,
		Logger:           logger,
		Service:          service,
		BaseContext:      ctx,
		CustomData:       make(map[string]interface{}),
	}
}

// SetCustomData stores a custom value in the context
func (c *StepContext) SetCustomData(key string, value interface{}) {
	if c.CustomData == nil {
		c.CustomData = make(map[string]interface{})
	}
	c.CustomData[key] = value
}

// GetCustomData retrieves a custom value from the context
// Returns nil and an error if the key is not found
func (c *StepContext) GetCustomData(key string) (interface{}, error) {
	if c.CustomData == nil {
		return nil, errors.New("no custom data available in context")
	}
	value, exists := c.CustomData[key]
	if !exists {
		return nil, errors.New("key not found in custom data")
	}
	return value, nil
}

// MustGetCustomData retrieves a custom value or panics if not found
// Use with caution - only for keys that are guaranteed to exist
func (c *StepContext) MustGetCustomData(key string) interface{} {
	value, err := c.GetCustomData(key)
	if err != nil {
		panic(err)
	}
	return value
}

// GetTypedCustomData retrieves and type-asserts a custom value
// Returns an error if the key is not found or if the type assertion fails
func GetTypedCustomData[T any](c *StepContext, key string) (T, error) {
	var result T
	value, err := c.GetCustomData(key)
	if err != nil {
		return result, err
	}
	typed, ok := value.(T)
	if !ok {
		return result, errors.New("type assertion failed for custom data key")
	}
	return typed, nil
}

// WithContext returns a new StepContext with an updated base context
func (c *StepContext) WithContext(ctx context.Context) *StepContext {
	newCtx := *c
	newCtx.BaseContext = ctx
	return &newCtx
}

// Done returns the done channel of the base context
func (c *StepContext) Done() <-chan struct{} {
	return c.BaseContext.Done()
}

// Err returns any error from the base context
func (c *StepContext) Err() error {
	return c.BaseContext.Err()
}

// Value returns a value from the base context
func (c *StepContext) Value(key interface{}) interface{} {
	return c.BaseContext.Value(key)
}

// WithLogger returns a new StepContext with an updated logger
func (c *StepContext) WithLogger(logger zerolog.Logger) *StepContext {
	newCtx := *c
	newCtx.Logger = logger
	return &newCtx
}

// WithDatabase returns a new StepContext with an updated database connection
func (c *StepContext) WithDatabase(db *gorm.DB) *StepContext {
	newCtx := *c
	newCtx.Database = db
	return &newCtx
}

// WithInstance returns a new StepContext with an updated workflow instance
func (c *StepContext) WithInstance(instance *engine.WorkflowInstance) *StepContext {
	newCtx := *c
	newCtx.WorkflowInstance = instance
	return &newCtx
}

// LoggerWithInstance returns a logger with workflow instance context fields
func (c *StepContext) LoggerWithInstance() zerolog.Logger {
	if c.WorkflowInstance == nil {
		return c.Logger
	}
	return c.Logger.With().
		Int64("instance_id", c.WorkflowInstance.ID).
		Str("workflow_type", c.WorkflowInstance.WorkflowType).
		Str("current_step", c.WorkflowInstance.CurrentStep).
		Logger()
}

// IsFinished returns true if the current workflow instance has finished
func (c *StepContext) IsFinished() bool {
	if c.WorkflowInstance == nil {
		return false
	}
	return c.WorkflowInstance.IsFinished()
}

// GetCurrentStep returns the current step name of the workflow instance
func (c *StepContext) GetCurrentStep() string {
	if c.WorkflowInstance == nil {
		return ""
	}
	return c.WorkflowInstance.CurrentStep
}

// GetCurrentState returns the current state of the workflow instance
func (c *StepContext) GetCurrentState() string {
	if c.WorkflowInstance == nil {
		return ""
	}
	return c.WorkflowInstance.CurrentState
}
