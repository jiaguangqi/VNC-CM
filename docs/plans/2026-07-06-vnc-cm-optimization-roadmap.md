# VNC-CM Optimization Roadmap Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Improve VNC-CM from a working single-machine VNC manager into a more reliable, observable, and easier-to-operate desktop platform.

**Architecture:** Keep the current Master Service + PostgreSQL + frontend + Host Agent shape. First harden the existing SSH-based VNC lifecycle, then move VNC start/stop responsibility toward Host Agent once lifecycle behavior is measurable and stable.

**Tech Stack:** Go 1.23, Gin, GORM, PostgreSQL, React 18, TypeScript, Vite, Ant Design, Docker Compose, Linux VNC/TigerVNC/TurboVNC, noVNC/websockify.

---

## Release Strategy

- Use patch releases for low-risk fixes: `v1.1.x`.
- Use minor releases for behavior or architecture changes: `v1.2.0`, `v1.3.0`.
- Keep each task independently deployable.
- For every task, run:
  - `npm run build` in `frontend`
  - `go test ./...` in `master-service`
  - `GOOS=linux GOARCH=amd64 go test ./...` in `host-agent` when Host Agent changes
  - `go test ./...` in `ssh-gateway` when SSH Gateway changes
  - `git diff --check`

## Phase 1: Desktop Lifecycle Reliability

### Task 1: Normalize Session Status Transitions

**Goal:** Make desktop status changes explicit and consistent.

**Files:**
- Modify: `master-service/models/models.go`
- Modify: `master-service/handlers/desktop.go`
- Modify: `frontend/src/pages/DesktopsPage.tsx`
- Modify: `README.md`

**Steps:**
1. Add constants or a small helper for valid session statuses: `pending`, `starting`, `running`, `stopping`, `terminated`, `error`.
2. Change `CreateDesktop` to create a session as `starting` before running remote commands.
3. Update the session to `running` only after VNC and websockify start successfully.
4. Update the session to `error` and store an error message in `connection_info` if startup fails after the session row exists.
5. Change close/delete flows to use `stopping` before cleanup and `terminated` after cleanup.
6. Update frontend tags to render `starting`, `stopping`, and `error`.
7. Run validation commands.
8. Commit:
   ```bash
   git add master-service/models/models.go master-service/handlers/desktop.go frontend/src/pages/DesktopsPage.tsx README.md
   git commit -m "fix: normalize desktop lifecycle states"
   ```

### Task 2: Add Actual Process Health Checks

**Goal:** Prevent database sessions from saying `running` when VNC/websockify is dead.

**Files:**
- Modify: `master-service/handlers/desktop.go`
- Create: `master-service/services/desktop_health.go`
- Modify: `master-service/main.go`
- Modify: `frontend/src/pages/DesktopsPage.tsx`

**Steps:**
1. Implement a remote SSH check that verifies the selected display and websockify port are alive.
2. Add a periodic scheduler in Master Service that checks active sessions every 60 seconds.
3. Mark sessions as `error` when the process check fails.
4. Decrement `hosts.current_sessions` only once per session transition out of `running`.
5. Show `error` sessions clearly in the desktop list.
6. Run validation commands.
7. Commit:
   ```bash
   git add master-service/services/desktop_health.go master-service/handlers/desktop.go master-service/main.go frontend/src/pages/DesktopsPage.tsx
   git commit -m "fix: reconcile desktop process health"
   ```

### Task 3: Make Stop Cleanup Idempotent

**Goal:** Closing a desktop should be safe to retry.

**Files:**
- Modify: `master-service/handlers/desktop.go`

**Steps:**
1. Make `stopVNCOnHost` tolerate missing VNC display and missing websockify process.
2. Always return a structured cleanup result internally.
3. Ensure repeated close/delete requests do not decrement `current_sessions` multiple times.
4. Run validation commands.
5. Commit:
   ```bash
   git add master-service/handlers/desktop.go
   git commit -m "fix: make desktop cleanup idempotent"
   ```

## Phase 2: Port and Display Allocation

### Task 4: Reuse Free Displays Safely

**Goal:** Avoid unbounded display and port growth.

**Files:**
- Modify: `master-service/handlers/desktop.go`
- Create: `master-service/services/display_allocator.go`

**Steps:**
1. Add a helper that lists active displays for a host from non-terminal sessions.
2. Allocate the lowest available display starting from `1`.
3. Keep port mapping as `5900 + display` and `6100 + display`.
4. Reject allocation if all displays up to host capacity are occupied.
5. Run validation commands.
6. Commit:
   ```bash
   git add master-service/handlers/desktop.go master-service/services/display_allocator.go
   git commit -m "fix: reuse available VNC displays"
   ```

