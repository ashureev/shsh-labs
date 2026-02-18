# Code Conventions

This document defines coding conventions and patterns used in this project.

## Mutex Patterns

All mutex operations MUST use the defer pattern for unlocks to ensure safety and consistency.

### Correct Pattern

```go
m.mu.Lock()
defer m.mu.Unlock()
// ... critical section ...
```

For RWMutex:

```go
m.mu.RLock()
defer m.mu.RUnlock()
// ... read-only critical section ...
```

### Incorrect Pattern (Avoid)

```go
m.mu.Lock()
// ... critical section ...
m.mu.Unlock()  // manual - avoid
```

### Exceptions

- **internal/terminal/monitor.go**: Has complex nested mutex patterns that are excluded from this convention due to specific ordering requirements.
- Test files may use manual unlocks where necessary for test clarity.

## Rationale

Using `defer` for mutex unlocks ensures:
1. **Safety**: Unlock is always called even if the function panics or returns early
2. **Consistency**: All code follows the same pattern
3. **Maintainability**: Reduces risk of forgotten unlocks during refactoring
