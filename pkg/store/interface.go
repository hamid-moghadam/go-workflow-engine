package store

import (
	"github.com/hamid-moghadam/go-workflow-engine/pkg/engine"
)

// Re-export types from engine for backward compatibility
type InstanceFilter = engine.StoreFilter

var (
	ErrInstanceNotFound   = engine.ErrInstanceNotFound
	ErrTransitionNotFound = engine.ErrTransitionNotFound
	ErrDuplicateInstance  = engine.ErrDuplicateInstance
)

// Store is an alias for engine.Store
type Store = engine.Store
