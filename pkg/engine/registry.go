package engine

import (
	"fmt"
	"sync"
)

// ValidationFunc is a function that validates transition input data
// Returns an error if validation fails
type ValidationFunc func(data map[string]interface{}) error

// Registry manages function registrations for workflow operations
type Registry struct {
	mu          sync.RWMutex
	validations map[string]ValidationFunc
}

// NewRegistry creates a new function registry
func NewRegistry() *Registry {
	return &Registry{
		validations: make(map[string]ValidationFunc),
	}
}

// RegisterValidation registers a validation function by name
func (r *Registry) RegisterValidation(name string, fn ValidationFunc) error {
	if name == "" {
		return fmt.Errorf("validation function name cannot be empty")
	}
	if fn == nil {
		return fmt.Errorf("validation function cannot be nil")
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	r.validations[name] = fn
	return nil
}

// GetValidation retrieves a validation function by name
func (r *Registry) GetValidation(name string) (ValidationFunc, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	fn, exists := r.validations[name]
	if !exists {
		return nil, fmt.Errorf("validation function '%s' not found", name)
	}
	return fn, nil
}

// HasValidation checks if a validation function is registered
func (r *Registry) HasValidation(name string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	_, exists := r.validations[name]
	return exists
}

// ListValidations returns all registered validation function names
func (r *Registry) ListValidations() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	names := make([]string, 0, len(r.validations))
	for name := range r.validations {
		names = append(names, name)
	}
	return names
}

// UnregisterValidation removes a validation function
func (r *Registry) UnregisterValidation(name string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	delete(r.validations, name)
}

// Clear removes all registered functions
func (r *Registry) Clear() {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.validations = make(map[string]ValidationFunc)
}
