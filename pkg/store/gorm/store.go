package gormstore

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/hamid-moghadam/go-workflow-engine/pkg/engine"
	"github.com/hamid-moghadam/go-workflow-engine/pkg/store"
	"gorm.io/gorm"
)

// GormStore implements the store.Store interface using GORM
type GormStore struct {
	db *gorm.DB
}

// NewGormStore creates a new GormStore instance
func NewGormStore(db *gorm.DB) *GormStore {
	return &GormStore{
		db: db,
	}
}

// CreateInstance creates a new workflow instance in the database
func (s *GormStore) CreateInstance(ctx context.Context, instance *engine.WorkflowInstance) error {
	if instance == nil {
		return errors.New("instance cannot be nil")
	}

	model := WorkflowInstanceFromEngine(instance)
	model.CreatedAt = time.Now()
	model.UpdatedAt = time.Now()

	if err := s.db.WithContext(ctx).Create(model).Error; err != nil {
		if isDuplicateKeyError(err) {
			return store.ErrDuplicateInstance
		}
		return fmt.Errorf("failed to create instance: %w", err)
	}

	instance.ID = model.ID
	return nil
}

// GetInstance retrieves a workflow instance by user ID and workflow type
func (s *GormStore) GetInstance(ctx context.Context, userID int64, workflowType string) (*engine.WorkflowInstance, error) {
	if userID == 0 {
		return nil, errors.New("userID cannot be 0")
	}
	if workflowType == "" {
		return nil, errors.New("workflowType cannot be empty")
	}

	var model WorkflowInstance
	err := s.db.WithContext(ctx).
		Where("user_id = ? AND workflow_type = ?", userID, workflowType).
		First(&model).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, store.ErrInstanceNotFound
		}
		return nil, fmt.Errorf("failed to get instance: %w", err)
	}

	return model.ToEngineInstance(), nil
}

// GetInstanceByID retrieves a workflow instance by its ID
func (s *GormStore) GetInstanceByID(ctx context.Context, id int64) (*engine.WorkflowInstance, error) {
	if id == 0 {
		return nil, errors.New("id cannot be 0")
	}

	var model WorkflowInstance
	err := s.db.WithContext(ctx).
		Preload("Transitions", func(db *gorm.DB) *gorm.DB {
			return db.Order("created_at DESC")
		}).
		Preload("Transitions.Histories", func(db *gorm.DB) *gorm.DB {
			return db.Order("created_at DESC")
		}).
		First(&model, id).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, store.ErrInstanceNotFound
		}
		return nil, fmt.Errorf("failed to get instance by ID: %w", err)
	}

	instance := model.ToEngineInstance()
	return instance, nil
}

// UpdateInstance updates an existing workflow instance
func (s *GormStore) UpdateInstance(ctx context.Context, instance *engine.WorkflowInstance) error {
	if instance == nil {
		return errors.New("instance cannot be nil")
	}
	if instance.ID == 0 {
		return errors.New("instance ID cannot be 0")
	}

	model := WorkflowInstanceFromEngine(instance)
	model.UpdatedAt = time.Now()

	result := s.db.WithContext(ctx).Model(&WorkflowInstance{}).
		Where("id = ?", instance.ID).
		Updates(map[string]interface{}{
			"current_step":  model.CurrentStep,
			"current_state": model.CurrentState,
			"updated_at":    model.UpdatedAt,
			"finished_at":   model.FinishedAt,
		})

	if result.Error != nil {
		return fmt.Errorf("failed to update instance: %w", result.Error)
	}

	if result.RowsAffected == 0 {
		return store.ErrInstanceNotFound
	}

	return nil
}

// CreateTransition creates a new transition record in the database
func (s *GormStore) CreateTransition(ctx context.Context, transition *engine.Transition) error {
	if transition == nil {
		return errors.New("transition cannot be nil")
	}

	model := TransitionFromEngine(transition)
	model.CreatedAt = time.Now()
	model.UpdatedAt = time.Now()

	if err := s.db.WithContext(ctx).Create(model).Error; err != nil {
		return fmt.Errorf("failed to create transition: %w", err)
	}

	transition.ID = model.ID
	return nil
}

// UpdateTransition updates an existing transition record
func (s *GormStore) UpdateTransition(ctx context.Context, transition *engine.Transition) error {
	if transition == nil {
		return errors.New("transition cannot be nil")
	}
	if transition.ID == 0 {
		return errors.New("transition ID cannot be 0")
	}

	model := TransitionFromEngine(transition)
	model.UpdatedAt = time.Now()

	result := s.db.WithContext(ctx).Model(&Transition{}).
		Where("id = ?", transition.ID).
		Updates(map[string]interface{}{
			"input_data": model.InputData,
			"state_name": model.StateName,
			"updated_at": model.UpdatedAt,
		})

	if result.Error != nil {
		return fmt.Errorf("failed to update transition: %w", result.Error)
	}

	if result.RowsAffected == 0 {
		return store.ErrTransitionNotFound
	}

	return nil
}

// GetTransition retrieves a transition by instance ID, step name, and action name
func (s *GormStore) GetTransition(ctx context.Context, instanceID int64, stepName, actionName string) (*engine.Transition, error) {
	if instanceID == 0 {
		return nil, errors.New("instanceID cannot be 0")
	}
	if stepName == "" {
		return nil, errors.New("stepName cannot be empty")
	}

	var model Transition
	query := s.db.WithContext(ctx).
		Where("instance_id = ? AND step_name = ?", instanceID, stepName)

	if actionName != "" {
		query = query.Where("action_name = ?", actionName)
	}

	err := query.First(&model).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, store.ErrTransitionNotFound
		}
		return nil, fmt.Errorf("failed to get transition: %w", err)
	}

	return model.ToEngineTransition(), nil
}

