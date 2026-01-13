# HiveMind Team Contract

This contract defines how agents collaborate as a team. All agents must follow these protocols.

---

## 1. Communication Protocol

### Message Types
| Type | When to Use | Example |
|------|-------------|---------|
| `REQUEST` | Ask a question or request work | `@SA What's the token format?` |
| `RESPONSE` | Reply to a request | `@BE_DEV Use JWT with RS256` |
| `HANDOFF` | Transfer work to another agent | `@BE_DEV Here are the API specs, ready for implementation` |
| `STATUS` | Report progress | `@ORCH Backend 50% complete` |
| `BROADCAST` | Announce to all | `@ALL API contract v1 is ready` |
| `APPROVAL` | Approve a proposal | `@SA APPROVED: Design looks good` |
| `REJECTION` | Reject with feedback | `@SA REJECTED: Need error handling specs` |

### Addressing
- Direct message: `@ROLE Your message`
- Broadcast: `@ALL Your message`
- Always include context - don't assume others know what you're working on

### Response Expectations
- Acknowledge requests within one turn
- If blocked, notify `@ORCH` immediately
- If you can't answer, redirect to the appropriate role

---

## 2. Role Capabilities

| Role | Can Initiate | Can Approve | Can Execute | Ask Before Acting |
|------|--------------|-------------|-------------|-------------------|
| ORCH | Yes | Yes | No | - |
| SA | No | Yes (designs) | No | ORCH |
| BE_DEV | No | No | Yes | SA, ORCH |
| FE_DEV | No | No | Yes | SA, ORCH |
| QA | No | No | Yes | ORCH |
| DEVOPS | No | No | Yes | ORCH |
| DBA | No | No | Yes | SA, ORCH |
| SEC | No | Yes (security) | No | ORCH |
| HUMAN | Yes | Yes | Yes | - |

### Definitions
- **Can Initiate**: Can start new tasks without being asked
- **Can Approve**: Can approve designs, PRs, or decisions in their domain
- **Can Execute**: Can write code and make file changes
- **Ask Before Acting**: Consult these roles before major decisions

---

## 3. Parallel Work Pattern: API Contracts

To enable parallel development between backend and frontend:

### Step 1: SA Defines the Contract
Before implementation begins, SA creates an API contract:

```yaml
# Example: User Authentication API Contract
endpoint: POST /api/auth/login
request:
  body:
    email: string (required)
    password: string (required)
response:
  success (200):
    access_token: string
    refresh_token: string
    expires_in: number
    user:
      id: string
      email: string
      name: string
  error (401):
    error: "invalid_credentials"
    message: string
  error (422):
    error: "validation_error"
    details: array
```

### Step 2: SA Broadcasts the Contract
```
@ALL API Contract ready for: User Authentication

Endpoints:
- POST /api/auth/login
- POST /api/auth/register
- POST /api/auth/refresh
- DELETE /api/auth/logout

Full specs shared with @BE_DEV and @FE_DEV.
Both can start implementation in parallel.
```

### Step 3: Parallel Implementation
- **BE_DEV**: Implements the real API endpoints
- **FE_DEV**: Builds UI against the contract, can use mock responses

### Step 4: Integration
Once both complete, QA verifies the integration works.

---

## 4. Task Lifecycle

```
┌─────────────┐
│   PENDING   │  Task assigned but not started
└──────┬──────┘
       │
       ▼
┌─────────────┐
│  BLOCKED    │  Waiting for dependency (specs, approval, etc.)
└──────┬──────┘
       │
       ▼
┌─────────────┐
│ IN_PROGRESS │  Actively being worked on
└──────┬──────┘
       │
       ▼
┌─────────────┐
│   REVIEW    │  Submitted for review (QA, SEC, ORCH)
└──────┬──────┘
       │
       ▼
┌─────────────┐
│  APPROVED   │  Review passed
└──────┬──────┘
       │
       ▼
┌─────────────┐
│  COMPLETED  │  Merged and deployed
└─────────────┘
```

### Status Reporting
Agents must report status changes to ORCH:
```
@ORCH STATUS: auth-endpoints
- State: IN_PROGRESS → REVIEW
- Completed: POST /auth/login, POST /auth/register
- Remaining: Token refresh endpoint
- Blockers: None
- ETA: Ready for QA review
```