### Task 5: Add Port Conflict Preflight

**Goal:** Fail early when the chosen node already has occupied ports.

**Files:**
- Modify: `master-service/handlers/desktop.go`

**Steps:**
1. Before starting VNC, SSH to the host and check candidate VNC/websockify ports.
2. Try the next available display when a port is occupied.
3. Return a clear error if no usable display is available.
4. Run validation commands.
5. Commit:
   ```bash
   git add master-service/handlers/desktop.go
   git commit -m "fix: avoid VNC port conflicts"
   ```

## Phase 3: User and Node Readiness

### Task 6: Add Node Readiness API

**Goal:** Make failures visible before users request a desktop.

**Files:**
- Create: `master-service/services/node_readiness.go`
- Modify: `master-service/handlers/host.go`
- Modify: `master-service/main.go`
- Modify: `frontend/src/pages/DesktopsPage.tsx`
- Modify: `frontend/src/pages/HostsPage.tsx`

**Steps:**
1. Add checks for SSH connectivity, current login user existence, `vncserver`, `vncpasswd`, `websockify`, noVNC path, GNOME, and XFCE.
2. Expose a user-safe readiness summary on `/api/v1/desktop-hosts`.
3. Expose a fuller admin readiness endpoint on `/api/v1/hosts/:id/readiness`.
4. Show readiness warnings in the user node dropdown.
5. Show detailed readiness diagnostics on the admin hosts page.
6. Run validation commands.
7. Commit:
   ```bash
   git add master-service/services/node_readiness.go master-service/handlers/host.go master-service/main.go frontend/src/pages/DesktopsPage.tsx frontend/src/pages/HostsPage.tsx
   git commit -m "feat: show desktop node readiness"
   ```

### Task 7: Add User-to-Node Compatibility Check

**Goal:** Make it clear whether the current system user can run on a selected node.

**Files:**
- Modify: `master-service/handlers/host.go`
- Modify: `master-service/handlers/desktop.go`
- Modify: `frontend/src/pages/DesktopsPage.tsx`

**Steps:**
1. Include `current_user_exists` in `/api/v1/desktop-hosts` results.
2. Disable host options where the current user does not exist.
3. Keep backend validation as the source of truth.
4. Return a clear error when selected host lacks the same Linux user.
5. Run validation commands.
6. Commit:
   ```bash
   git add master-service/handlers/host.go master-service/handlers/desktop.go frontend/src/pages/DesktopsPage.tsx
   git commit -m "feat: validate user availability on desktop nodes"
   ```

## Phase 4: Quotas and Permissions

### Task 8: Add Per-User Desktop Limits

**Goal:** Prevent one user from exhausting all nodes.

**Files:**
- Modify: `master-service/models/models.go`
- Modify: `master-service/database/db.go`
- Modify: `master-service/handlers/desktop.go`
- Modify: `README.md`

**Steps:**
1. Add config defaults for maximum running desktops per user.
2. Before creating a desktop, count current user sessions in `running` or `starting`.
3. Reject requests beyond the configured quota.
4. Document the environment variable.
5. Run validation commands.
6. Commit:
   ```bash
   git add master-service/models/models.go master-service/database/db.go master-service/handlers/desktop.go README.md
   git commit -m "feat: enforce per-user desktop quota"
   ```

### Task 9: Add Node Access Policy

**Goal:** Allow admins to restrict which users can use which nodes.

**Files:**
- Modify: `master-service/models/models.go`
- Create: `master-service/handlers/node_policy.go`
- Modify: `master-service/main.go`
- Modify: `frontend/src/pages/HostsPage.tsx`
- Modify: `frontend/src/pages/DesktopsPage.tsx`

**Steps:**
1. Add a small policy model for allowed users or roles per host.
2. Filter `/api/v1/desktop-hosts` by policy.
3. Enforce policy in `CreateDesktop`.
4. Add admin UI for basic allow-list editing.
5. Run validation commands.
6. Commit:
   ```bash
   git add master-service/models/models.go master-service/handlers/node_policy.go master-service/main.go frontend/src/pages/HostsPage.tsx frontend/src/pages/DesktopsPage.tsx
   git commit -m "feat: add desktop node access policy"
   ```

## Phase 5: Audit and Operations

### Task 10: Write Audit Events

**Goal:** Track important actions for debugging and accountability.

**Files:**
- Create: `master-service/services/audit.go`
- Modify: `master-service/handlers/auth.go`
- Modify: `master-service/handlers/desktop.go`
- Modify: `master-service/handlers/host.go`
- Modify: `master-service/handlers/collaboration.go`

