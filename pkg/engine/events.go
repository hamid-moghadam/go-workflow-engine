package engine

import (
	"context"
	"sync"

	"github.com/rs/zerolog"
)

// TransitionEventType represents the type of transition event
type TransitionEventType int

const (
	EventAfterTransition TransitionEventType = iota
)

// TransitionEvent contains all information about a transition that occurred
type TransitionEvent struct {
	Type      TransitionEventType
	Instance  *WorkflowInstance
	Action    string
	FromStep  string
	ToStep    string
	FromState string
	ToState   string
	InputData map[string]interface{}
	Logger    zerolog.Logger
	BaseCtx   context.Context
}

// TransitionListener is a function that handles transition events.
// Returning an error does not block the transition; errors are logged only.
type TransitionListener func(event TransitionEvent) error

// BeforeTransitionEvent contains information about a transition about to occur.
// Listeners can modify InputData and return the enriched version.
type BeforeTransitionEvent struct {
	Instance  *WorkflowInstance
	Action    string
	StepName  string
	InputData map[string]interface{}
	Logger    zerolog.Logger
	BaseCtx   context.Context
}

// BeforeTransitionListener is a function that enriches input data before a transition.
// Return the (possibly modified) input data. Returning an error blocks the transition.
type BeforeTransitionListener func(event BeforeTransitionEvent) (map[string]interface{}, error)

// transitionListenerEntry pairs a listener with its filter criteria.
// Empty filter fields match everything (wildcard).
type transitionListenerEntry struct {
	workflowType string // empty = match all workflows
	toState      string // empty = match all target states
	listener     TransitionListener
}

// beforeTransitionListenerEntry pairs a before-transition listener with its filter criteria.
type beforeTransitionListenerEntry struct {
	workflowType string
	actionName   string // empty = match all actions
	listener     BeforeTransitionListener
}

// TransitionListenerManager manages registered transition listeners
type TransitionListenerManager struct {
	mu              sync.RWMutex
	listeners       []transitionListenerEntry
	beforeListeners []beforeTransitionListenerEntry
}

// NewTransitionListenerManager creates a new listener manager
func NewTransitionListenerManager() *TransitionListenerManager {
	return &TransitionListenerManager{}
}

// OnAfterTransition registers a listener for transitions.
// Both workflowType and toState are optional filters:
//   - Both empty: fires on every transition
//   - workflowType set, toState empty: fires on transitions for that workflow type
//   - workflowType empty, toState set: fires on transitions ending in that state
//   - Both set: fires only when both match
func (m *TransitionListenerManager) OnAfterTransition(workflowType, toState string, listener TransitionListener) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.listeners = append(m.listeners, transitionListenerEntry{
		workflowType: workflowType,
		toState:      toState,
		listener:     listener,
	})
}

// OnBeforeTransition registers a listener that fires before a transition executes.
// Listeners can modify InputData by returning the enriched version.
// Returning an error blocks the transition.
//
// Filters:
//   - Both empty: fires on every transition
//   - workflowType set, actionName empty: fires on all actions for that workflow type
//   - workflowType empty, actionName set: fires on that action for all workflows
//   - Both set: fires only when both match
func (m *TransitionListenerManager) OnBeforeTransition(workflowType, actionName string, listener BeforeTransitionListener) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.beforeListeners = append(m.beforeListeners, beforeTransitionListenerEntry{
		workflowType: workflowType,
		actionName:   actionName,
		listener:     listener,
	})
}

// emitBefore fires all matching before-transition listeners and returns enriched input data.
// If any listener returns an error, the transition is blocked.
func (m *TransitionListenerManager) emitBefore(event BeforeTransitionEvent) (map[string]interface{}, error) {
	m.mu.RLock()
	entries := make([]beforeTransitionListenerEntry, len(m.beforeListeners))
	copy(entries, m.beforeListeners)
	m.mu.RUnlock()

	inputData := event.InputData
	for _, entry := range entries {
		if !matchesBeforeFilter(entry, event) {
			continue
		}
		var err error
		inputData, err = entry.listener(BeforeTransitionEvent{
			Instance:  event.Instance,
			Action:    event.Action,
			StepName:  event.StepName,
			InputData: inputData,
			Logger:    event.Logger,
			BaseCtx:   event.BaseCtx,
		})
		if err != nil {
			return nil, err
		}
	}
	return inputData, nil
}

// emit fires all matching listeners for the given event
func (m *TransitionListenerManager) emit(event TransitionEvent) {
	m.mu.RLock()
	entries := make([]transitionListenerEntry, len(m.listeners))
	copy(entries, m.listeners)
	m.mu.RUnlock()

	for _, entry := range entries {
		if !matchesFilter(entry, event) {
			continue
		}
		if err := entry.listener(event); err != nil {
			event.Logger.Error().
				Err(err).
				Str("action", event.Action).
				Str("workflow_type", event.Instance.WorkflowType).
				Str("to_state", event.ToState).
				Msg("Transition listener failed")
		}
	}
}

// matchesFilter checks if an event matches a listener's filter criteria
func matchesFilter(entry transitionListenerEntry, event TransitionEvent) bool {
	if entry.workflowType != "" && entry.workflowType != event.Instance.WorkflowType {
		return false
	}
	if entry.toState != "" && entry.toState != event.ToState {
		return false
	}
	return true
}

// matchesBeforeFilter checks if an event matches a before-transition listener's filter criteria
func matchesBeforeFilter(entry beforeTransitionListenerEntry, event BeforeTransitionEvent) bool {
	if entry.workflowType != "" && entry.workflowType != event.Instance.WorkflowType {
		return false
	}
	if entry.actionName != "" && entry.actionName != event.Action {
		return false
	}
	return true
}

// TransitionListenerEntryForTest creates a transitionListenerEntry for testing
func TransitionListenerEntryForTest(workflowType, toState string) transitionListenerEntry {
	return transitionListenerEntry{workflowType: workflowType, toState: toState}
}

// TransitionEventForTest creates a TransitionEvent for testing
func TransitionEventForTest(workflowType, toState string) TransitionEvent {
	return TransitionEvent{
		Instance: &WorkflowInstance{WorkflowType: workflowType},
		ToState:  toState,
	}
}

// MatchesFilterForTest exposes matchesFilter for testing
func MatchesFilterForTest(entry transitionListenerEntry, event TransitionEvent) bool {
	return matchesFilter(entry, event)
}

// BeforeTransitionEventForTest creates a BeforeTransitionEvent for testing
func BeforeTransitionEventForTest(workflowType, action string) BeforeTransitionEvent {
	return BeforeTransitionEvent{
		Instance: &WorkflowInstance{WorkflowType: workflowType},
		Action:   action,
	}
}

// BeforeTransitionListenerEntryForTest creates a beforeTransitionListenerEntry for testing
func BeforeTransitionListenerEntryForTest(workflowType, actionName string) beforeTransitionListenerEntry {
	return beforeTransitionListenerEntry{workflowType: workflowType, actionName: actionName}
}

// MatchesBeforeFilterForTest exposes matchesBeforeFilter for testing
func MatchesBeforeFilterForTest(entry beforeTransitionListenerEntry, event BeforeTransitionEvent) bool {
	return matchesBeforeFilter(entry, event)
}
