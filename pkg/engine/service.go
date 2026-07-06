package engine

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/rs/zerolog"
)

// Service defines the interface for workflow operations
type Service interface {
	CreateInstance(ctx context.Context, workflowType string, userID int64, initialData map[string]interface{}) (*WorkflowInstance, error)
	GetInstance(ctx context.Context, userID int64, workflowType string) (*WorkflowInstance, error)
	GetInstanceByID(ctx context.Context, id int64) (*WorkflowInstance, error)
	GetWorkflowDefinition(workflowType string) (WorkflowDefinition, error)
	TransitionStep(ctx context.Context, instanceID int64, actionName string, inputData map[string]interface{}) error
	GetCurrentStep(ctx context.Context, instance *WorkflowInstance) (*StepDefinition, error)
	ListInstances(ctx context.Context, filter InstanceFilter) ([]WorkflowInstance, error)
	GetTransitionByID(ctx context.Context, id int64) (*Transition, error)
	ListTransitionHistory(ctx context.Context, transitionID int64) ([]TransitionHistory, error)
	EventCh() <-chan TransitionEvent
	Close() error
}

// InstanceFilter provides filtering criteria for listing instances
type InstanceFilter struct {
	WorkflowType string
	UserID       *int64
	CurrentStep  string
	CurrentState string
	IsFinished   *bool
	CreatedAfter *time.Time
	Limit        int
	Offset       int
}

// WorkflowService implements the Service interface
type WorkflowService struct {
	store     Store
	registry  *Registry
	logger    zerolog.Logger
	events    *TransitionListenerManager
	eventCh   chan TransitionEvent
	done      chan struct{}
	closeOnce sync.Once
}

// NewWorkflowService creates a new workflow service
func NewWorkflowService(store Store, registry *Registry, logger zerolog.Logger) *WorkflowService {
	svc := &WorkflowService{
		store:    store,
		registry: registry,
		logger:   logger,
		events:   NewTransitionListenerManager(),
		eventCh:  make(chan TransitionEvent, 100),
		done:     make(chan struct{}),
	}
	go svc.processEvents()
	return svc
}

// OnAfterTransition registers a listener for transitions
func (s *WorkflowService) OnAfterTransition(workflowType, toState string, listener TransitionListener) {
	s.events.OnAfterTransition(workflowType, toState, listener)
}

// OnBeforeTransition registers a listener that fires before a transition executes.
// Listeners can modify input data and return the enriched version.
// Returning an error blocks the transition.
func (s *WorkflowService) OnBeforeTransition(workflowType, actionName string, listener BeforeTransitionListener) {
	s.events.OnBeforeTransition(workflowType, actionName, listener)
}

// EventCh returns a read-only channel for consuming transition events.
func (s *WorkflowService) EventCh() <-chan TransitionEvent {
	return s.eventCh
}

// processEvents reads from the event channel and dispatches to listeners.
func (s *WorkflowService) processEvents() {
	defer close(s.done)
	for event := range s.eventCh {
		s.events.emit(event)
	}
}

// Close drains the event channel and waits for the worker goroutine to finish.
func (s *WorkflowService) Close() error {
	s.closeOnce.Do(func() {
		close(s.eventCh)
		<-s.done
	})
	return nil
}

func (s *WorkflowService) CreateInstance(ctx context.Context, workflowType string, userID int64, initialData map[string]interface{}) (*WorkflowInstance, error) {
	def, err := GetWorkflow(workflowType)
	if err != nil {
		return nil, fmt.Errorf("workflow type not found: %w", err)
	}

	instance := &WorkflowInstance{
		WorkflowType: workflowType,
		CurrentStep:  def.GetInitialStepName(),
		CurrentState: def.GetInitialState(),
		UserID:       userID,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	if err := s.store.CreateInstance(ctx, instance); err != nil {
		return nil, fmt.Errorf("failed to create instance: %w", err)
	}

	s.logger.Info().
		Int64("user_id", userID).
		Str("workflow_type", workflowType).
		Str("step", instance.CurrentStep).
		Msg("Created new workflow instance")

	return instance, nil
}

func (s *WorkflowService) GetInstance(ctx context.Context, userID int64, workflowType string) (*WorkflowInstance, error) {
	instance, err := s.store.GetInstance(ctx, userID, workflowType)
	if err != nil {
		return nil, fmt.Errorf("no active workflow instance found for user %d and type %s", userID, workflowType)
	}
	return instance, nil
}

func (s *WorkflowService) GetInstanceByID(ctx context.Context, id int64) (*WorkflowInstance, error) {
	instance, err := s.store.GetInstanceByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("workflow instance %d not found", id)
	}
	return instance, nil
}

func (s *WorkflowService) GetWorkflowDefinition(workflowType string) (WorkflowDefinition, error) {
	return GetWorkflow(workflowType)
}

func (s *WorkflowService) GetCurrentStep(ctx context.Context, instance *WorkflowInstance) (*StepDefinition, error) {
	def, err := GetWorkflow(instance.WorkflowType)
	if err != nil {
		return nil, err
	}
	return def.GetStep(instance.CurrentStep)
}

