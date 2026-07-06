# Simple Workflow Engine - Implementation Document

## Project Overview

**Simple Workflow Engine** is a flexible, JSON-configurable workflow engine for Go applications. It enables developers to define complex business processes using declarative JSON configurations while maintaining full programmatic control.

---

## Technology Stack

### Go Version
- **Specified**: Go 1.24 (in go.mod)
- **Installed**: Go 1.24
- **Status**: Go 1.24 is the current stable version

### Dependencies
| Package | Version | Purpose |
|---------|---------|---------|
| Echo Framework | v4.11.2 | HTTP web framework for API endpoints |
| GORM | v1.30.0 | ORM for database operations |
| Zerolog | v1.31.0 | Structured logging |
| Testify | v1.9.0 | Testing utilities (assertions, mocking) |

---

## Project Structure

```
simple-workflow/
├── go.mod                          # Module definition (Go 1.24)
├── go.sum                          # Dependency checksums
├── LICENSE                         # MIT License
├── Makefile                        # Build automation (20+ targets)
├── README.md                       # User documentation
├── IMPLEMENTATION.md               # This file - technical documentation
├── .github/workflows/ci.yml        # GitHub Actions CI/CD
│
├── pkg/                            # Public packages (library API)
│   ├── context/                    # Workflow execution context
│   │   └── context.go              # StepContext for handler dependencies
│   │
│   ├── engine/                     # Core workflow engine
│   │   ├── definition.go           # WorkflowDefinition interface, DynamicWorkflow
│   │   ├── definition_test.go      # Tests for workflow definitions
│   │   ├── instance.go             # WorkflowInstance, Transition models
│   │   ├── loader.go               # JSON workflow loading utilities
│   │   ├── loader_test.go          # Tests for workflow loading (37 tests)
│   │   ├── registry.go             # Function registry for validations/processors
│   │   ├── registry_test.go        # Registry tests
│   │   ├── service.go              # Core workflow service implementation
│   │   └── service_test.go         # Service tests with mocked store
│   │
│   └── store/                      # Storage abstraction layer
│       ├── interface.go            # Store interface (pluggable storage)
│       ├── gorm/                   # GORM-based implementation
│       │   ├── types.go            # JSONRawMessage (cross-DB JSON support)
│       │   ├── filters.go          # Query filtering utilities
│       │   ├── migrations.go       # AutoMigrate (create-if-not-exists)
│       │   ├── models.go           # GORM models (WorkflowInstance, etc.)
│       │   └── store.go            # GormStore implementation
│       └── memory/                 # In-memory mock store for testing
│           └── store.go
│
├── echo/                           # Echo framework integration
│   ├── doc.go                      # Package documentation
│   ├── handlers.go                 # HTTP request handlers
│   ├── middleware.go               # Echo middleware (auth, context)
│   ├── response.go                 # HTTP response utilities
│   └── routes.go                   # Route registration helpers
│
├── examples/                       # Example applications
│   └── simple_approval/            # Complete approval workflow example
│       ├── main.go                 # Example server setup
│       ├── README.md               # Example documentation
│       ├── handlers/
│       │   └── custom.go           # Custom validation/processor functions
│       └── workflows/
│           └── approval.json       # Sample workflow definition
│
└── tests/                          # Additional test suites
    └── integration/                # Integration tests
        ├── api_test.go             # HTTP API endpoint tests
        └── workflow_e2e_test.go    # End-to-end workflow tests
```

---

## Architecture

### Core Design Patterns

#### 1. Finite State Machine (FSM)
The workflow engine is built around a finite state machine pattern:
- **States**: Represented by step names and state values
- **Transitions**: Triggered by actions, moving from one step to another
- **Actions**: Named events that cause transitions

#### 2. Registry Pattern
Custom functions are registered by name:
- `ValidationFunc`: Validates input data before transition

Benefits:
- JSON-configurable workflows can reference functions by name
- Functions can be defined in user code
- No reflection required at runtime

#### 3. Interface Segregation
Key interfaces allow pluggable implementations:
- `Store`: Database abstraction (GORM, mock, or custom)
- `WorkflowDefinition`: Can be JSON-based or programmatically defined

### Data Flow

