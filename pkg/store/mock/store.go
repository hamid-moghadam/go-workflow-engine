package mock

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/hamid-moghadam/go-workflow-engine/pkg/engine"
	"github.com/hamid-moghadam/go-workflow-engine/pkg/store"
)

// InMemoryStore is a thread-safe in-memory implementation of store.Store
// It is useful for testing and development scenarios where a real database is not needed
type InMemoryStore struct {
	mu                sync.RWMutex
	instances         map[int64]*engine.WorkflowInstance
	transitions       map[int64]*engine.Transition
	transitionHistory map[int64]*engine.TransitionHistory
	nextInstanceID    int64
	nextTransitionID  int64
	nextHistoryID     int64
}

// NewInMemoryStore creates a new in-memory store
func NewInMemoryStore() *InMemoryStore {
	return &InMemoryStore{
		instances:         make(map[int64]*engine.WorkflowInstance),
		transitions:       make(map[int64]*engine.Transition),
		transitionHistory: make(map[int64]*engine.TransitionHistory),
		nextInstanceID:    1,
		nextTransitionID: 1,
		nextHistoryID:     1,
	}
}

// Reset clears all data from the store
func (s *InMemoryStore) Reset() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.instances = make(map[int64]*engine.WorkflowInstance)
	s.transitions = make(map[int64]*engine.Transition)
	s.transitionHistory = make(map[int64]*engine.TransitionHistory)
	s.nextInstanceID = 1
	s.nextTransitionID = 1
	s.nextHistoryID = 1
}

// GetInstanceCount returns the number of stored instances
func (s *InMemoryStore) GetInstanceCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.instances)
}

// GetTransitionCount returns the number of stored transitions
func (s *InMemoryStore) GetTransitionCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.transitions)
}

// GetHistoryCount returns the number of stored history entries
func (s *InMemoryStore) GetHistoryCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.transitionHistory)
}

