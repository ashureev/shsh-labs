# Branch Protection Rules for SHSH
# Apply these settings in GitHub Repository Settings > Branches

## Main Branch Protection

To configure branch protection for `main`:

1. Go to **Settings** -> **Branches** -> **Add rule**
2. Branch name pattern: `main`

### Recommended Settings

```yaml
require_status_checks:
  strict: true
  contexts:
    - "CI Required / ci-required"

# Required pull request reviews
require_pull_request_reviews:
  required_approving_review_count: 1
  dismiss_stale_reviews: true
  require_code_owner_reviews: false

# Other protections
require_signed_commits: false
require_linear_history: false
allow_force_pushes: false
allow_deletions: false

# Bypass permissions (optional)
bypass_pull_request_allowances:
  users: []
```

### Manual Quality Gate

Before merging any PR, run local checks:

```bash
go test ./...
npm run lint
go build ./cmd/server
```

### Optional Heavy Validation (Manual Workflow)

For expensive checks (to preserve private-minute budget), run the manual workflow:

- **Workflow**: `Full Stack Manual`
- **Inputs**:
  - `run_python_tests`
  - `run_frontend_tests`
  - `run_docker_builds`

This keeps required PR checks fast while still allowing full-stack validation on demand.

### What This Protects

| Protection | Effect |
|------------|--------|
| **PR Reviews** | At least 1 approval required |
| **Dismiss Stale** | Re-approval needed after new commits |
| **No Force Push** | History cannot be rewritten |
| **No Deletions** | Branch cannot be deleted |
