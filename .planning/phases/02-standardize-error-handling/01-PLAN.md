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
    - All errors use fmt.Errorf with %w for wrapping
    - No errors.New used for wrapping error messages
    - slog used consistently for structured logging
    - No naked fmt.Printf/Println in production code
  artifacts:
    - path: internal/agent/grpc_client.go
      provides: "Fixed error wrapping in Chat method"
      contains: "fmt.Errorf(\"%s: %w\", errMsg, baseErr)"
  key_links:
    - from: internal/agent/grpc_client.go:167
      to: error wrapping pattern
      via: "fmt.Errorf with %w"
---

<objective>
Fix error handling inconsistency in gRPC client to use proper error wrapping with %w.

Purpose: Ensure all errors can be unwrapped for inspection, maintaining consistency with the rest of the codebase.
Output: Updated grpc_client.go with proper error wrapping pattern.
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

    1. Check for any remaining %v error formatting:
       `grep -rn "fmt.Errorf.*%v" internal/ --include="*.go" | grep -v "_test.go"`
       Should return empty (no matches found).

    2. Check for naked fmt.Printf/Println:
       `grep -rn "log\.Println\|fmt\.Printf\|fmt\.Println" internal/ --include="*.go" | grep -v "_test.go"`
       Should return empty.

    3. Verify slog usage is consistent:
       `grep -rn "log/slog" internal/ --include="*.go" | wc -l`
       Should show multiple files using slog.

    4. Check for proper error wrapping (%w usage):
       `grep -rn "fmt.Errorf.*%w" internal/ --include="*.go" | wc -l`
       Should show 60+ occurrences.

    Document findings in verification output.
  </action>
  <verify>
    All grep commands return expected results confirming standards compliance.
  </verify>
  <done>
    - No %v error formatting found in production code
    - No naked print statements found
    - slog used consistently across codebase
    - %w wrapping pattern used throughout
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
- ERR-01 satisfied: All errors use fmt.Errorf with %w for wrapping
- ERR-02 satisfied: No redundant error type checks (wrapping handles this)
- ERR-03 satisfied: slog used for structured logging throughout
- ERR-04 satisfied: No fmt.Printf/Println in production code
- All tests pass
- Code compiles without errors
</success_criteria>

<output>
After completion, create `.planning/phases/02-standardize-error-handling/02-01-SUMMARY.md`
</output>
