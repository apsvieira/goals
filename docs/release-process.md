# Release Process

## Prerequisites

Tags must be cut from `main`. The tag SHA must equal the current HEAD of `main` so both the backend deploy and the Android build use the same code.

## Triggering a release

Push a tag matching `vX.Y.Z` (strict semver, e.g. `v1.2.0`):

```bash
git tag v1.2.0
git push origin v1.2.0
```

This triggers the `Android Build` workflow. The jobs run in this order:

1. **Backend tests** — `go test -race ./...` runs inside the `deploy-backend` job.
2. **Backend deploy** — `flyctl deploy --remote-only` pushes the tagged SHA to Fly.io (`goal-tracker-app`). The server auto-applies any pending SQL migrations on startup.
3. **Android release** — only starts after `deploy-backend` succeeds. Builds a signed AAB and publishes it to the Play Store internal track.

## If backend deploy fails

The `release` job is skipped. No AAB is published. Fix the failing test or deploy issue on `main`, then re-tag (or re-run the failed workflow run if the issue was transient). Re-running is safe:

- `flyctl deploy` is idempotent — if the same image is already live, it no-ops.
- `publishBundle` rejects a duplicate `versionCode`, so you must bump the tag if the AAB was already accepted by Play.

## Main-branch deploys

The separate `deploy-backend.yml` workflow still triggers on every push to `main`. Merges to `main` deploy the backend independently, without waiting for a tag. Tag-triggered deploys are therefore usually no-ops because `main` was already deployed.