**Steps:**
1. Add a small audit service that writes `models.AuditLog`.
2. Log login success/failure.
3. Log desktop create/close/delete.
4. Log host create/update/delete.
5. Log collaboration invite/stop.
6. Run validation commands.
7. Commit:
   ```bash
   git add master-service/services/audit.go master-service/handlers/auth.go master-service/handlers/desktop.go master-service/handlers/host.go master-service/handlers/collaboration.go
   git commit -m "feat: record audit events"
   ```

### Task 11: Add Deployment Self-Check Command

**Goal:** Give admins a single command to diagnose the server and nodes.

**Files:**
- Create: `master-service/cmd/selfcheck/main.go`
- Modify: `install.sh`
- Modify: `README.md`

**Steps:**
1. Implement checks for required environment variables.
2. Check database connectivity.
3. Check Docker Compose service health when run on the server.
4. Check registered hosts and readiness status.
5. Add `./install.sh selfcheck`.
6. Run validation commands.
7. Commit:
   ```bash
   git add master-service/cmd/selfcheck/main.go install.sh README.md
   git commit -m "feat: add deployment self check"
   ```

## Phase 6: Host Agent Ownership of VNC Lifecycle

### Task 12: Implement Real Agent VNC Start

**Goal:** Move desktop start away from Master SSH commands.

**Files:**
- Modify: `host-agent/agent/agent.go`
- Create: `host-agent/agent/vnc_linux.go`
- Modify: `master-service/grpc/server.go`
- Modify: `master-service/handlers/desktop.go`

**Steps:**
1. Define the create desktop instruction payload with display, ports, backend, resolution, color depth, desktop environment, and password.
2. Implement Linux VNC start in Host Agent using local commands.
3. Send desktop status updates from Agent to Master.
4. In Master, use Agent instruction path when the node is online.
5. Keep SSH path as a fallback during the transition.
6. Run validation commands.
7. Commit:
   ```bash
   git add host-agent/agent/agent.go host-agent/agent/vnc_linux.go master-service/grpc/server.go master-service/handlers/desktop.go
   git commit -m "feat: start VNC desktops through host agent"
   ```

### Task 13: Implement Real Agent VNC Stop

**Goal:** Move desktop stop cleanup to Host Agent.

**Files:**
- Modify: `host-agent/agent/agent.go`
- Modify: `host-agent/agent/vnc_linux.go`
- Modify: `master-service/grpc/server.go`
- Modify: `master-service/handlers/desktop.go`

**Steps:**
1. Implement local VNC and websockify stop in Host Agent.
2. Send terminal status updates back to Master.
3. Make Master close/delete use Agent when possible.
4. Keep SSH cleanup as fallback for offline Agent.
5. Run validation commands.
6. Commit:
   ```bash
   git add host-agent/agent/agent.go host-agent/agent/vnc_linux.go master-service/grpc/server.go master-service/handlers/desktop.go
   git commit -m "feat: stop VNC desktops through host agent"
   ```

### Task 14: Remove SSH Requirement for Agent-Managed Nodes

**Goal:** Reduce credential risk once Agent lifecycle is stable.

**Files:**
- Modify: `master-service/models/models.go`
- Modify: `master-service/handlers/host.go`
- Modify: `master-service/handlers/desktop.go`
- Modify: `frontend/src/pages/HostsPage.tsx`
- Modify: `README.md`

**Steps:**
1. Allow hosts to be marked as `agent_managed`.
2. Do not require SSH credentials for `agent_managed` hosts.
3. Keep SSH fields for legacy/fallback nodes.
4. Update host admin UI to explain the difference.
5. Update docs and deployment guidance.
6. Run validation commands.
7. Commit:
   ```bash
   git add master-service/models/models.go master-service/handlers/host.go master-service/handlers/desktop.go frontend/src/pages/HostsPage.tsx README.md
   git commit -m "feat: support agent-managed desktop nodes"
   ```

## Phase 7: Frontend Polish

### Task 15: Improve Desktop Connection UX

**Goal:** Make connection details easier to use.

**Files:**
- Modify: `frontend/src/pages/DesktopsPage.tsx`

**Steps:**
1. Add copy buttons for VNC URL, web URL, host IP, port, and password.
2. Add clearer reconnect and full-screen actions.
3. Show desktop lifecycle status in the connection modal.
4. Run validation commands.
5. Commit:
   ```bash
   git add frontend/src/pages/DesktopsPage.tsx
   git commit -m "feat: improve desktop connection actions"
   ```

### Task 16: Improve Host Selection UX

**Goal:** Help users choose the right node when not using auto scheduling.

**Files:**
- Modify: `frontend/src/pages/DesktopsPage.tsx`

