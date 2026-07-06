package memory

import (
	"context"
	"sync"

	"github.com/hamid-moghadam/go-workflow-engine/pkg/engine"
)

// Store implements engine.Store with in-memory storage for testing
type Store struct {
	mu          sync.RWMutex
	instances   map[int64]*engine.WorkflowInstance
	transitions map[int64]*engine.Transition
	histories   map[int64]*engine.TransitionHistory
	nextID      int64
	nextTransID int64
	nextHistID  int64
}

// New creates a new in-memory store
func New() *Store {
	return &Store{
		instances:   make(map[int64]*engine.WorkflowInstance),
		transitions: make(map[int64]*engine.Transition),
		histories:   make(map[int64]*engine.TransitionHistory),
		nextID:      1,
		nextTransID: 1,
		nextHistID:  1,
	}
}

func (s *Store) CreateInstance(ctx context.Context, instance *engine.WorkflowInstance) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	instance.ID = s.nextID
	s.nextID++
	s.instances[instance.ID] = instance
	return nil
}

func (s *Store) GetInstance(ctx context.Context, userID int64, workflowType string) (*engine.WorkflowInstance, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, inst := range s.instances {
		if inst.UserID == userID && inst.WorkflowType == workflowType && inst.FinishedAt == nil {
			return inst, nil
		}
	}
	return nil, engine.ErrInstanceNotFound
}

func (s *Store) GetInstanceByID(ctx context.Context, id int64) (*engine.WorkflowInstance, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	inst, ok := s.instances[id]
	if !ok {
		return nil, engine.ErrInstanceNotFound
	}
	return inst, nil
}

func (s *Store) UpdateInstance(ctx context.Context, instance *engine.WorkflowInstance) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.instances[instance.ID]; !ok {
		return engine.ErrInstanceNotFound
	}
	s.instances[instance.ID] = instance
	return nil
}

func (s *Store) CreateTransition(ctx context.Context, transition *engine.Transition) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	transition.ID = s.nextTransID
	s.nextTransID++
	s.transitions[transition.ID] = transition
	return nil
}

func (s *Store) UpdateTransition(ctx context.Context, transition *engine.Transition) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.transitions[transition.ID]; !ok {
		return engine.ErrTransitionNotFound
	}
	s.transitions[transition.ID] = transition
	return nil
}

func (s *Store) GetTransition(ctx context.Context, instanceID int64, stepName, actionName string) (*engine.Transition, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, t := range s.transitions {
		if t.InstanceID == instanceID && t.StepName == stepName && t.ActionName == actionName {
			return t, nil
		}
	}
	return nil, engine.ErrTransitionNotFound
}

func (s *Store) GetTransitionByID(ctx context.Context, id int64) (*engine.Transition, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	t, ok := s.transitions[id]
	if !ok {
		return nil, engine.ErrTransitionNotFound
	}
	return t, nil
}

func (s *Store) CreateTransitionHistory(ctx context.Context, history *engine.TransitionHistory) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	history.ID = s.nextHistID
	s.nextHistID++
	s.histories[history.ID] = history
	return nil
}

func (s *Store) ListInstances(ctx context.Context, filter engine.StoreFilter) ([]engine.WorkflowInstance, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []engine.WorkflowInstance
	for _, inst := range s.instances {
		if !matchesFilter(inst, filter) {
			continue
		}
		result = append(result, *inst)
	}

	// Apply offset
	if filter.Offset > 0 && filter.Offset < len(result) {
		result = result[filter.Offset:]
	} else if filter.Offset >= len(result) {
		return nil, nil
	}

	// Apply limit
	if filter.Limit > 0 && filter.Limit < len(result) {
		result = result[:filter.Limit]
	}

	return result, nil
}

func (s *Store) ListTransitions(ctx context.Context, instanceID int64) ([]engine.Transition, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []engine.Transition
	for _, t := range s.transitions {
		if t.InstanceID == instanceID {
			result = append(result, *t)
		}
	}
	return result, nil
}

func (s *Store) ListTransitionHistory(ctx context.Context, transitionID int64) ([]engine.TransitionHistory, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []engine.TransitionHistory
	for _, h := range s.histories {
		if h.TransitionID == transitionID {
			result = append(result, *h)
		}
	}
	return result, nil
}

func (s *Store) Transaction(ctx context.Context, fn func(tx engine.Store) error) error {
	return fn(s)
}

func (s *Store) Close() error {
	return nil
}

func matchesFilter(inst *engine.WorkflowInstance, filter engine.StoreFilter) bool {
	if filter.WorkflowType != "" && inst.WorkflowType != filter.WorkflowType {
		return false
	}
	if filter.UserID != nil && inst.UserID != *filter.UserID {
		return false
	}
	if filter.CurrentStep != "" && inst.CurrentStep != filter.CurrentStep {
		return false
	}
	if filter.CurrentState != "" && inst.CurrentState != filter.CurrentState {
		return false
	}
	if filter.IsFinished != nil {
		finished := inst.FinishedAt != nil
		if *filter.IsFinished != finished {
			return false
		}
	}
	if filter.CreatedAfter != nil && inst.CreatedAt.Before(*filter.CreatedAfter) {
		return false
	}
	if filter.CreatedBefore != nil && inst.CreatedAt.After(*filter.CreatedBefore) {
		return false
	}
	return true
}

var _ engine.Store = (*Store)(nil)
