package gormstore

import (
	"context"
	"fmt"

	"gorm.io/gorm"
)

// AutoMigrate automatically migrates all GORM models to create/update database tables.
// This is database-agnostic and works with SQLite, PostgreSQL, MySQL, SQL Server, etc.
// It creates tables if they don't exist and applies pending versioned migrations.
// Use this instead of db.AutoMigrate() for reliable cross-database support.
func AutoMigrate(db *gorm.DB) error {
	if err := db.AutoMigrate(
		&WorkflowInstance{},
		&Transition{},
		&TransitionHistory{},
	); err != nil {
		return err
	}

	return RunMigrations(db)
}

// RunMigrations runs all pending versioned migrations.
// Call this after AutoMigrate or separately for manual migration control.
func RunMigrations(db *gorm.DB) error {
	mm := NewMigrationManager(db)
	for _, migration := range GetDefaultMigrations() {
		mm.AddMigration(migration)
	}
	return mm.Migrate()
}

// AutoMigrateWithContext migrates all models with context support
func AutoMigrateWithContext(ctx context.Context, db *gorm.DB) error {
	return AutoMigrate(db.WithContext(ctx))
}

// DropWorkflowTables drops all workflow-related tables (use with caution!)
func DropWorkflowTables(db *gorm.DB) error {
	tables := []string{
		"transition_history",
		"workflow_transitions",
		"workflow_instances",
	}

	for _, table := range tables {
		if err := db.Migrator().DropTable(table); err != nil {
			return fmt.Errorf("failed to drop table %s: %w", table, err)
		}
	}

	return nil
}

// Migration provides a migration framework for database versioning
type Migration struct {
	ID        string
	Name      string
	Apply     func(*gorm.DB) error
	Rollback  func(*gorm.DB) error
}

// MigrationManager handles database migrations
type MigrationManager struct {
	db         *gorm.DB
	migrations []Migration
}

// NewMigrationManager creates a new migration manager
func NewMigrationManager(db *gorm.DB) *MigrationManager {
	return &MigrationManager{
		db:         db,
		migrations: []Migration{},
	}
}

// AddMigration adds a migration to the manager
func (m *MigrationManager) AddMigration(migration Migration) {
	m.migrations = append(m.migrations, migration)
}

// Migrate runs all pending migrations
func (m *MigrationManager) Migrate() error {
	if err := m.createMigrationsTable(); err != nil {
		return err
	}

	applied, err := m.getAppliedMigrations()
	if err != nil {
		return err
	}

	for _, migration := range m.migrations {
		if _, ok := applied[migration.ID]; !ok {
			if err := m.applyMigration(migration); err != nil {
				return fmt.Errorf("failed to apply migration %s: %w", migration.ID, err)
			}
		}
	}

	return nil
}

// createMigrationsTable creates the migrations tracking table if it doesn't exist
func (m *MigrationManager) createMigrationsTable() error {
	type SchemaMigration struct {
		ID        string `gorm:"primaryKey"`
		Name      string `gorm:"not null"`
		AppliedAt string `gorm:"column:applied_at;autoCreateTime"`
	}
	migrator := m.db.Migrator()
	if !migrator.HasTable(&SchemaMigration{}) {
		return migrator.CreateTable(&SchemaMigration{})
	}
	return nil
}

// getAppliedMigrations returns a map of applied migration IDs
func (m *MigrationManager) getAppliedMigrations() (map[string]bool, error) {
	type SchemaMigration struct {
		ID string `gorm:"primaryKey"`
	}

	var migrations []SchemaMigration
	if err := m.db.Find(&migrations).Error; err != nil {
		return nil, err
	}

	applied := make(map[string]bool, len(migrations))
	for _, migration := range migrations {
		applied[migration.ID] = true
	}

	return applied, nil
}

// applyMigration applies a single migration and records it
func (m *MigrationManager) applyMigration(migration Migration) error {
	type SchemaMigration struct {
		ID   string `gorm:"primaryKey"`
		Name string `gorm:"not null"`
	}

	tx := m.db.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	if err := migration.Apply(tx); err != nil {
		tx.Rollback()
		return err
	}

	if err := tx.Create(&SchemaMigration{ID: migration.ID, Name: migration.Name}).Error; err != nil {
		tx.Rollback()
		return err
	}

	return tx.Commit().Error
}

// RollbackMigration rolls back a specific migration by ID
func (m *MigrationManager) RollbackMigration(migrationID string) error {
	type SchemaMigration struct {
		ID string `gorm:"primaryKey"`
	}

	for _, migration := range m.migrations {
		if migration.ID == migrationID {
			if migration.Rollback == nil {
				return fmt.Errorf("migration %s has no rollback function", migrationID)
			}

			tx := m.db.Begin()
			defer func() {
				if r := recover(); r != nil {
					tx.Rollback()
				}
			}()

			if err := migration.Rollback(tx); err != nil {
				tx.Rollback()
				return err
			}

			if err := tx.Where("id = ?", migrationID).Delete(&SchemaMigration{}).Error; err != nil {
				tx.Rollback()
				return err
			}

			return tx.Commit().Error
		}
	}

	return fmt.Errorf("migration %s not found", migrationID)
}

// GetMigrationStatus returns the status of all migrations
func (m *MigrationManager) GetMigrationStatus() ([]MigrationStatus, error) {
	if err := m.createMigrationsTable(); err != nil {
		return nil, err
	}

	applied, err := m.getAppliedMigrations()
	if err != nil {
		return nil, err
	}

	status := make([]MigrationStatus, len(m.migrations))
	for i, migration := range m.migrations {
		status[i] = MigrationStatus{
			ID:      migration.ID,
			Name:    migration.Name,
			Applied: applied[migration.ID],
		}
	}

	return status, nil
}

// MigrationStatus represents the status of a migration
type MigrationStatus struct {
	ID      string
	Name    string
	Applied bool
}

// GetDefaultMigrations returns the default set of migrations for the workflow engine.
// Add new migrations here when schema changes are needed in future versions.
// Each migration has a unique ID, runs once, and is tracked in schema_migrations table.
//
// To add a new migration in a future version:
// 1. Append a new Migration{} to this slice with a unique ID (e.g., "002_add_priority_column")
// 2. Implement Apply func with the schema change (e.g., AddColumn, RenameColumn, etc.)
// 3. Optionally implement Rollback func for reversal
// 4. Consumers who update the package will automatically run pending migrations on next startup
func GetDefaultMigrations() []Migration {
	return []Migration{
		{
			ID:   "001_create_initial_tables",
			Name: "Create initial workflow tables",
			Apply: func(db *gorm.DB) error {
				// Tables are already created by AutoMigrate before running migrations.
				// This migration is a marker for the initial schema version.
				return nil
			},
		},
		// Example for future versions:
		// {
		//     ID:   "002_add_priority_column",
		//     Name: "Add priority column to workflow_instances",
		//     Apply: func(db *gorm.DB) error {
		//         return db.Migrator().AddColumn(&WorkflowInstance{}, "Priority")
		//     },
		//     Rollback: func(db *gorm.DB) error {
		//         return db.Migrator().DropColumn(&WorkflowInstance{}, "Priority")
		//     },
		// },
	}
}
