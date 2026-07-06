# GitHub Readiness & Async Events Design

## [S1] Problem

The workflow engine needs to be ready for public GitHub release. Key issues:
1. Version inconsistencies (go.mod says 1.24, README/CI say 1.21)
2. Synchronous event system blocks transitions on slow listeners
3. Documentation is outdated
4. Comments need aggressive cleanup
5. Sample project needs updating

## [S2] Solution Overview

Three parallel workstreams:
1. **Async events**: Add channel + goroutine dispatch to `WorkflowService`
2. **Documentation updates**: Fix all version refs, add async events docs
3. **Code cleanup**: Remove redundant comments, update sample project

## [S3] Async Event Architecture

Add to `WorkflowService`:
- `eventCh chan TransitionEvent` — buffered channel (default 100)
- `done chan struct{}` — signals worker completion
- `EventCh() <-chan TransitionEvent` — read-only accessor for consumers
- Background goroutine `processEvents()` dispatches to `emit()`
- `Close()` drains channel and waits for worker
- `TransitionStep` sends to channel (non-blocking `select`)

Channel buffer full behavior: drop event with warning log.

## [S4] Documentation Updates

- README.md: Badge → Go 1.24, prerequisites → Go 1.24, add Async Events section
- docs/implementation.md: Go 1.24, GORM v1.30.0, add async architecture section
- CI (.github/workflows/ci.yml): Go 1.21 → 1.24

## [S5] Sample Project Update

Update `examples/simple_approval/main.go` to demonstrate `EventCh()` consumption.

## [S6] Comment Cleanup

Aggressive removal: delete all comments that restate code behavior. Keep only:
- Exported API doc comments (GoDoc)
- Complex logic explanations
- Non-obvious design decisions
