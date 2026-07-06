package handlers

import (
	"fmt"

	"github.com/hamid-moghadam/go-workflow-engine/pkg/engine"
)

// InitCustomHandlers registers custom validation functions
// with the provided registry. This should be called during application
// initialization before loading workflows.
func InitCustomHandlers(registry *engine.Registry) error {
	// Register validation function
	if err := registry.RegisterValidation("validateSubmit", validateSubmit); err != nil {
		return fmt.Errorf("failed to register validateSubmit: %w", err)
	}

	return nil
}

// validateSubmit validates that required fields are present in the submission
func validateSubmit(data map[string]interface{}) error {
	required := []string{"title", "description", "amount"}
	for _, field := range required {
		if _, ok := data[field]; !ok {
			return fmt.Errorf("missing required field: %s", field)
		}
	}
	return nil
}
