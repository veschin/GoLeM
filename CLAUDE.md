<!-- ---ptsd--- -->
# Claude Agent Instructions

## Authority Hierarchy (ENFORCED BY HOOKS)

PTSD (iron law) > User (context provider) > Assistant (executor)

- PTSD decides what CAN and CANNOT be done. Pipeline, gates, validation — non-negotiable.
  Hooks enforce this automatically — writes that violate pipeline are BLOCKED.
- User provides context and requirements. User also follows ptsd rules.
- Assistant executes within ptsd constraints. Writes code, docs, tests on behalf of user.

## Session Start Protocol

EVERY session, BEFORE any work:
1. Run: ptsd context --agent — see full pipeline state
2. Run: ptsd task next --agent — get next task
3. Follow output exactly.

## Commands (always use --agent flag)

- ptsd context --agent              — full pipeline state (auto-injected by hooks)
- ptsd status --agent               — project overview
- ptsd task next --agent            — next task to work on
- ptsd task update <id> --status WIP — mark task in progress
- ptsd validate --agent             — check pipeline before commit
- ptsd feature list --agent         — list all features
- ptsd seed init <id> --agent       — initialize seed directory
- ptsd gate-check --file <path> --agent — check if file write is allowed

## Skills

PTSD pipeline skills are in `.claude/skills/` — auto-loaded when relevant.

| Skill | When to Use |
|-------|------------|
| write-prd | Creating or updating a PRD section |
| write-seed | Creating seed data for a feature |
| write-bdd | Writing Gherkin BDD scenarios |
| write-tests | Writing tests from BDD scenarios |
| write-impl | Implementing to make tests pass |
| create-tasks | Adding tasks to tasks.yaml |
| review-prd | Reviewing PRD before advancing to seed |
| review-seed | Reviewing seed data before advancing to bdd |
| review-bdd | Reviewing BDD before advancing to tests |
| review-tests | Reviewing tests before advancing to impl |
| review-impl | Reviewing implementation after tests pass |
| workflow | Session start or when unsure what to do next |
| adopt | Bootstrapping existing project into PTSD |

Use the corresponding write skill, then review skill at each pipeline stage.

## Pipeline (strict order, no skipping)

PRD → Seed → BDD → Tests → Implementation

Each stage requires review score ≥ 7 before advancing.
Hooks enforce gates automatically — blocked writes show the reason.

## Rules

- NO mocks for internal code. Real tests, real files, temp directories.
- NO garbage files. Every file must link to a feature.
- NO hiding errors. Explain WHY something failed.
- NO over-engineering. Minimum code for the current task.
- ALWAYS run: ptsd validate --agent before committing.
- COMMIT FORMAT: [SCOPE] type: message
  Scopes: PRD, SEED, BDD, TEST, IMPL, TASK, STATUS
  Types: feat, add, fix, refactor, remove, update

## Troubleshooting

When ptsd status/validate shows unexpected results, debug with these steps:

| Symptom | Cause | Fix |
|---------|-------|-----|
| TESTS:0 but test files exist | Tests not mapped to features | `ptsd test map .ptsd/bdd/<id>.feature <test-file>` for each feature |
| BDD:0 but .feature files exist | State hashes empty, SyncState not run | `ptsd status --agent` triggers sync; if still 0, check `.ptsd/bdd/<id>.feature` has `@feature:<id>` tag on line 1 |
| Feature stuck at wrong stage | review-status.yaml stale or stage not advanced | Run `ptsd review <id> <stage> <score>` to advance; check `ptsd context --agent` for blockers |
| "no test files mapped" on `ptsd test run` | Test mapping missing in state.yaml | `ptsd test map .ptsd/bdd/<id>.feature <test-file>` |
| Gate blocks file write | File not in allowed list for current stage | Check `ptsd gate-check --file <path> --agent`; advance feature to correct stage first |
| Validate shows "mock detected" | Test file contains mock/stub patterns | Replace mocks with real file-based tests in temp directories |
| Regression warning on status | Artifact file changed after stage was reviewed | Re-review the stage: `ptsd review <id> <stage> <score>` |

### Debug flow
1. `ptsd context --agent` — shows next action, blockers, stage per feature
2. `ptsd feature show <id> --agent` — shows artifact counts and test stats
3. `ptsd validate --agent` — shows all pipeline violations
4. Check `.ptsd/state.yaml` — hashes, test mappings, stages
5. Check `.ptsd/review-status.yaml` — review verdicts per feature

### Test mapping
Each feature needs: BDD file (`.ptsd/bdd/<id>.feature`) with `@feature:<id>` tag → mapped to test file via `ptsd test map`. Without mapping, ptsd cannot track test results per feature.

## Forbidden

- Mocking internal code
- Skipping pipeline steps
- Hiding errors or pretending something works
- Generating files not linked to a feature
- Using --force, --skip-validation, --no-verify

<!-- ---ptsd--- -->
