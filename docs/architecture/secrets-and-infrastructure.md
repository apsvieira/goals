# Secrets and Infrastructure

## Overview

tiny tracker uses GitHub Actions for CI/CD. Sensitive files and credentials are stored as GitHub Actions secrets and decoded at build time. None of these files should be committed to the repository.

## GitHub Actions Secrets

| Secret | What it is | Used by |
|---|---|---|
| `GOOGLE_SERVICES_JSON` | Base64-encoded `google-services.json` — Firebase project config for the Android app. Contains project identifiers, API keys, and OAuth client IDs that the Firebase SDK needs at build time. | `android-build.yml` (build + release) |
| `KEYSTORE_BASE64` | Base64-encoded `tiny-tracker-upload.keystore` — Android upload signing key. | `android-build.yml` (release) |
| `KEYSTORE_PASSWORD` | Password for the upload keystore. | `android-build.yml` (release) |
| `KEY_ALIAS` | Alias of the signing key within the keystore. | `android-build.yml` (release) |
| `KEY_PASSWORD` | Password for the signing key. | `android-build.yml` (release) |
| `PLAY_PUBLISHER_KEY` | JSON service-account credentials for the Google Play Developer API. Used by Gradle Play Publisher to upload AABs to the internal track. | `android-build.yml` (release) |
| `FLY_API_TOKEN` | API token for Fly.io. Used to deploy the Go backend. | `deploy-backend.yml` |

## Local-only files (gitignored)

| File | Purpose |
|---|---|
| `frontend/android/app/google-services.json` | Firebase config for local builds. Regenerate with `scripts/setup-firebase.sh`. |
| `frontend/android/keystore.properties` | Points Gradle to the local keystore and its passwords. |
| `tiny-tracker-upload.keystore` | Android upload signing keystore. |
| `.env` | Backend environment variables (Google OAuth client ID/secret, database URL). |
| `play-publisher-key.json` | Google Play service-account key for local Gradle Play Publisher runs. |

## Rotating or recreating secrets

### google-services.json

Regenerate from Firebase and update the GitHub secret:

```bash
bash scripts/setup-firebase.sh
base64 -w0 frontend/android/app/google-services.json | gh secret set GOOGLE_SERVICES_JSON
```

### Upload keystore

If the upload keystore is lost, a new one must be created and re-enrolled with Google Play via Play App Signing. Then update all keystore-related secrets.

### Play Publisher key

Create a new service-account key in the GCP console for project `tiny-tracker-f97da`, then:

```bash
gh secret set PLAY_PUBLISHER_KEY < play-publisher-key.json
```