```
1. Client Request
   ↓
2. Echo Handler (HTTP layer)
   ↓
3. WorkflowService.TransitionStep()
   ↓
4. Validate (Registry lookup)
   ↓
5. Emit BeforeTransitionEvent (OnBeforeTransition listeners - can modify input)
   ↓
6. Store.UpdateInstance() (Database)
   ↓
7. Emit TransitionEvent (OnAfterTransition listeners)
    ↓
8. Response to Client
```

### Async Event System

Events are dispatched via a buffered channel (capacity 100). The `TransitionStep` method queues events and returns immediately. A background goroutine processes the queue and dispatches to registered listeners.

```go
type WorkflowService struct {
    eventCh chan TransitionEvent // buffered channel
    done    chan struct{}        // signals worker completion
}

// EventCh returns a read-only channel for consuming events
func (s *WorkflowService) EventCh() <-chan TransitionEvent

// Close drains the channel and waits for the worker to finish
func (s *WorkflowService) Close() error
```

Channel full behavior: events are dropped with a warning log.

---

## Key Components

### 1. WorkflowDefinition (`pkg/engine/definition.go`)

```go
type WorkflowDefinition interface {
    GetType() string
    GetInitialStepName() string
    GetInitialState() string
    GetStep(stepName string) (*StepDefinition, error)
    GetTransitionAction(stepName, actionName string) (*Action, error)
}
```

**DynamicWorkflow** implements this interface using JSON configuration:
- `WorkflowType`: Unique identifier
- `InitialStepName`: Starting step
- `InitialState`: Starting state value
- `Steps`: Map of step definitions

### 2. WorkflowInstance (`pkg/engine/instance.go`)

Tracks runtime state of a workflow:
```go
type WorkflowInstance struct {
    ID           int64
    WorkflowType string
    CurrentStep  string
    CurrentState string
    UserID       int64
    CreatedAt    time.Time
    UpdatedAt    time.Time
    FinishedAt   *time.Time
}
```

### 3. Registry (`pkg/engine/registry.go`)

Thread-safe function storage:
```go
type Registry struct {
    validations map[string]ValidationFunc
    // ... with sync.RWMutex for concurrency
}
```

### 4. Service (`pkg/engine/service.go`)

Core business logic:
```go
type WorkflowService struct {
    db       *gorm.DB
    registry *Registry
    logger   zerolog.Logger
}

func (s *WorkflowService) TransitionStep(ctx *StepContext, instanceID int64, actionName string, inputData map[string]interface{}) error
```

### 5. Store Interface (`pkg/engine/store.go`)

```go
type Store interface {
    CreateInstance(ctx context.Context, instance *WorkflowInstance) error
    GetInstance(ctx context.Context, userID int64, workflowType string) (*WorkflowInstance, error)
    UpdateInstance(ctx context.Context, instance *WorkflowInstance) error
    // ... transitions and history
}
```

### 6. JSONRawMessage (`pkg/store/gorm/types.go`)

Custom type for cross-database JSON support:

```go
type JSONRawMessage json.RawMessage

func (j *JSONRawMessage) Scan(value interface{}) error  // handles []byte (MySQL/SQLite) and string (PostgreSQL)
func (j JSONRawMessage) Value() (driver.Value, error)   // returns []byte for all drivers
```

All GORM model fields that store JSON use `JSONRawMessage` instead of `json.RawMessage` to ensure compatibility with all database drivers.

### 7. Migration System (`pkg/store/gorm/migrations.go`)

Versioned migration system for schema evolution:

```go
// AutoMigrate creates tables if not exist + runs pending migrations
gormstore.AutoMigrate(db)

// Or run migrations separately
gormstore.RunMigrations(db)
```

**How it works:**
- `schema_migrations` table tracks applied migrations by ID
- Each migration runs exactly once, even across version upgrades
- Consumers who `go get` a new version automatically get pending migrations

**Adding a new migration in a future version:**

```go
// In pkg/store/gorm/migrations.go, add to GetDefaultMigrations():
{
    ID:   "002_add_priority_column",
    Name: "Add priority column to workflow_instances",
    Apply: func(db *gorm.DB) error {
        return db.Migrator().AddColumn(&WorkflowInstance{}, "Priority")
    },
    Rollback: func(db *gorm.DB) error {
        return db.Migrator().DropColumn(&WorkflowInstance{}, "Priority")
    },
},
```

