# GitHub Readiness & Async Events Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use compose:subagent (recommended) or compose:execute to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make the workflow engine GitHub-ready with async event dispatch, updated docs, and clean code.

**Architecture:** Add a buffered channel + background goroutine to `WorkflowService` for non-blocking event dispatch. Update all documentation for Go 1.24. Aggressively remove redundant comments. Update sample project.

**Tech Stack:** Go 1.24, Echo v4, zerolog, testify

## Global Constraints

- Go version: 1.24
- Module path: `github.com/hamid-moghadam/go-workflow-engine`
- Use existing project patterns (table-driven tests, zerolog, testify)
- TDD: write failing test first, then implement, then verify
- Commit after each task

---

## File Map

| File | Action | Purpose |
|------|--------|---------|
| `pkg/engine/service.go` | Modify | Add eventCh, EventCh(), Close(), async dispatch |
| `pkg/engine/service_test.go` | Modify | Add async event tests |
| `README.md` | Modify | Update Go version, add async events docs |
| `docs/implementation.md` | Modify | Update Go/GORM versions, add async section |
| `.github/workflows/ci.yml` | Modify | Update Go version to 1.24 |
| `examples/simple_approval/main.go` | Modify | Show EventCh() usage |
| `echo/middleware.go` | Modify | Remove commented-out code |
| `echo/handlers.go` | Modify | Remove redundant comments |
| `echo/routes.go` | Modify | Remove redundant comments |
| `pkg/engine/definition.go` | Modify | Remove redundant comments |

---

### Task 1: Add Async Event Channel to WorkflowService (TDD)

**Covers:** [S3]

**Files:**
- Modify: `pkg/engine/service.go`
- Modify: `pkg/engine/service_test.go`

**Interfaces:**
- Produces: `EventCh() <-chan TransitionEvent`, `Close() error`

- [ ] **Step 1: Write the failing test for EventCh()**

```go
func TestEventCh_ReturnsChannel(t *testing.T) {
	store := &mockStore{}
	registry := NewRegistry()
	logger := zerolog.Nop()
	service := NewWorkflowService(store, registry, logger)
	defer service.Close()

	ch := service.EventCh()
	if ch == nil {
		t.Fatal("EventCh() returned nil")
	}

	select {
	case ch <- TransitionEvent{}:
		// Should be able to send (buffered)
	default:
		t.Fatal("EventCh() channel is not buffered")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test -v -run TestEventCh_ReturnsChannel ./pkg/engine/...`
Expected: FAIL with "service.EventCh undefined"

- [ ] **Step 3: Add eventCh field and EventCh() method**

Add to `WorkflowService` struct in `service.go`:
```go
type WorkflowService struct {
	store    Store
	registry *Registry
	logger   zerolog.Logger
	events   *TransitionListenerManager
	eventCh  chan TransitionEvent
	done     chan struct{}
}
```

Add `EventCh()` method:
```go
func (s *WorkflowService) EventCh() <-chan TransitionEvent {
	return s.eventCh
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test -v -run TestEventCh_ReturnsChannel ./pkg/engine/...`
Expected: PASS

- [ ] **Step 5: Write the failing test for async dispatch**

```go
func TestTransitionStep_AsyncEvent(t *testing.T) {
	store := &mockStore{}
	registry := NewRegistry()
	logger := zerolog.Nop()
	service := NewWorkflowService(store, registry, logger)
	defer service.Close()

	// Register a listener
	listenerCalled := make(chan TransitionEvent, 1)
	service.OnAfterTransition("", "", func(e TransitionEvent) error {
		listenerCalled <- e
		return nil
	})

	// Setup test data
	instance := &WorkflowInstance{
		ID:           1,
		WorkflowType: "test",
		CurrentStep:  "start",
		CurrentState: "pending",
	}
	store.instances[1] = instance

	def := &DynamicWorkflow{
		WorkflowType:    "test",
		InitialStepName: "start",
		InitialState:    "pending",
		Steps: map[string]*StepDefinition{
			"start": {
				Name: "start",
				Actions: []Action{
					{Name: "NEXT", NextStep: "end", NewState: "done"},
				},
			},
			"end": {Name: "end"},
		},
	}
	RegisterWorkflow(def)

	err := service.TransitionStep(context.Background(), 1, "NEXT", nil)
	if err != nil {
		t.Fatalf("TransitionStep failed: %v", err)
	}

	// Instance should be updated synchronously
	if instance.CurrentStep != "end" {
		t.Fatalf("expected step 'end', got '%s'", instance.CurrentStep)
	}

	// Event should be received asynchronously
	select {
	case e := <-listenerCalled:
		if e.ToState != "done" {
			t.Fatalf("expected to_state 'done', got '%s'", e.ToState)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for async event")
	}
}
```