// GetTransitionByID retrieves a transition by its ID
func (s *GormStore) GetTransitionByID(ctx context.Context, id int64) (*engine.Transition, error) {
	if id == 0 {
		return nil, errors.New("id cannot be 0")
	}

	var model Transition
	err := s.db.WithContext(ctx).
		Preload("Histories", func(db *gorm.DB) *gorm.DB {
			return db.Order("created_at DESC")
		}).
		First(&model, id).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, store.ErrTransitionNotFound
		}
		return nil, fmt.Errorf("failed to get transition by ID: %w", err)
	}

	return model.ToEngineTransition(), nil
}

// CreateTransitionHistory creates a new transition history record
func (s *GormStore) CreateTransitionHistory(ctx context.Context, history *engine.TransitionHistory) error {
	if history == nil {
		return errors.New("history cannot be nil")
	}

	model := TransitionHistoryFromEngine(history)
	model.CreatedAt = time.Now()

	if err := s.db.WithContext(ctx).Create(model).Error; err != nil {
		return fmt.Errorf("failed to create transition history: %w", err)
	}

	history.ID = model.ID
	return nil
}

// ListInstances retrieves workflow instances based on filter criteria
func (s *GormStore) ListInstances(ctx context.Context, filter store.InstanceFilter) ([]engine.WorkflowInstance, error) {
	query := s.db.WithContext(ctx).Model(&WorkflowInstance{})

	// Apply filters
	query = applyInstanceFilter(query, filter)

	// Apply ordering - default to created_at DESC
	query = query.Order("created_at DESC")

	// Apply pagination
	if filter.Limit > 0 {
		query = query.Limit(filter.Limit)
	}
	if filter.Offset > 0 {
		query = query.Offset(filter.Offset)
	}

	var models []WorkflowInstance
	if err := query.Find(&models).Error; err != nil {
		return nil, fmt.Errorf("failed to list instances: %w", err)
	}

	instances := make([]engine.WorkflowInstance, len(models))
	for i, model := range models {
		instances[i] = *model.ToEngineInstance()
	}

	return instances, nil
}

// ListTransitions retrieves all transitions for a workflow instance
func (s *GormStore) ListTransitions(ctx context.Context, instanceID int64) ([]engine.Transition, error) {
	if instanceID == 0 {
		return nil, errors.New("instanceID cannot be 0")
	}

	var models []Transition
	err := s.db.WithContext(ctx).
		Where("instance_id = ?", instanceID).
		Order("created_at DESC").
		Find(&models).Error

	if err != nil {
		return nil, fmt.Errorf("failed to list transitions: %w", err)
	}

	transitions := make([]engine.Transition, len(models))
	for i, model := range models {
		transitions[i] = *model.ToEngineTransition()
	}

	return transitions, nil
}

// ListTransitionHistory retrieves history for a specific transition
func (s *GormStore) ListTransitionHistory(ctx context.Context, transitionID int64) ([]engine.TransitionHistory, error) {
	if transitionID == 0 {
		return nil, errors.New("transitionID cannot be 0")
	}

	var models []TransitionHistory
	err := s.db.WithContext(ctx).
		Where("transition_id = ?", transitionID).
		Order("created_at DESC").
		Find(&models).Error

	if err != nil {
		return nil, fmt.Errorf("failed to list transition history: %w", err)
	}

	histories := make([]engine.TransitionHistory, len(models))
	for i, model := range models {
		histories[i] = *model.ToEngineHistory()
	}

	return histories, nil
}

// Transaction executes a function within a database transaction
func (s *GormStore) Transaction(ctx context.Context, fn func(tx store.Store) error) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		txStore := &GormStore{db: tx}
		return fn(txStore)
	})
}

// Close closes the store connection
// For GORM, this is typically handled by the underlying database connection pool
func (s *GormStore) Close() error {
	sqlDB, err := s.db.DB()
	if err != nil {
		return fmt.Errorf("failed to get underlying sql.DB: %w", err)
	}
	return sqlDB.Close()
}

// DB returns the underlying GORM DB instance (useful for advanced operations)
func (s *GormStore) DB() *gorm.DB {
	return s.db
}

// isDuplicateKeyError checks if the error is a duplicate key error
// Supports PostgreSQL, MySQL, and SQLite error patterns
func isDuplicateKeyError(err error) bool {
	if err == nil {
		return false
	}
	errStr := err.Error()

	// PostgreSQL: "duplicate key value violates unique constraint" / error code 23505
	if contains(errStr, "duplicate key") || contains(errStr, "23505") {
		return true
	}

	// SQLite: "UNIQUE constraint failed" / "UNIQUE constraint violated"
	if contains(errStr, "UNIQUE constraint failed") || contains(errStr, "UNIQUE constraint violated") {
		return true
	}

	// MySQL: "Duplicate entry" / error code 1062
	if contains(errStr, "Duplicate entry") || contains(errStr, "Error 1062") || contains(errStr, "1062") {
		return true
	}

	// SQL Server: "Violation of PRIMARY KEY constraint" / "Violation of UNIQUE KEY constraint" / error 2601/2627
	if contains(errStr, "Violation of") && (contains(errStr, "constraint") || contains(errStr, "unique")) {
		return true
	}
	if contains(errStr, "2601") || contains(errStr, "2627") {
		return true
	}

	return false
}

// contains checks if a string contains a substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsSubstr(s, substr))
}

func containsSubstr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
