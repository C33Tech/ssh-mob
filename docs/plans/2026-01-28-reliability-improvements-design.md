# Reliability Improvements Design

## Overview

Add three reliability features to SSH Mob:
1. Graceful shutdown with SIGINT/SIGTERM handling
2. Connection retry logic with exponential backoff
3. Context support for cancellation propagation

## Context Flow & Signal Handling

A root `context.Context` is created with `signal.NotifyContext` listening for SIGINT and SIGTERM. This context is passed to each agent.

**Signal behavior:**
- First signal: Context cancelled, agents finish current command then exit gracefully
- Second signal: Immediate `os.Exit(1)`

**User feedback:**
- First signal: `"Shutting down gracefully... (press Ctrl+C again to force)"`
- Second signal: `"Forcing exit..."`

## Connection Retry Logic

**New CLI flag:**
```
-retries int    Maximum connection retry attempts (default 0)
```

**Backoff strategy:**
- Exponential: 1s, 2s, 4s, 8s, 16s (capped)
- Formula: `min(2^attempt seconds, 16s)`

**Connect() behavior:**
- Check `ctx.Done()` before each attempt
- Apply ConnectionDelay only on first attempt
- On failure with retries remaining: log warning, sleep with backoff, retry
- On final failure: log error, return error

## Context Integration in Command Loops

**Method signatures change to accept context:**
```go
func (a *Agent) Connect(ctx context.Context) error
func (a *Agent) RunProgram(ctx context.Context) error
func (a *Agent) RunTTYProgram(ctx context.Context) error
func (a *Agent) RunStandardProgram(ctx context.Context) error
```

**Loop structure:**
1. Check `ctx.Done()` - if cancelled, return (graceful exit)
2. Check TTL - if expired, return
3. Execute current command
4. Context-aware sleep using select with `time.After` and `ctx.Done()`

Commands complete before shutdown - "graceful" means finish current work.

## Main Orchestration

```go
// Setup context with signal handling
ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
defer cancel()

// Handle second signal for force exit
go func() {
    <-ctx.Done()
    log.Warn("Shutting down gracefully... (press Ctrl+C again to force)")
    sigChan := make(chan os.Signal, 1)
    signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
    <-sigChan
    log.Warn("Forcing exit...")
    os.Exit(1)
}()
```

**New imports:** `context`, `os/signal`, `syscall`

## Files Changed

- `main.go`: Signal handling, context creation, new `-retries` flag
- `ssh.go`: Context parameters, retry logic in Connect(), context-aware sleeps