- [ ] **Step 6: Run test to verify it fails**

Run: `go test -v -run TestTransitionStep_AsyncEvent ./pkg/engine/...`
Expected: FAIL (no background worker yet)

- [ ] **Step 7: Implement async dispatch in service.go**

Modify `NewWorkflowService`:
```go
func NewWorkflowService(store Store, registry *Registry, logger zerolog.Logger) *WorkflowService {
	svc := &WorkflowService{
		store:    store,
		registry: registry,
		logger:   logger,
		events:   NewTransitionListenerManager(),
		eventCh:  make(chan TransitionEvent, 100),
		done:     make(chan struct{}),
	}
	go svc.processEvents()
	return svc
}
```

Add `processEvents` method:
```go
func (s *WorkflowService) processEvents() {
	defer close(s.done)
	for event := range s.eventCh {
		s.events.emit(event)
	}
}
```

Add `Close` method:
```go
func (s *WorkflowService) Close() error {
	close(s.eventCh)
	<-s.done
	return nil
}
```

Modify `TransitionStep` — replace direct `s.events.emit(...)` call with channel send:
```go
select {
case s.eventCh <- TransitionEvent{
	Type:      EventAfterTransition,
	Instance:  instance,
	Action:    actionName,
	FromStep:  fromStep,
	ToStep:    instance.CurrentStep,
	FromState: fromState,
	ToState:   instance.CurrentState,
	InputData: inputData,
	Logger:    s.logger,
	BaseCtx:   ctx,
}:
default:
	s.logger.Warn().Msg("event channel full, dropping event")
}
```

- [ ] **Step 8: Run test to verify it passes**

Run: `go test -v -run TestTransitionStep_AsyncEvent ./pkg/engine/...`
Expected: PASS

- [ ] **Step 9: Write test for channel buffer full behavior**

```go
func TestEventCh_BufferFull_DropsEvent(t *testing.T) {
	store := &mockStore{}
	registry := NewRegistry()
	logger := zerolog.Nop()
	service := NewWorkflowService(store, registry, logger)
	defer service.Close()

	// Fill the channel buffer
	for i := 0; i < 100; i++ {
		service.eventCh <- TransitionEvent{}
	}

	// Next send should not block (drops instead)
	done := make(chan struct{})
	go func() {
		service.eventCh <- TransitionEvent{}
		close(done)
	}()

	select {
	case <-done:
		// Good - didn't block
	case <-time.After(100 * time.Millisecond):
		t.Fatal("TransitionStep blocked when channel was full")
	}
}
```

- [ ] **Step 10: Run test to verify it passes**

Run: `go test -v -run TestEventCh_BufferFull_DropsEvent ./pkg/engine/...`
Expected: PASS

- [ ] **Step 11: Write test for Close() graceful shutdown**

```go
func TestClose_GracefulShutdown(t *testing.T) {
	store := &mockStore{}
	registry := NewRegistry()
	logger := zerolog.Nop()
	service := NewWorkflowService(store, registry, logger)

	err := service.Close()
	if err != nil {
		t.Fatalf("Close() failed: %v", err)
	}

	// Channel should be closed
	_, ok := <-service.EventCh()
	if ok {
		t.Fatal("channel should be closed after Close()")
	}
}
```

- [ ] **Step 12: Run all service tests**

Run: `go test -v -count=1 ./pkg/engine/...`
Expected: All PASS

- [ ] **Step 13: Commit**

```bash
git add pkg/engine/service.go pkg/engine/service_test.go
git commit -m "feat: add async event dispatch via buffered channel"
```

---

### Task 2: Update Documentation

**Covers:** [S4]

**Files:**
- Modify: `README.md`
- Modify: `docs/implementation.md`
- Modify: `.github/workflows/ci.yml`

- [ ] **Step 1: Update README.md — Go version badge**

Change line 3:
```markdown
[![Go Version](https://img.shields.io/badge/go-1.24+-blue.svg)](https://golang.org)
```

- [ ] **Step 2: Update README.md — Prerequisites**

Change lines 38-40:
```markdown
### Prerequisites

- Go 1.24 or later
```

- [ ] **Step 3: Update README.md — Add Async Events section**

Add after the "Event Listeners" section (around line 440):

