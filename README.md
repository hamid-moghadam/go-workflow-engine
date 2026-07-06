# Go Workflow Engine

[![Go Version](https://img.shields.io/badge/go-1.24+-blue.svg)](https://golang.org)
[![License](https://img.shields.io/badge/license-MIT-green.svg)](LICENSE)
[![CI](https://github.com/hamid-moghadam/go-workflow-engine/workflows/CI/badge.svg)](https://github.com/hamid-moghadam/go-workflow-engine/actions)

A flexible, JSON-configurable workflow engine for Go applications. Define complex business processes using declarative JSON while maintaining full programmatic control.

Built with [Cursor](https://cursor.sh) and [MiMo Code](https://github.com/XiaoMi/mimo-code) — AI-powered development tools.

## Features

- **JSON-Based Configuration** — Define workflows declaratively
- **Pluggable Storage** — In-memory (testing) and GORM (PostgreSQL, MySQL, SQLite, MSSQL)
- **Event System** — Before/after transition listeners with filtering
- **HTTP API** — Echo framework integration with user and admin routes
- **Async Dispatch** — Non-blocking event channel with graceful shutdown
- **Concurrent Safe** — Thread-safe operations
- **Versioned Migrations** — Auto-upgrade schema on package update

## Quick Start

```go
package main

import (
    "log"
    "os"
    "github.com/labstack/echo/v4"
    "github.com/rs/zerolog"
    "github.com/hamid-moghadam/go-workflow-engine/pkg/engine"
    workflowecho "github.com/hamid-moghadam/go-workflow-engine/echo"
    "github.com/hamid-moghadam/go-workflow-engine/pkg/store/memory"
)

func main() {
    store := memory.New()
    loader := engine.NewWorkflowLoader("./workflows")
    loader.LoadAll()

    logger := zerolog.New(os.Stdout)
    service := engine.NewWorkflowService(store, engine.NewRegistry(), logger)
    defer service.Close()

    e := echo.New()
    wc := &workflowecho.WorkflowContext{Service: service}
    e.Use(workflowecho.WorkflowContextMiddleware(wc))

    userGroup := e.Group("/api", workflowecho.DefaultUserIDMiddleware())
    workflowecho.RegisterWorkflowRoutes(userGroup, service)

    e.Start(":8080")
}
```

## Installation

```bash
go get github.com/hamid-moghadam/go-workflow-engine
```

## Database Support

| Store | Package | Use Case |
|-------|---------|----------|
| In-Memory | `pkg/store/memory` | Testing, development |
| GORM | `pkg/store/gorm` | Production |

For production, install a GORM driver:

```bash
go get gorm.io/driver/postgres   # PostgreSQL
go get gorm.io/driver/mysql      # MySQL
go get gorm.io/driver/sqlite     # SQLite
go get gorm.io/driver/sqlserver  # SQL Server
```

Use `gormstore.AutoMigrate(db)` instead of `db.AutoMigrate()` for versioned schema management.

## Workflow Definition

```json
{
  "workflow_type": "approval",
  "initial_step_name": "submit",
  "initial_state": "Pending",
  "steps": [
    {
      "name": "submit",
      "title": "Submit Request",
      "order": 1,
      "actions": [
        {
          "name": "SUBMIT",
          "next_step": "review",
          "new_state": "Submitted"
        }
      ]
    },
    {
      "name": "review",
      "title": "Review Request",
      "order": 2,
      "actions": [
        { "name": "APPROVE", "next_step": "done", "new_state": "Approved" },
        { "name": "REJECT", "next_step": "submit", "new_state": "Rejected" }
      ]
    },
    {
      "name": "done",
      "title": "Complete",
      "order": 3,
      "actions": []
    }
  ]
}
```

## API Endpoints

| Method | Endpoint | Description |
|--------|----------|-------------|
| `PUT` | `/workflows/:type/steps/:step_name` | Execute transition |
| `GET` | `/workflows/:type` | Get workflow instance |
| `GET` | `/workflows/:type/steps/:step_name` | Get step details |
| `GET` | `/admin/workflows` | List all instances |
| `PUT` | `/admin/workflows/:id/steps/:step_name` | Admin transition |

## Event Listeners

```go
// After transition (side effects)
service.OnAfterTransition("approval", "Approved", func(e engine.TransitionEvent) error {
    emailService.Send(e.Instance.UserID, "Your request was approved")
    return nil
})

// Before transition (data enrichment)
service.OnBeforeTransition("", "", func(e engine.BeforeTransitionEvent) (map[string]interface{}, error) {
    e.InputData["timestamp"] = time.Now().Format(time.RFC3339)
    return e.InputData, nil
})
```

## Async Events

```go
go func() {
    for event := range service.EventCh() {
        log.Printf("Transition: %s -> %s", event.FromState, event.ToState)
    }
}()
defer service.Close()
```

## Authentication

```go
// No auth (anonymous)
userGroup := e.Group("/api", workflowecho.DefaultUserIDMiddleware())

// Custom auth (JWT, headers, API keys)
userGroup := e.Group("/api", workflowecho.UserIDMiddleware(func(c echo.Context) (int64, error) {
    idStr := c.Request().Header.Get("X-User-ID")
    return strconv.ParseInt(idStr, 10, 64)
}))
```

## Documentation

- [Full Documentation](docs/implementation.md) — Architecture, API reference, configuration details
- [Example Application](examples/simple_approval/) — Complete approval workflow with HTTP API

## Development

```bash
go mod download
make test
make lint
make build
```

## Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Run `make ci`
5. Open a Pull Request

## License

MIT License — see [LICENSE](LICENSE)