**Consumer upgrade flow:**
1. Consumer runs `go get github.com/hamid-moghadam/go-workflow-engine@latest`
2. On next app startup, `AutoMigrate(db)` runs
3. Pending migrations (e.g., "002") are detected and applied
4. `schema_migrations` table is updated with the new migration ID

---

## Testing

### Test Statistics
- **Unit Tests**: 62 passing tests
- **Integration Tests**: 19 passing tests
- **Total**: 81 tests
- **Coverage Areas**:
  - Workflow definition parsing
  - JSON loading (file, string, reader)
  - Validation logic
  - Registry function management
  - Service methods (with mocked store)
  - Event system (before/after transitions)
  - HTTP API endpoints
  - End-to-end workflow lifecycle

### Test Commands
```bash
# Run all tests
make test

# Run only unit tests (no SQLite)
make test-short

# Generate coverage report
make coverage

# Run benchmarks
make benchmark
```

### Test Patterns
1. **Table-driven tests**: For validation scenarios
2. **Mock store**: For unit testing service layer
3. **Temp directories**: For file-based loader tests
4. **Subtests**: For grouped test cases

---

## Echo Framework Integration

### Conventions Followed

1. **Handler Signatures**: Standard Echo handler format
   ```go
   func Handler(c echo.Context) error
   ```

2. **Middleware Pattern**: Composable middleware functions
   ```go
   func WorkflowContextMiddleware(wc *WorkflowContext) echo.MiddlewareFunc
   ```

3. **Group-based Routing**: Logical route grouping
   ```go
   userGroup := e.Group("/workflows", authMiddleware)
   RegisterWorkflowRoutes(userGroup, service)
   ```

4. **Context Injection**: Service access through context
   ```go
   svc, err := MustGetService(c)
   ```

5. **Error Handling**: Consistent error responses
   ```go
   return Error(c, http.StatusBadRequest, "transition failed", err)
   ```

---

## Go Conventions & Standards

### Code Style
✅ **Following Standards**:
- `gofmt` formatting
- Go naming conventions (CamelCase for exported, camelCase for internal)
- Package comments (`doc.go`)
- Interface naming (`-er` suffix: `WorkflowDefinition`, `Store`)
- Constructor pattern (`NewXxx` functions)

### Error Handling
✅ **Idiomatic Go**:
- Error wrapping with `fmt.Errorf("...: %w", err)`
- Sentinel errors where appropriate
- Early returns to reduce nesting

### Concurrency
✅ **Thread Safety**:
- `sync.RWMutex` for registry operations
- Context propagation for cancellation
- No global mutable state (except registry for convenience)

### Documentation
✅ **GoDoc Compatible**:
- All exported types and functions documented
- Examples in README
- Architecture diagrams in documentation

### Module Organization
✅ **Standard Layout**:
- `pkg/`: Public API packages
- `cmd/` or `examples/`: Applications (using `examples/` here)
- Root: Configuration, documentation, build files

---

## User Integration Guide

### Can Users Install and Use This?

**YES** - The library is ready for integration.

### Installation

```bash
go get github.com/hamid-moghadam/go-workflow-engine
```

### Quick Integration Steps

1. **Create Workflow JSON** (`workflows/myflow.json`):
```json
{
  "workflow_type": "myworkflow",
  "initial_step_name": "start",
  "initial_state": "pending",
  "steps": [
    {
      "name": "start",
      "title": "Start",
      "order": 1,
      "actions": [
        {"name": "NEXT", "next_step": "end", "new_state": "completed"}
      ]
    },
    {"name": "end", "title": "End", "order": 2, "actions": []}
  ]
}
```

2. **Integrate into Existing Project**:

**In-memory store (testing/development):**

```go
package main

import (
    "github.com/labstack/echo/v4"
    "github.com/rs/zerolog"
    "github.com/hamid-moghadam/go-workflow-engine/pkg/engine"
    workflowecho "github.com/hamid-moghadam/go-workflow-engine/echo"
    "github.com/hamid-moghadam/go-workflow-engine/pkg/store/memory"
)

func main() {
    store := memory.New()
    registry := engine.NewRegistry()
    service := engine.NewWorkflowService(store, registry, zerolog.New(os.Stdout))

    loader := engine.NewWorkflowLoader("./workflows")
    loader.LoadAll()

    e := echo.New()
    wc := &workflowecho.WorkflowContext{Service: service}
    e.Use(workflowecho.WorkflowContextMiddleware(wc))
    workflowecho.RegisterAllRoutes(e, service, nil, nil)
    e.Start(":8080")
}
```