```markdown
### Async Event Dispatch

Events are dispatched asynchronously via a buffered channel. The `TransitionStep` method returns immediately after queuing the event.

```go
// Listen on the event channel
go func() {
    for event := range service.EventCh() {
        log.Printf("Transition: %s -> %s", event.FromState, event.ToState)
    }
}()

// Graceful shutdown
defer service.Close()
```

Events are buffered (default 100). If the buffer is full, events are dropped with a warning log.
```

- [ ] **Step 4: Update docs/implementation.md — Go version**

Change line 12:
```markdown
- **Specified**: Go 1.24 (in go.mod)
- **Installed**: Go 1.24
- **Status**: Go 1.24 is the current stable version
```

- [ ] **Step 5: Update docs/implementation.md — GORM version**

Change line 21:
```markdown
| GORM | v1.30.0 | ORM for database operations |
```

- [ ] **Step 6: Update docs/implementation.md — Add async section**

Add after the "Data Flow" section (around line 131):

```markdown
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
```

- [ ] **Step 7: Update CI workflow**

Change all `go-version: "1.21"` to `go-version: "1.24"` in `.github/workflows/ci.yml` (lines 21, 33, 49, 62, 74, 91).

- [ ] **Step 8: Commit**

```bash
git add README.md docs/implementation.md .github/workflows/ci.yml
git commit -m "docs: update Go version to 1.24, add async events docs"
```

---

### Task 3: Update Sample Project

**Covers:** [S5]

**Files:**
- Modify: `examples/simple_approval/main.go`

- [ ] **Step 1: Update main.go to show EventCh() usage**

Replace the `OnAfterTransition` calls with `EventCh()` consumption:

```go
package main

import (
	"log"
	"os"

	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog"
	"github.com/hamid-moghadam/go-workflow-engine/examples/simple_approval/handlers"
	"github.com/hamid-moghadam/go-workflow-engine/pkg/engine"
	workflowecho "github.com/hamid-moghadam/go-workflow-engine/echo"
	"github.com/hamid-moghadam/go-workflow-engine/pkg/store/memory"
)

func main() {
	store := memory.New()

	loader := engine.NewWorkflowLoader("./workflows")
	if err := loader.LoadAll(); err != nil {
		log.Fatal(err)
	}

	registry := engine.NewRegistry()
	if err := handlers.InitCustomHandlers(registry); err != nil {
		log.Fatal("Failed to initialize custom handlers:", err)
	}

	logger := zerolog.New(os.Stdout)
	service := engine.NewWorkflowService(store, registry, logger)
	defer service.Close()

	go func() {
		for event := range service.EventCh() {
			log.Printf("[EVENT] %s: %s/%s -> %s/%s",
				event.Action, event.FromStep, event.FromState, event.ToStep, event.ToState)

			if event.ToState == "Approved" {
				log.Printf("[APPROVED] Request %d approved, sending email...", event.Instance.ID)
			}
		}
	}()

	e := echo.New()
	wc := &workflowecho.WorkflowContext{Service: service}
	e.Use(workflowecho.WorkflowContextMiddleware(wc))
	workflowecho.RegisterAllRoutes(e, service, nil, nil)

	log.Println("Server starting on :8080")
	e.Start(":8080")
}
```

- [ ] **Step 2: Build and verify**

Run: `go build ./examples/simple_approval`
Expected: No errors

- [ ] **Step 3: Commit**

```bash
git add examples/simple_approval/main.go
git commit -m "refactor: update sample to use async EventCh()"
```

---

### Task 4: Comment Cleanup

**Covers:** [S6]

**Files:**
- Modify: `echo/middleware.go`
- Modify: `echo/handlers.go`
- Modify: `echo/routes.go`
- Modify: `pkg/engine/definition.go`
- Modify: `pkg/engine/instance.go`

- [ ] **Step 1: Clean up echo/middleware.go**

Remove these lines:
- Line 30-31: `// Logger zerolog.Logger` and `// Config holds optional configuration`
- Line 36-37: `// SetWorkflowContext stores the WorkflowContext in the echo.Context` and `// This should be called by middleware before workflow handlers`
- Line 42-43: `// GetWorkflowContext retrieves the WorkflowContext from echo.Context` and `// Returns nil if not found`
- Line 56-57: `// MustGetWorkflowContext retrieves the WorkflowContext from echo.Context` and `// Returns an error if the context is not found`
- Line 66-67: `// GetService retrieves the workflow service from the echo.Context` and `// Returns nil if WorkflowContext or Service is not found`
- Line 76-77: `// MustGetService retrieves the workflow service from the echo.Context` and `// Returns an error if the service is not found`
- Line 86-87: `// SetUserID stores the user ID in the echo.Context` and `// This should be called by authentication middleware`
- Line 92-93: `// GetUserID retrieves the user ID from echo.Context` and `// Returns 0 if user ID is not set (for anonymous requests or missing auth)`
- Line 106-107: `// GetUserIDWithError retrieves the user ID from echo.Context` and `// Returns an error if user ID is not set or invalid`

