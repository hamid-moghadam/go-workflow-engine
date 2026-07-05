package gormstore

import (
	"fmt"

	"github.com/hamid-moghadam/go-workflow-engine/pkg/store"
	"gorm.io/gorm"
)

// applyInstanceFilter applies filter criteria to a GORM query
func applyInstanceFilter(query *gorm.DB, filter store.InstanceFilter) *gorm.DB {
	if filter.WorkflowType != "" {
		query = query.Where("workflow_type = ?", filter.WorkflowType)
	}

	if filter.UserID != nil {
		query = query.Where("user_id = ?", *filter.UserID)
	}

	if filter.CurrentStep != "" {
		query = query.Where("current_step = ?", filter.CurrentStep)
	}

	if filter.CurrentState != "" {
		query = query.Where("current_state = ?", filter.CurrentState)
	}

	if filter.IsFinished != nil {
		if *filter.IsFinished {
			query = query.Where("finished_at IS NOT NULL")
		} else {
			query = query.Where("finished_at IS NULL")
		}
	}

	if filter.CreatedAfter != nil {
		query = query.Where("created_at >= ?", *filter.CreatedAfter)
	}

	if filter.CreatedBefore != nil {
		query = query.Where("created_at <= ?", *filter.CreatedBefore)
	}

	return query
}

// ApplyInstanceFilter applies filter criteria to a GORM query (exported version)
func ApplyInstanceFilter(query *gorm.DB, filter store.InstanceFilter) *gorm.DB {
	return applyInstanceFilter(query, filter)
}

// ApplyPagination applies pagination to a GORM query
func ApplyPagination(query *gorm.DB, limit, offset int) *gorm.DB {
	if limit > 0 {
		query = query.Limit(limit)
	}
	if offset > 0 {
		query = query.Offset(offset)
	}
	return query
}

// ApplyOrdering applies ordering to a GORM query
func ApplyOrdering(query *gorm.DB, orderBy, orderDir string) *gorm.DB {
	if orderBy == "" {
		orderBy = "created_at"
	}

	if orderDir == "" {
		orderDir = "DESC"
	}

	// Validate order direction
	if orderDir != "ASC" && orderDir != "DESC" {
		orderDir = "DESC"
	}

	// Validate orderBy to prevent SQL injection
	allowedFields := map[string]bool{
		"id":            true,
		"workflow_type":   true,
		"current_step":    true,
		"current_state":   true,
		"user_id":         true,
		"created_at":      true,
		"updated_at":      true,
		"finished_at":     true,
	}

	if !allowedFields[orderBy] {
		orderBy = "created_at"
	}

	return query.Order(fmt.Sprintf("%s %s", orderBy, orderDir))
}

// InstanceFilterBuilder provides a fluent interface for building instance filters
type InstanceFilterBuilder struct {
	filter store.InstanceFilter
}

// NewInstanceFilter creates a new filter builder
func NewInstanceFilter() *InstanceFilterBuilder {
	return &InstanceFilterBuilder{
		filter: store.InstanceFilter{
			Limit:  100, // Default limit
			Offset: 0,
		},
	}
}

// WithWorkflowType filters by workflow type
func (b *InstanceFilterBuilder) WithWorkflowType(workflowType string) *InstanceFilterBuilder {
	b.filter.WorkflowType = workflowType
	return b
}

// WithUserID filters by user ID
func (b *InstanceFilterBuilder) WithUserID(userID int64) *InstanceFilterBuilder {
	b.filter.UserID = &userID
	return b
}

// WithCurrentStep filters by current step
func (b *InstanceFilterBuilder) WithCurrentStep(step string) *InstanceFilterBuilder {
	b.filter.CurrentStep = step
	return b
}

// WithCurrentState filters by current state
func (b *InstanceFilterBuilder) WithCurrentState(state string) *InstanceFilterBuilder {
	b.filter.CurrentState = state
	return b
}

// WithFinished filters by finished status
func (b *InstanceFilterBuilder) WithFinished(finished bool) *InstanceFilterBuilder {
	b.filter.IsFinished = &finished
	return b
}

// WithPagination sets pagination parameters
func (b *InstanceFilterBuilder) WithPagination(limit, offset int) *InstanceFilterBuilder {
	b.filter.Limit = limit
	b.filter.Offset = offset
	return b
}

// WithLimit sets the limit
func (b *InstanceFilterBuilder) WithLimit(limit int) *InstanceFilterBuilder {
	b.filter.Limit = limit
	return b
}

// WithOffset sets the offset
func (b *InstanceFilterBuilder) WithOffset(offset int) *InstanceFilterBuilder {
	b.filter.Offset = offset
	return b
}

// Build returns the built filter
func (b *InstanceFilterBuilder) Build() store.InstanceFilter {
	return b.filter
}

// QueryOptions provides additional query options
type QueryOptions struct {
	OrderBy  string
	OrderDir string
	Preload  []string
}

// ApplyQueryOptions applies query options to a GORM query
func ApplyQueryOptions(query *gorm.DB, opts QueryOptions) *gorm.DB {
	query = ApplyOrdering(query, opts.OrderBy, opts.OrderDir)

	for _, preload := range opts.Preload {
		query = query.Preload(preload)
	}

	return query
}