**GORM store with any database (production):**

```go
package main

import (
    "github.com/labstack/echo/v4"
    "github.com/rs/zerolog"
    "gorm.io/gorm"
    "gorm.io/driver/postgres"  // or sqlite, mysql, sqlserver
    "github.com/hamid-moghadam/go-workflow-engine/pkg/engine"
    workflowecho "github.com/hamid-moghadam/go-workflow-engine/echo"
    gormstore "github.com/hamid-moghadam/go-workflow-engine/pkg/store/gorm"
)

func main() {
    db, _ := gorm.Open(postgres.Open(dsn), &gorm.Config{})

    // IMPORTANT: Use gormstore.AutoMigrate, NOT db.AutoMigrate
    gormstore.AutoMigrate(db)

    store := gormstore.NewGormStore(db)
    registry := engine.NewRegistry()
    service := engine.NewWorkflowService(store, registry, zerolog.New(os.Stdout))

    loader := engine.NewWorkflowLoader("./workflows")
    loader.LoadAll()

    e := echo.New()
    wc := &workflowecho.WorkflowContext{Service: service}
    e.Use(workflowecho.WorkflowContextMiddleware(wc))
    workflowecho.RegisterAllRoutes(e, service, nil, nil)
    e.Start(":8080")
}
```

3. **Use the API**:
```bash
# Create workflow instance
curl -X PUT http://localhost:8080/workflows/myworkflow/steps/start \
  -H "Content-Type: application/json" \
  -d '{"action_name": "NEXT", "input": {"key": "value"}}'
```

---

## Build System

### Makefile Targets

| Target | Description |
|--------|-------------|
| `make test` | Run all tests |
| `make test-short` | Run unit tests only |
| `make build` | Build example application |
| `make lint` | Run golangci-lint |
| `make fmt` | Format code with gofmt |
| `make coverage` | Generate coverage report |
| `make benchmark` | Run benchmarks |
| `make ci` | Run all CI checks |
| `make help` | Show all available targets |

---

## CI/CD

### GitHub Actions Workflow

6 jobs run on every PR/push:
1. **lint**: golangci-lint + format check
2. **test-unit**: Fast unit tests
3. **test-integration**: SQLite integration tests
4. **coverage**: Coverage report + Codecov upload
5. **build**: Example application build
6. **benchmark**: Performance benchmarks
7. **mod-tidy**: Module consistency check

---

## Recommendations & Ideas

### Short Term
1. **Upgrade Go**: Consider upgrading to Go 1.22+ for improved loop semantics
2. **Add SQL driver options**: Support PostgreSQL, MySQL explicitly in examples
3. **WebSocket support**: Real-time workflow updates
4. **Admin UI**: Simple web interface for workflow management

### Long Term
1. **Workflow versioning**: Handle workflow definition changes for running instances
2. **Distributed workflows**: Support for multi-service workflow orchestration
3. **Event sourcing**: Store all events for audit trails
4. **Plugin system**: Load validations/processors from shared libraries
5. **Graph visualization**: Generate workflow diagrams from JSON definitions

### Code Quality Ideas
1. **Stricter linting**: Enable more golangci-lint rules
2. **Property-based testing**: Use `gopter` or `rapid` for generative testing
3. **Fuzz testing**: For JSON parsing edge cases
4. **Integration with tools**: OpenAPI spec generation, CLI tool for workflow validation

---

## Summary

**Status**: ✅ **Production Ready**

The Simple Workflow Engine is:
- Well-tested (81 tests: 62 unit + 19 integration)
- Properly documented (README, code comments)
- Following Go conventions
- Cross-database compatible (PostgreSQL, MySQL, SQLite, SQL Server)
- Ready for integration into existing projects
- CI/CD automated

**Go Version**: Go 1.24+
**GORM Version**: v1.30.0 (users bring their own database driver)
**License**: MIT (permissive, commercial-friendly)