Keep only the exported function signatures and the `WorkflowContextMiddleware` doc comment.

- [ ] **Step 2: Clean up echo/handlers.go**

Remove these comment blocks:
- Line 127: `// Parse request body`
- Line 133: `// Get path parameters`
- Line 141-142: `// Get or create workflow instance` and `// Create new instance if not exists`
- Line 152: `// Execute the transition`
- Line 157: `// Refresh instance to get updated state`
- Line 188: `// Parse request parameters`
- Line 202: `// Get workflow instance`
- Line 209: `// Get workflow definition`
- Line 215: `// Get current step definition`
- Line 221: `// Build action info list`
- Line 231-232: `// Get transition data if action name is provided`
- Line 300: `// Parse query parameters`
- Line 306: `// Set default limit`
- Line 311: `// Build filter`
- Line 333: `// Build response`
- Line 363-364: `// Parse workflow instance ID`
- Line 375: `// Parse request body`
- Line 387: `// Get current instance to validate step_name matches`
- Line 393: `// Validate that the provided step_name matches the instance's current step`
- Line 399: `// Execute the transition`
- Line 404: `// Get updated instance`
- Line 431: `// Parse transition ID`
- Line 439: `// Verify transition exists`
- Line 445: `// Get transition history`
- Line 451: `// Build response`

- [ ] **Step 3: Clean up echo/routes.go**

Remove these comment blocks:
- Lines 45-46: `// GET /workflows/:type - Get user's workflow instance` and `// GET /workflows/:type/steps/:step_name - Get step data`
- Lines 52-53: `// PUT /workflows/:type/steps/:step_name - Execute step transition` and `// Body: { workflow_type, step_name, action_name, input }`
- Lines 87-88: `// GET /admin/workflows - List all workflow instances with filtering` and `// Query params: workflow_type, user_id, current_step, current_state, is_finished, limit, offset`
- Lines 91-92: `// PUT /admin/workflows/:id/steps/:step_name - Admin step transition` and `// Body: { action_name, input }`
- Lines 95: `// GET /admin/workflows/:id/transitions/:transition_id/history - Get transition history`
- Lines 141-142: `// RegisterWorkflowRoutes already includes /workflows prefix in paths` and `// RegisterAdminRoutes already includes /workflows prefix in paths,`

Keep the exported function doc comments (GoDoc style).

- [ ] **Step 4: Clean up pkg/engine/instance.go**

Remove these lines:
- Lines 26-27: `// SetContextData stores arbitrary context data as JSON` and the one-liner after
- Lines 40-41: `// GetContextData retrieves context data as a map` and the one-liner after
- Lines 65-66: `// SetInputData stores input data as JSON` and the one-liner after
- Lines 79-80: `// GetInputData retrieves input data as a map` and the one-liner after
- Lines 101-102: `// SetOldValue stores the old value as JSON` and the one-liner after
- Lines 115-116: `// SetNewValue stores the new value as JSON` and the one-liner after
- Lines 129-130: `// GetOldValue retrieves the old value` and the one-liner after
- Lines 141-142: `// GetNewValue retrieves the new value` and the one-liner after
- Lines 153-154: `// TableName returns the table name for WorkflowInstance` and similar

Keep the exported type and function signatures.

- [ ] **Step 5: Run tests to verify nothing broke**

Run: `go test -v -count=1 ./pkg/... ./echo/...`
Expected: All PASS

- [ ] **Step 6: Commit**

```bash
git add echo/middleware.go echo/handlers.go echo/routes.go pkg/engine/instance.go
git commit -m "chore: remove redundant comments throughout codebase"
```

---

### Task 5: Final Verification

**Covers:** [S1, S2, S3, S4, S5, S6]

- [ ] **Step 1: Run full test suite**

Run: `make test`
Expected: All PASS

- [ ] **Step 2: Run lint**

Run: `make lint`
Expected: No errors

- [ ] **Step 3: Build example**

Run: `make build`
Expected: Binary built successfully

- [ ] **Step 4: Final commit if needed**

```bash
git add -A
git commit -m "chore: final cleanup for GitHub readiness"
```
