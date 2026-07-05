package engine

import (
	"errors"
	"fmt"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNewRegistry tests creating a new registry
func TestNewRegistry(t *testing.T) {
	r := NewRegistry()
	assert.NotNil(t, r)
	assert.NotNil(t, r.validations)
}

// TestRegisterValidation tests registering validation functions
func TestRegisterValidation(t *testing.T) {
	r := NewRegistry()

	// Valid registration
	fn := func(data map[string]interface{}) error { return nil }
	err := r.RegisterValidation("testValidation", fn)
	require.NoError(t, err)
	assert.True(t, r.HasValidation("testValidation"))

	// Test empty name
	err = r.RegisterValidation("", fn)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "name cannot be empty")

	// Test nil function
	err = r.RegisterValidation("nilFunc", nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cannot be nil")

	// Test duplicate registration (should overwrite)
	fn2 := func(data map[string]interface{}) error { return errors.New("new error") }
	err = r.RegisterValidation("testValidation", fn2)
	require.NoError(t, err)

	// Verify it was overwritten by checking the function exists
	retrieved, err := r.GetValidation("testValidation")
	require.NoError(t, err)
	assert.NotNil(t, retrieved)
	// Call the function to verify it's the new one
	callErr := retrieved(nil)
	assert.Error(t, callErr)
	assert.Contains(t, callErr.Error(), "new error")
}

// TestGetValidation tests retrieving validation functions
func TestGetValidation(t *testing.T) {
	r := NewRegistry()

	// Register and retrieve
	fn := func(data map[string]interface{}) error { return nil }
	err := r.RegisterValidation("getTest", fn)
	require.NoError(t, err)

	retrieved, err := r.GetValidation("getTest")
	require.NoError(t, err)
	assert.NotNil(t, retrieved)

	// Test non-existent
	retrieved, err = r.GetValidation("nonexistent")
	assert.Error(t, err)
	assert.Nil(t, retrieved)
	assert.Contains(t, err.Error(), "not found")
}

// TestHasFunctions tests the Has* functions
func TestHasFunctions(t *testing.T) {
	r := NewRegistry()

	// Initially nothing should exist
	assert.False(t, r.HasValidation("test"))

	// Register
	err := r.RegisterValidation("val", func(map[string]interface{}) error { return nil })
	require.NoError(t, err)

	// Now it should exist
	assert.True(t, r.HasValidation("val"))
}

// TestUnregisterFunctions tests unregistering functions
func TestUnregisterFunctions(t *testing.T) {
	r := NewRegistry()

	// Register function
	err := r.RegisterValidation("val", func(map[string]interface{}) error { return nil })
	require.NoError(t, err)

	// Verify it exists
	assert.True(t, r.HasValidation("val"))

	// Unregister
	r.UnregisterValidation("val")

	// Verify it's gone
	assert.False(t, r.HasValidation("val"))

	// Unregistering non-existent should not panic
	r.UnregisterValidation("nonexistent")
}

// TestClear tests clearing all functions
func TestClear(t *testing.T) {
	r := NewRegistry()

	// Register multiple functions
	err := r.RegisterValidation("val1", func(map[string]interface{}) error { return nil })
	require.NoError(t, err)
	err = r.RegisterValidation("val2", func(map[string]interface{}) error { return nil })
	require.NoError(t, err)

	// Verify they exist
	assert.True(t, r.HasValidation("val1"))
	assert.True(t, r.HasValidation("val2"))

	// Clear
	r.Clear()

	// Verify all gone
	assert.False(t, r.HasValidation("val1"))
	assert.False(t, r.HasValidation("val2"))
}

// TestListFunctions tests listing registered functions
func TestListFunctions(t *testing.T) {
	r := NewRegistry()

	// Register functions
	err := r.RegisterValidation("val1", func(map[string]interface{}) error { return nil })
	require.NoError(t, err)
	err = r.RegisterValidation("val2", func(map[string]interface{}) error { return nil })
	require.NoError(t, err)

	// List validations
	validations := r.ListValidations()
	assert.Len(t, validations, 2)
	assert.Contains(t, validations, "val1")
	assert.Contains(t, validations, "val2")
}

// TestConcurrentRegistrations tests thread-safety of concurrent registrations
func TestConcurrentRegistrations(t *testing.T) {
	r := NewRegistry()

	var wg sync.WaitGroup
	numGoroutines := 100
	regsPerGoroutine := 10

	// Concurrent registrations
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < regsPerGoroutine; j++ {
				name := fmt.Sprintf("validation_%d_%d", id, j)
				_ = r.RegisterValidation(name, func(map[string]interface{}) error { return nil })
			}
		}(i)
	}

	// Concurrent reads
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < regsPerGoroutine; j++ {
				r.HasValidation(fmt.Sprintf("validation_%d_%d", id, j))
				_, _ = r.GetValidation(fmt.Sprintf("validation_%d_%d", id, j))
			}
		}(i)
	}

	wg.Wait()

	// Verify all registrations succeeded
	assert.Len(t, r.ListValidations(), numGoroutines*regsPerGoroutine)
}