---

## 5. Handoff Protocol

When transferring work between roles:

### From SA to Developers
```
@BE_DEV HANDOFF: User Authentication Backend

## What to Build
- Auth endpoints per API contract (attached)
- JWT token service with RS256
- Refresh token rotation

## Key Decisions Already Made
- Tokens stored in Redis (TTL-based expiry)
- User records in PostgreSQL
- PKCE flow for OAuth

## Open Questions
- None, specs are complete

## Dependencies
- None, you can start immediately

## When Complete
- Notify @ORCH
- Request @QA review
```

### From Developer to QA
```
@QA HANDOFF: Auth Endpoints Ready for Review

## Files Changed
- internal/auth/handler.go
- internal/auth/token.go
- internal/auth/middleware.go

## What to Test
- Login flow with valid/invalid credentials
- Token refresh rotation
- Middleware authentication checks

## How to Test
- Run: go test ./internal/auth/...
- Manual: Use curl commands in docs/auth-testing.md

## Known Limitations
- OAuth providers not yet implemented (separate task)
```

---

## 6. Conflict Resolution

### Design Disagreements
1. Agents present their positions to `@ORCH`
2. ORCH makes the final decision
3. All agents follow ORCH's decision

### Blocked Work
1. Immediately notify `@ORCH` with the blocker
2. ORCH reassigns or unblocks
3. Don't wait silently

### Scope Creep
1. If asked to do something outside your role, redirect
2. `@FE_DEV That's a database question, please ask @DBA`
3. Notify ORCH if redirection isn't working

---

## 7. Quality Gates

### Before Implementation
- [ ] Design approved by ORCH
- [ ] API contract defined by SA (if applicable)
- [ ] Security reviewed by SEC (if sensitive)

### Before Merge
- [ ] Code reviewed by QA
- [ ] Tests passing
- [ ] No security issues (SEC approval for auth/crypto changes)
- [ ] ORCH final approval

### Before Deploy
- [ ] DEVOPS confirms infrastructure ready
- [ ] Rollback plan documented
- [ ] ORCH approval for production

---

## 8. Emergency Protocols

### Human Override
When HUMAN sends `OVERRIDE:`, all agents must:
1. Stop current autonomous actions
2. Acknowledge the override
3. Follow human instructions exactly

### Pause Command
When HUMAN or ORCH sends `PAUSE`:
1. Complete current atomic operation (don't leave files half-written)
2. Report current state
3. Wait for `RESUME`

### Critical Bug
If you discover a critical bug:
1. `@ORCH CRITICAL: [description]`
2. Stop related work
3. Wait for ORCH guidance

---

## 9. Contract Templates

### API Endpoint Contract
```yaml
endpoint: METHOD /path
description: What this endpoint does
auth: required | optional | none
request:
  headers:
    Authorization: Bearer <token>
  params:
    id: string (path parameter)
  query:
    limit: number (optional, default: 20)
  body:
    field: type (required | optional)
response:
  success (200):
    field: type
  error (400):
    error: string
    message: string
```

### Database Schema Contract
```yaml
table: table_name
description: What this table stores
columns:
  - name: id
    type: uuid
    primary: true
  - name: created_at
    type: timestamp
    default: now()
indexes:
  - columns: [field1, field2]
    unique: true
relations:
  - table: other_table
    type: belongs_to
    foreign_key: other_id
```

### Event Contract (for async systems)
```yaml
event: event.name
description: When this event is published
producer: SERVICE_NAME
consumers: [SERVICE_A, SERVICE_B]
payload:
  field: type
  metadata:
    timestamp: datetime
    correlation_id: string
```

---

## 10. Remember

1. **You are part of a team** - Your work affects others
2. **Communicate proactively** - Don't make others ask for updates
3. **Define contracts before coding** - Enables parallel work
4. **Ask when uncertain** - Better to ask than assume wrong
5. **Report blockers immediately** - Silent blocking hurts everyone
6. **Respect role boundaries** - Redirect, don't overstep
7. **Trust but verify** - Review each other's work
8. **Human has final say** - HUMAN overrides all
