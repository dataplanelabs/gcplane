---
phase: 4
priority: high
effort: M
status: pending
---

# Phase 4: Provider — WS Resources

## Overview

Implement Observe/Create/Update for WS-only resources: CronJob, Team, TTSConfig.

## Related Code Files

- **Create:** `internal/provider/goclaw/cron_jobs.go`
- **Create:** `internal/provider/goclaw/teams.go`
- **Create:** `internal/provider/goclaw/tts_config.go`
- **Modify:** `internal/provider/goclaw/provider.go` — add WSClient, update routing

## Key Insights

### Cron Jobs
- `cron.list` → `{ jobs: [...], status: {...} }` — filter by name/id
- `cron.create` → `{ name, schedule: {kind, expr, tz}, message, deliver, channel, to, agentId }`
- `cron.update` → `{ jobId, patch: {...} }`
- Natural key: `name` (kebab-case slug, same as job ID)

### Teams
- `teams.list` → list teams
- `teams.create` → `{ name, displayName }`
- `teams.update` → `{ teamId, patch: {...} }`

### TTS Config
- `tts.get` → current TTS config
- `tts.set` → `{ mode, provider, voice, ... }`
- Single global resource (key: "global")

## Implementation Steps

1. Update `Provider` struct to hold `*WSClient` (lazy-connect on first WS call)

2. **cron_jobs.go**:
   - `observeCronJob(key)`: Call `cron.list` → filter by name
   - `createCronJob(key, spec)`: Call `cron.create`
   - `updateCronJob(key, spec)`: Call `cron.update` with jobId

3. **teams.go**:
   - `observeTeam(key)`: Call `teams.list` → filter by name
   - `createTeam(key, spec)`: Call `teams.create`
   - `updateTeam(key, spec)`: Call `teams.update`

4. **tts_config.go**:
   - `observeTTSConfig(key)`: Call `tts.get`
   - `createTTSConfig(key, spec)`: Call `tts.set` (same as update)
   - `updateTTSConfig(key, spec)`: Call `tts.set`

5. Update `provider.go` Observe/Create/Update switch to route CronJob/Team/TTSConfig

## Todo

- [ ] Add WSClient to Provider (lazy init)
- [ ] cron_jobs.go — observe/create/update via WS
- [ ] teams.go — observe/create/update via WS
- [ ] tts_config.go — observe/update via WS
- [ ] Update provider.go routing
- [ ] Compile check

## Success Criteria

- Can observe/create/update cron jobs via WS RPC
- Can observe/create/update teams via WS RPC
- Can observe/update TTS config via WS RPC
- WS connection lazy-initialized on first WS resource