// CreateInstance creates a new workflow instance
func (s *InMemoryStore) CreateInstance(ctx context.Context, instance *engine.WorkflowInstance) error {
	if instance == nil {
		return fmt.Errorf("instance cannot be nil")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Check for duplicate instance for the same user and workflow type
	for _, existing := range s.instances {
		if existing.UserID == instance.UserID &&
			existing.WorkflowType == instance.WorkflowType &&
			!existing.IsFinished() {
			return store.ErrDuplicateInstance
		}
	}

	// Assign ID
	instance.ID = s.nextInstanceID
	s.nextInstanceID++

	// Set timestamps if not already set
	if instance.CreatedAt.IsZero() {
		instance.CreatedAt = time.Now()
	}
	if instance.UpdatedAt.IsZero() {
		instance.UpdatedAt = time.Now()
	}

	s.instances[instance.ID] = instance
	return nil
}

// GetInstance retrieves a workflow instance by user ID and workflow type
func (s *InMemoryStore) GetInstance(ctx context.Context, userID int64, workflowType string) (*engine.WorkflowInstance, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if userID == 0 {
		return nil, fmt.Errorf("userID cannot be 0")
	}
	if workflowType == "" {
		return nil, fmt.Errorf("workflowType cannot be empty")
	}

	for _, instance := range s.instances {
		if instance.UserID == userID &&
			instance.WorkflowType == workflowType &&
			!instance.IsFinished() {
			// Return a copy to prevent external modification
			return s.copyInstance(instance), nil
		}
	}

	return nil, store.ErrInstanceNotFound
}

// GetInstanceByID retrieves a workflow instance by its ID
func (s *InMemoryStore) GetInstanceByID(ctx context.Context, id int64) (*engine.WorkflowInstance, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if id == 0 {
		return nil, fmt.Errorf("id cannot be 0")
	}

	instance, exists := s.instances[id]
	if !exists {
		return nil, store.ErrInstanceNotFound
	}

	return s.copyInstance(instance), nil
}

// UpdateInstance updates an existing workflow instance
func (s *InMemoryStore) UpdateInstance(ctx context.Context, instance *engine.WorkflowInstance) error {
	if instance == nil {
		return fmt.Errorf("instance cannot be nil")
	}
	if instance.ID == 0 {
		return fmt.Errorf("instance ID cannot be 0")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.instances[instance.ID]; !exists {
		return store.ErrInstanceNotFound
	}

	instance.UpdatedAt = time.Now()
	s.instances[instance.ID] = s.copyInstance(instance)
	return nil
}

// CreateTransition creates a new transition record
func (s *InMemoryStore) CreateTransition(ctx context.Context, transition *engine.Transition) error {
	if transition == nil {
		return fmt.Errorf("transition cannot be nil")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Verify instance exists
	if _, exists := s.instances[transition.InstanceID]; !exists {
		return fmt.Errorf("instance %d not found", transition.InstanceID)
	}

	// Assign ID
	transition.ID = s.nextTransitionID
	s.nextTransitionID++

	// Set timestamps
	if transition.CreatedAt.IsZero() {
		transition.CreatedAt = time.Now()
	}
	if transition.UpdatedAt.IsZero() {
		transition.UpdatedAt = time.Now()
	}

	s.transitions[transition.ID] = s.copyTransition(transition)
	return nil
}

// UpdateTransition updates an existing transition record
func (s *InMemoryStore) UpdateTransition(ctx context.Context, transition *engine.Transition) error {
	if transition == nil {
		return fmt.Errorf("transition cannot be nil")
	}
	if transition.ID == 0 {
		return fmt.Errorf("transition ID cannot be 0")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.transitions[transition.ID]; !exists {
		return store.ErrTransitionNotFound
	}

	transition.UpdatedAt = time.Now()
	s.transitions[transition.ID] = s.copyTransition(transition)
	return nil
}

// GetTransition retrieves a transition by instance ID, step name, and action name
func (s *InMemoryStore) GetTransition(ctx context.Context, instanceID int64, stepName, actionName string) (*engine.Transition, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if instanceID == 0 {
		return nil, fmt.Errorf("instanceID cannot be 0")
	}
	if stepName == "" {
		return nil, fmt.Errorf("stepName cannot be empty")
	}

	for _, transition := range s.transitions {
		if transition.InstanceID == instanceID &&
			transition.StepName == stepName &&
			(actionName == "" || transition.ActionName == actionName) {
			return s.copyTransition(transition), nil
		}
	}

	return nil, store.ErrTransitionNotFound
}

// GetTransitionByID retrieves a transition by its ID
func (s *InMemoryStore) GetTransitionByID(ctx context.Context, id int64) (*engine.Transition, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if id == 0 {
		return nil, fmt.Errorf("id cannot be 0")
	}

	transition, exists := s.transitions[id]
	if !exists {
		return nil, store.ErrTransitionNotFound
	}

	return s.copyTransition(transition), nil
}

// CreateTransitionHistory creates a new transition history record
func (s *InMemoryStore) CreateTransitionHistory(ctx context.Context, history *engine.TransitionHistory) error {
	if history == nil {
		return fmt.Errorf("history cannot be nil")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Verify transition exists
	if _, exists := s.transitions[history.TransitionID]; !exists {
		return fmt.Errorf("transition %d not found", history.TransitionID)
	}

	// Assign ID
	history.ID = s.nextHistoryID
	s.nextHistoryID++

	// Set timestamp
	if history.CreatedAt.IsZero() {
		history.CreatedAt = time.Now()
	}

	s.transitionHistory[history.ID] = s.copyHistory(history)
	return nil
}

// ListInstances retrieves workflow instances based on filter criteria
func (s *InMemoryStore) ListInstances(ctx context.Context, filter store.InstanceFilter) ([]engine.WorkflowInstance, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var results []engine.WorkflowInstance

	for _, instance := range s.instances {
		// Apply filters
		if filter.WorkflowType != "" && instance.WorkflowType != filter.WorkflowType {
			continue
		}
		if filter.UserID != nil && instance.UserID != *filter.UserID {
			continue
		}
		if filter.CurrentStep != "" && instance.CurrentStep != filter.CurrentStep {
			continue
		}
		if filter.CurrentState != "" && instance.CurrentState != filter.CurrentState {
			continue
		}
		if filter.IsFinished != nil {
			if *filter.IsFinished != instance.IsFinished() {
				continue
			}
		}
		if filter.CreatedAfter != nil && instance.CreatedAt.Before(*filter.CreatedAfter) {
			continue
		}
		if filter.CreatedBefore != nil && instance.CreatedAt.After(*filter.CreatedBefore) {
			continue
		}

		results = append(results, *s.copyInstance(instance))
	}

	// Sort by created_at DESC
	for i, j := 0, len(results)-1; i < j; i, j = i+1, j-1 {
		results[i], results[j] = results[j], results[i]
	}

	// Apply pagination
	if filter.Offset > 0 && filter.Offset < len(results) {
		results = results[filter.Offset:]
	} else if filter.Offset >= len(results) {
		results = []engine.WorkflowInstance{}
	}

	if filter.Limit > 0 && filter.Limit < len(results) {
		results = results[:filter.Limit]
	}

	return results, nil
}

// ListTransitions retrieves all transitions for a workflow instance
func (s *InMemoryStore) ListTransitions(ctx context.Context, instanceID int64) ([]engine.Transition, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if instanceID == 0 {
		return nil, fmt.Errorf("instanceID cannot be 0")
	}

	var results []engine.Transition
	for _, transition := range s.transitions {
		if transition.InstanceID == instanceID {
			results = append(results, *s.copyTransition(transition))
		}
	}

	return results, nil
}

// ListTransitionHistory retrieves history for a specific transition
func (s *InMemoryStore) ListTransitionHistory(ctx context.Context, transitionID int64) ([]engine.TransitionHistory, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if transitionID == 0 {
		return nil, fmt.Errorf("transitionID cannot be 0")
	}

	var results []engine.TransitionHistory
	for _, history := range s.transitionHistory {
		if history.TransitionID == transitionID {
			results = append(results, *s.copyHistory(history))
		}
	}

	return results, nil
}

// Transaction executes a function within a transaction
// For in-memory store, this is a no-op since operations are atomic
func (s *InMemoryStore) Transaction(ctx context.Context, fn func(tx store.Store) error) error {
	// Create a transactional store that shares the same data but with locking
	txStore := &inMemoryTxStore{
		parent: s,
	}
	return fn(txStore)
}

// Close closes the store (no-op for in-memory)
func (s *InMemoryStore) Close() error {
	return nil
}

// Helper methods for copying structs to prevent external modification

func (s *InMemoryStore) copyInstance(original *engine.WorkflowInstance) *engine.WorkflowInstance {
	return &engine.WorkflowInstance{
		ID:           original.ID,
		WorkflowType: original.WorkflowType,
		CurrentStep:    original.CurrentStep,
		CurrentState:   original.CurrentState,
		UserID:         original.UserID,
		CreatedAt:      original.CreatedAt,
		UpdatedAt:      original.UpdatedAt,
		FinishedAt:     original.FinishedAt,
	}
}

func (s *InMemoryStore) copyTransition(original *engine.Transition) *engine.Transition {
	return &engine.Transition{
		ID:         original.ID,
		InstanceID: original.InstanceID,
		StepName:   original.StepName,
		ActionName: original.ActionName,
		StateName:  original.StateName,
		InputData:  append([]byte(nil), original.InputData...),
		UserID:     original.UserID,
		CreatedAt:  original.CreatedAt,
		UpdatedAt:  original.UpdatedAt,
	}
}

func (s *InMemoryStore) copyHistory(original *engine.TransitionHistory) *engine.TransitionHistory {
	return &engine.TransitionHistory{
		ID:           original.ID,
		TransitionID: original.TransitionID,
		FieldName:    original.FieldName,
		OldValue:     append([]byte(nil), original.OldValue...),
		NewValue:     append([]byte(nil), original.NewValue...),
		CreatedAt:    original.CreatedAt,
	}
}

// inMemoryTxStore is a transactional view of the in-memory store
// It provides the same interface but delegates to the parent store with proper locking
type inMemoryTxStore struct {
	parent *InMemoryStore
}

func (tx *inMemoryTxStore) CreateInstance(ctx context.Context, instance *engine.WorkflowInstance) error {
	return tx.parent.CreateInstance(ctx, instance)
}

func (tx *inMemoryTxStore) GetInstance(ctx context.Context, userID int64, workflowType string) (*engine.WorkflowInstance, error) {
	return tx.parent.GetInstance(ctx, userID, workflowType)
}

func (tx *inMemoryTxStore) GetInstanceByID(ctx context.Context, id int64) (*engine.WorkflowInstance, error) {
	return tx.parent.GetInstanceByID(ctx, id)
}

func (tx *inMemoryTxStore) UpdateInstance(ctx context.Context, instance *engine.WorkflowInstance) error {
	return tx.parent.UpdateInstance(ctx, instance)
}

func (tx *inMemoryTxStore) CreateTransition(ctx context.Context, transition *engine.Transition) error {
	return tx.parent.CreateTransition(ctx, transition)
}

func (tx *inMemoryTxStore) UpdateTransition(ctx context.Context, transition *engine.Transition) error {
	return tx.parent.UpdateTransition(ctx, transition)
}

func (tx *inMemoryTxStore) GetTransition(ctx context.Context, instanceID int64, stepName, actionName string) (*engine.Transition, error) {
	return tx.parent.GetTransition(ctx, instanceID, stepName, actionName)
}

func (tx *inMemoryTxStore) GetTransitionByID(ctx context.Context, id int64) (*engine.Transition, error) {
	return tx.parent.GetTransitionByID(ctx, id)
}

func (tx *inMemoryTxStore) CreateTransitionHistory(ctx context.Context, history *engine.TransitionHistory) error {
	return tx.parent.CreateTransitionHistory(ctx, history)
}

func (tx *inMemoryTxStore) ListInstances(ctx context.Context, filter store.InstanceFilter) ([]engine.WorkflowInstance, error) {
	return tx.parent.ListInstances(ctx, filter)
}

func (tx *inMemoryTxStore) ListTransitions(ctx context.Context, instanceID int64) ([]engine.Transition, error) {
	return tx.parent.ListTransitions(ctx, instanceID)
}

func (tx *inMemoryTxStore) ListTransitionHistory(ctx context.Context, transitionID int64) ([]engine.TransitionHistory, error) {
	return tx.parent.ListTransitionHistory(ctx, transitionID)
}

func (tx *inMemoryTxStore) Transaction(ctx context.Context, fn func(tx store.Store) error) error {
	return fn(tx)
}

func (tx *inMemoryTxStore) Close() error {
	return nil
}