func (s *WorkflowService) TransitionStep(ctx context.Context, instanceID int64, actionName string, inputData map[string]interface{}) error {
	instance, err := s.GetInstanceByID(ctx, instanceID)
	if err != nil {
		return err
	}

	def, err := GetWorkflow(instance.WorkflowType)
	if err != nil {
		return err
	}

	step, err := def.GetStep(instance.CurrentStep)
	if err != nil {
		return err
	}

	action, err := def.GetTransitionAction(instance.CurrentStep, actionName)
	if err != nil {
		return err
	}

	fromStep := instance.CurrentStep
	fromState := instance.CurrentState

	if action.ValidationFunc != "" && s.registry.HasValidation(action.ValidationFunc) {
		validateFn, err := s.registry.GetValidation(action.ValidationFunc)
		if err != nil {
			return fmt.Errorf("validation function not found: %w", err)
		}
		if err := validateFn(inputData); err != nil {
			return fmt.Errorf("validation failed: %w", err)
		}
	}

	inputData, err = s.events.emitBefore(BeforeTransitionEvent{
		Instance:  instance,
		Action:    actionName,
		StepName:  step.Name,
		InputData: inputData,
		Logger:    s.logger,
		BaseCtx:   ctx,
	})
	if err != nil {
		return fmt.Errorf("before-transition listener failed: %w", err)
	}

	if action.NextStep != "" {
		instance.CurrentStep = action.NextStep
	}
	if action.NewState != "" {
		instance.CurrentState = action.NewState
	}
	instance.UpdatedAt = time.Now()

	if err := s.store.UpdateInstance(ctx, instance); err != nil {
		return fmt.Errorf("failed to update instance: %w", err)
	}

	transition := &Transition{
		InstanceID: instanceID,
		StepName:   step.Name,
		ActionName: action.Name,
		StateName:  instance.CurrentState,
		UserID:     instance.UserID,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}

	if inputData != nil {
		if err := transition.SetInputData(inputData); err != nil {
			return fmt.Errorf("failed to set transition input data: %w", err)
		}
	}

	if err := s.store.CreateTransition(ctx, transition); err != nil {
		return fmt.Errorf("failed to create transition: %w", err)
	}

	select {
	case s.eventCh <- TransitionEvent{
		Type:      EventAfterTransition,
		Instance:  instance,
		Action:    actionName,
		FromStep:  fromStep,
		ToStep:    instance.CurrentStep,
		FromState: fromState,
		ToState:   instance.CurrentState,
		InputData: inputData,
		Logger:    s.logger,
		BaseCtx:   ctx,
	}:
	default:
		s.logger.Warn().Msg("event channel full, dropping event")
	}

	s.logger.Info().
		Int64("instance_id", instanceID).
		Str("action", actionName).
		Str("new_step", instance.CurrentStep).
		Str("new_state", instance.CurrentState).
		Msg("Transition completed successfully")

	return nil
}

func (s *WorkflowService) ListInstances(ctx context.Context, filter InstanceFilter) ([]WorkflowInstance, error) {
	storeFilter := StoreFilter{
		WorkflowType: filter.WorkflowType,
		CurrentStep:  filter.CurrentStep,
		CurrentState: filter.CurrentState,
		Limit:        filter.Limit,
		Offset:       filter.Offset,
	}

	if filter.UserID != nil {
		storeFilter.UserID = filter.UserID
	}
	if filter.IsFinished != nil {
		storeFilter.IsFinished = filter.IsFinished
	}
	if filter.CreatedAfter != nil {
		storeFilter.CreatedAfter = filter.CreatedAfter
	}

	instances, err := s.store.ListInstances(ctx, storeFilter)
	if err != nil {
		return nil, fmt.Errorf("failed to list instances: %w", err)
	}

	return instances, nil
}

func (s *WorkflowService) FinishInstance(ctx context.Context, instanceID int64) error {
	instance, err := s.store.GetInstanceByID(ctx, instanceID)
	if err != nil {
		return fmt.Errorf("failed to get instance: %w", err)
	}

	now := time.Now()
	instance.FinishedAt = &now
	instance.UpdatedAt = now

	if err := s.store.UpdateInstance(ctx, instance); err != nil {
		return fmt.Errorf("failed to finish instance: %w", err)
	}

	s.logger.Info().
		Int64("instance_id", instanceID).
		Msg("Workflow instance marked as finished")

	return nil
}

func (s *WorkflowService) GetTransition(ctx context.Context, instanceID int64, stepName, actionName string) (*Transition, error) {
	transition, err := s.store.GetTransition(ctx, instanceID, stepName, actionName)
	if err != nil {
		return nil, fmt.Errorf("transition not found")
	}
	return transition, nil
}

func (s *WorkflowService) GetTransitionByID(ctx context.Context, id int64) (*Transition, error) {
	transition, err := s.store.GetTransitionByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("transition %d not found", id)
	}
	return transition, nil
}

func (s *WorkflowService) ListTransitionHistory(ctx context.Context, transitionID int64) ([]TransitionHistory, error) {
	histories, err := s.store.ListTransitionHistory(ctx, transitionID)
	if err != nil {
		return nil, fmt.Errorf("failed to list transition history: %w", err)
	}
	return histories, nil
}
