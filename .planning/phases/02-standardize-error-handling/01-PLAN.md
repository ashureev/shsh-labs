---
phase: 02-standardize-error-handling
plan: 01
type: execute
wave: 1
depends_on: []
files_modified:
  - internal/agent/grpc_client.go
autonomous: true
requirements:
  - ERR-01
  - ERR-02
  - ERR-03
  - ERR-04
must_haves:
  truths:
    - ERR-01: All errors use fmt.Errorf with %w for wrapping (1 fix applied)
    - ERR-02: No redundant error type checks - codebase uses wrapping pattern (verified)
    - ERR-03: slog used consistently for structured logging (17+ files - verified)
    - ERR-04: No naked fmt.Printf/Println in production code (verified - only fmt.Fprintf to response writers)
  artifacts:
    - path: internal/agent/grpc_client.go
      provides: "Fixed error wrapping in Chat method"
      contains: "fmt.Errorf with %w pattern"
  key_links:
    - from: internal/agent/grpc_client.go:167
      to: error wrapping pattern
      via: "fmt.Errorf with %w"
---

<objective>
Fix error handling inconsistency in gRPC client to use proper error wrapping with %w, and verify all error handling standards are met.

Purpose:
1. Fix the one violation of ERR-01 (line 167 in grpc_client.go)
2. Verify ERR-02, ERR-03, ERR-04 are already satisfied by the codebase

Output: Updated grpc_client.go with proper error wrapping pattern; verification documentation showing all 4 requirements satisfied.

Note: The codebase is already in excellent shape for error handling. Only one fix required.
</objective>

<execution_context>
@/home/ashu/.claude/get-shit-done/workflows/execute-plan.md
@/home/ashu/.claude/get-shit-done/templates/summary.md
</execution_context>

<context>
@.planning/ROADMAP.md
@.planning/STATE.md
@internal/agent/grpc_client.go
</context>

<tasks>

<task type="auto">
  <name>Fix error wrapping in grpc_client.go Chat method</name>
  <files>internal/agent/grpc_client.go</files>
  <action>
    In the Chat method at line 167, change:
    ```go
    yield(nil, errors.New(errMsg))
    ```
    to:
    ```go
    yield(nil, fmt.Errorf("chat error: %w", errors.New(errMsg)))
    ```

    This ensures the error uses the standard %w wrapping pattern consistent with the rest of the codebase. The error message remains user-facing compatible while allowing error unwrapping for inspection.

    Note: Line 104 uses `errors.New("connection shutdown")` which is correct - it's a static sentinel error, not wrapping another error.
  </action>
  <verify>
    Run `go build ./...` to ensure code compiles.
    Run `grep -n "errors.New(errMsg)" internal/agent/grpc_client.go` - should return no results.
    Run `grep -n "fmt.Errorf.*%w" internal/agent/grpc_client.go` - should show the new wrapping pattern.
  </verify>
  <done>
    - grpc_client.go compiles without errors
    - errors.New(errMsg) replaced with fmt.Errorf using %w
    - All existing tests pass
  </done>
</task>

<task type="auto">
  <name>Verify error handling standards across codebase</name>
  <files>internal/</files>
  <action>
    Run comprehensive checks to verify all error handling requirements are met:

    1. Check for ERR-02 (redundant error type checks):
       `grep -rn "errors\.As\|errors\.Is" internal/ --include="*.go" | grep -v "_test.go"`
       Note: No redundant type checks expected - codebase uses error wrapping.

    2. Check for ERR-03 (slog usage):
       `grep -rn "log/slog" internal/ --include="*.go" | wc -l`
       Should show 15+ files using slog.

    3. Check for ERR-04 (naked fmt.Printf/Println):
       `grep -rn "^\s*fmt\.Printf\|^\s*fmt\.Println" internal/ --include="*.go" | grep -v "_test.go"`
       Should return empty (fmt.Fprintf to response writers is acceptable).

    4. Check for proper error wrapping (%w usage):
       `grep -rn "fmt.Errorf.*%w" internal/ --include="*.go" | wc -l`
       Should show 60+ occurrences.

    Document findings in verification output.

    **Note on scope:** The codebase is already in excellent shape for ERR-02, ERR-03, ERR-04.
    This verification confirms standards compliance — only ERR-01 required a code change.
  </action>
  <verify>
    All grep commands return expected results confirming standards compliance.
  </verify>
  <done>
    - ERR-02: No redundant error type checks found (wrapping pattern used)
    - ERR-03: slog used consistently across codebase (17+ files)
    - ERR-04: No naked print statements found
    - ERR-01: Fixed in Task 1
    - %w wrapping pattern used throughout (60+ occurrences)
  </done>
</task>

</tasks>

<verification>
1. Code compiles: `go build ./...`
2. Tests pass: `go test ./internal/...`
3. Error wrapping verified: `grep -n "fmt.Errorf.*%w" internal/agent/grpc_client.go | wc -l` >= 3
4. No regression: No new lint errors introduced
</verification>

<success_criteria>
- ERR-01 satisfied: Fixed - grpc_client.go Chat method now uses fmt.Errorf with %w
- ERR-02 satisfied: Verified - no redundant error type checks (codebase uses wrapping pattern)
- ERR-03 satisfied: Verified - slog used for structured logging throughout (17+ files)
- ERR-04 satisfied: Verified - no naked fmt.Printf/Println in production code
- All tests pass
- Code compiles without errors
</success_criteria>

<output>
After completion, create `.planning/phases/02-standardize-error-handling/02-01-SUMMARY.md`
</output>