**Steps:**
1. Show node load, region, AZ, CPU/RAM, and readiness in the dropdown.
2. Keep the default option as automatic scheduling.
3. Add disabled reasons for unavailable nodes.
4. Run validation commands.
5. Commit:
   ```bash
   git add frontend/src/pages/DesktopsPage.tsx
   git commit -m "feat: improve desktop node picker"
   ```

## Phase 8: VNC Bandwidth and Performance Optimization

### Task 17: Add VNC Performance Profiles

**Goal:** Reduce network bandwidth for users on weak links while keeping a high-quality option for LAN users.

**Files:**
- Modify: `master-service/handlers/desktop.go`
- Modify: `master-service/models/models.go`
- Modify: `frontend/src/pages/DesktopsPage.tsx`
- Modify: `README.md`

**Steps:**
1. Add a `performance_profile` request field with values `quality`, `balanced`, and `low_bandwidth`.
2. Keep `balanced` as the default.
3. Map profiles to resolution, color depth, desktop environment hints, and VNC backend-specific startup options where supported.
4. In the frontend create-desktop form, add a profile selector with `均衡` as default.
5. Document what each profile changes.
6. Run validation commands.
7. Commit:
   ```bash
   git add master-service/handlers/desktop.go master-service/models/models.go frontend/src/pages/DesktopsPage.tsx README.md
   git commit -m "feat: add VNC performance profiles"
   ```

### Task 18: Add Lightweight Desktop Defaults

**Goal:** Reduce continuous screen updates caused by heavy desktop effects.

**Files:**
- Modify: `master-service/handlers/desktop.go`
- Modify: `README.md`

**Steps:**
1. Prefer XFCE for `low_bandwidth` unless the user explicitly selects another desktop environment.
2. Add xstartup templates that disable compositing or heavy visual effects when possible.
3. Keep GNOME available for users who need it.
4. Run validation commands.
5. Commit:
   ```bash
   git add master-service/handlers/desktop.go README.md
   git commit -m "feat: prefer lightweight desktops for low bandwidth sessions"
   ```

### Task 19: Add Per-Session Bandwidth Visibility

**Goal:** Make VNC bandwidth usage measurable instead of anecdotal.

**Files:**
- Modify: `host-agent/agent/agent.go`
- Modify: `master-service/models/models.go`
- Modify: `master-service/grpc/server.go`
- Modify: `master-service/handlers/stats.go`
- Modify: `frontend/src/pages/DashboardPage.tsx`
- Modify: `frontend/src/pages/DesktopsPage.tsx`

**Steps:**
1. Track per-session network bytes where the process ownership and ports can be identified safely.
2. Report session bandwidth from Host Agent to Master.
3. Store recent bandwidth samples or aggregate counters.
4. Show current bandwidth and peak bandwidth in desktop cards/details.
5. Add dashboard summary for top bandwidth sessions.
6. Run validation commands.
7. Commit:
   ```bash
   git add host-agent/agent/agent.go master-service/models/models.go master-service/grpc/server.go master-service/handlers/stats.go frontend/src/pages/DashboardPage.tsx frontend/src/pages/DesktopsPage.tsx
   git commit -m "feat: show VNC session bandwidth usage"
   ```

### Task 20: Add Adaptive Low-Bandwidth Recommendations

**Goal:** Help users choose better settings before wasting bandwidth.

**Files:**
- Modify: `frontend/src/pages/DesktopsPage.tsx`
- Modify: `master-service/handlers/desktop.go`

**Steps:**
1. Show estimated bandwidth impact when users pick high resolution, 24-bit color, GNOME, or quality mode.
2. Recommend `low_bandwidth` when users select remote nodes across regions or when recent session bandwidth is high.
3. Keep recommendations advisory; do not silently override user choices.
4. Run validation commands.
5. Commit:
   ```bash
   git add frontend/src/pages/DesktopsPage.tsx master-service/handlers/desktop.go
   git commit -m "feat: recommend low bandwidth desktop settings"
   ```

## Recommended Execution Order

1. Phase 1: lifecycle reliability.
2. Phase 2: display and port allocation.
3. Phase 3: readiness and user-node compatibility.
4. Phase 4: quotas and permissions.
5. Phase 5: audit and self-check.
6. Phase 6: Host Agent lifecycle ownership.
7. Phase 7: frontend polish.
8. Phase 8: VNC bandwidth and performance optimization.

## First Work Item

Start with **Task 1: Normalize Session Status Transitions**. It has the best risk-to-value ratio: it improves correctness immediately and prepares the codebase for health checks, quotas, and Agent-managed lifecycle later.
