# Tag-Triggered Play Store Draft Release

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Automatically build and publish a Play Store draft when a semver git tag (e.g., `v1.0.3`) is pushed, deriving version info from the tag.

**Architecture:** Modify the existing `android-build.yml` workflow to add a tag-push trigger on the release job, with a step that parses the tag into `versionName` and `versionCode`. Update `build.gradle` to accept version overrides via Gradle project properties, falling back to hardcoded defaults for local dev.

**Tech Stack:** GitHub Actions, Gradle (play-publisher plugin), shell scripting for tag parsing.

---

### Task 1: Update `build.gradle` to accept version from Gradle properties

**Files:**
- Modify: `frontend/android/app/build.gradle:13-14`

**Step 1: Modify `build.gradle` to read version from project properties with fallback**

Replace the hardcoded `versionCode` and `versionName` in `defaultConfig` with property lookups that fall back to current values:

```groovy
        versionCode project.hasProperty('versionCode') ? project.property('versionCode').toInteger() : 3
        versionName project.hasProperty('versionName') ? project.property('versionName') : "1.0.2"
```

These two lines replace the existing lines 13-14 in `frontend/android/app/build.gradle`:
```groovy
        versionCode 3
        versionName "1.0.2"
```

This means local builds still work with defaults, but CI can override via `-PversionCode=10003 -PversionName=1.0.3`.

**Step 2: Commit**

```bash
git add frontend/android/app/build.gradle
git commit -m "feat(android): accept versionCode/versionName from gradle properties"
```

---

### Task 2: Update workflow to trigger on version tags

**Files:**
- Modify: `.github/workflows/android-build.yml`

**Step 1: Replace the `workflow_dispatch` trigger on the release job with a tag trigger**

Change the top-level `on:` block from:

```yaml
on:
  push:
    branches:
      - main
    paths:
      - 'frontend/**'
      - '.github/workflows/android-build.yml'
  workflow_dispatch:
```

To:

```yaml
on:
  push:
    branches:
      - main
    paths:
      - 'frontend/**'
      - '.github/workflows/android-build.yml'
    tags-ignore:
      - '**'
  push:
    tags:
      - 'v[0-9]+.[0-9]+.[0-9]+'
```

**WAIT** — YAML doesn't allow duplicate `push` keys. Instead, use this structure:

```yaml
on:
  push:
    branches:
      - main
    paths:
      - 'frontend/**'
      - '.github/workflows/android-build.yml'
  push:
    tags:
      - 'v[0-9]+.[0-9]+.[0-9]+'
```

**ACTUALLY** — this still won't work. GitHub Actions merges all push events. The correct approach is a single `push` trigger that matches both, and use job-level `if` conditions to control which jobs run.

Replace the entire `on:` block with:

```yaml
on:
  push:
    branches:
      - main
    paths:
      - 'frontend/**'
      - '.github/workflows/android-build.yml'
    tags:
      - 'v[0-9]+.[0-9]+.[0-9]+'
```

Then update the job conditions:
- `build` job: add `if: github.ref_type != 'tag'` (only runs on branch pushes)
- `release` job: change `if: github.event_name == 'workflow_dispatch'` to `if: github.ref_type == 'tag'`

**Note on `paths` + `tags` interaction:** When GitHub sees a tag push, it ignores the `paths` filter, so tag pushes will correctly trigger the workflow regardless of which files changed. The `paths` filter only applies to branch pushes.

**Step 2: Commit**

```bash
git add .github/workflows/android-build.yml
git commit -m "ci(android): trigger release job on version tag push"
```

---

### Task 3: Add version parsing and pass to Gradle

**Files:**
- Modify: `.github/workflows/android-build.yml` (release job)

**Step 1: Add a version parsing step at the top of the release job**

Add this as the first step of the `release` job (before checkout):

```yaml
      - name: Parse version from tag
        id: version
        run: |
          TAG="${GITHUB_REF#refs/tags/v}"
          IFS='.' read -r MAJOR MINOR PATCH <<< "$TAG"
          VERSION_CODE=$((MAJOR * 10000 + MINOR * 100 + PATCH))
          echo "version_name=$TAG" >> "$GITHUB_OUTPUT"
          echo "version_code=$VERSION_CODE" >> "$GITHUB_OUTPUT"
          echo "Parsed tag v$TAG → versionName=$TAG, versionCode=$VERSION_CODE"
```

**Step 2: Pass version properties to the Gradle build commands**

Update the "Build release AAB" step from:

```yaml
      - name: Build release AAB
        working-directory: frontend/android
        run: ./gradlew bundleRelease
```

To:

```yaml
      - name: Build release AAB
        working-directory: frontend/android
        run: ./gradlew bundleRelease -PversionName=${{ steps.version.outputs.version_name }} -PversionCode=${{ steps.version.outputs.version_code }}
```

Update the "Publish to internal track" step from:

```yaml
      - name: Publish to internal track
        working-directory: frontend/android
        run: ./gradlew publishBundle
```

To:

```yaml
      - name: Publish to internal track
        working-directory: frontend/android
        run: ./gradlew publishBundle -PversionName=${{ steps.version.outputs.version_name }} -PversionCode=${{ steps.version.outputs.version_code }}
```

**Step 3: Commit**

```bash
git add .github/workflows/android-build.yml
git commit -m "ci(android): parse version from tag and pass to gradle build"
```

---

### Task 4: Test the workflow

**Step 1: Push changes to main**

```bash
git push origin main
```

Verify the `build` job does NOT run (no frontend file changes, unless the workflow file itself changed — in which case it should run the build job only, not the release job).

**Step 2: Create and push a test tag**

```bash
git tag v1.0.3
git push origin v1.0.3
```

**Step 3: Verify in GitHub Actions**

- The `release` job should trigger
- The `build` job should NOT run
- Check the "Parse version from tag" step output: `versionName=1.0.3, versionCode=10003`
- Check that the AAB is built and published as a draft to the internal track

**Step 4: Verify in Google Play Console**

- Navigate to the internal testing track
- Confirm a new draft release exists with version 1.0.3 (10003)

---

## Reference: Complete final workflow

For clarity, here's what the final `.github/workflows/android-build.yml` should look like:

```yaml
name: Android Build

on:
  push:
    branches:
      - main
    paths:
      - 'frontend/**'
      - '.github/workflows/android-build.yml'
    tags:
      - 'v[0-9]+.[0-9]+.[0-9]+'

jobs:
  build:
    runs-on: ubuntu-latest
    if: github.ref_type != 'tag'

    steps:
      - name: Checkout code
        uses: actions/checkout@v4

      - name: Setup Node.js
        uses: actions/setup-node@v4
        with:
          node-version: '22'
          cache: 'npm'
          cache-dependency-path: frontend/package-lock.json

      - name: Setup Java
        uses: actions/setup-java@v4
        with:
          distribution: 'temurin'
          java-version: '21'

      - name: Setup Gradle
        uses: gradle/actions/setup-gradle@v4
        with:
          build-root-directory: frontend/android

      - name: Install frontend dependencies
        working-directory: frontend
        run: npm ci

      - name: Build frontend
        working-directory: frontend
        run: npm run build

      - name: Sync Capacitor
        working-directory: frontend
        run: npx cap sync android

      - name: Build debug APK
        working-directory: frontend/android
        run: ./gradlew assembleDebug

      - name: Upload APK artifact
        uses: actions/upload-artifact@v4
        with:
          name: debug-apk
          path: frontend/android/app/build/outputs/apk/debug/*.apk
          retention-days: 14

  release:
    runs-on: ubuntu-latest
    if: github.ref_type == 'tag'

    steps:
      - name: Parse version from tag
        id: version
        run: |
          TAG="${GITHUB_REF#refs/tags/v}"
          IFS='.' read -r MAJOR MINOR PATCH <<< "$TAG"
          VERSION_CODE=$((MAJOR * 10000 + MINOR * 100 + PATCH))
          echo "version_name=$TAG" >> "$GITHUB_OUTPUT"
          echo "version_code=$VERSION_CODE" >> "$GITHUB_OUTPUT"
          echo "Parsed tag v$TAG → versionName=$TAG, versionCode=$VERSION_CODE"

      - name: Checkout code
        uses: actions/checkout@v4

      - name: Setup Node.js
        uses: actions/setup-node@v4
        with:
          node-version: '22'
          cache: 'npm'
          cache-dependency-path: frontend/package-lock.json

      - name: Setup Java
        uses: actions/setup-java@v4
        with:
          distribution: 'temurin'
          java-version: '21'

      - name: Setup Gradle
        uses: gradle/actions/setup-gradle@v4
        with:
          build-root-directory: frontend/android

      - name: Decode keystore
        run: echo "${{ secrets.KEYSTORE_BASE64 }}" | base64 -d > frontend/tiny-tracker-upload.keystore

      - name: Create keystore.properties
        working-directory: frontend/android
        run: |
          cat > keystore.properties <<EOF
          storeFile=../../tiny-tracker-upload.keystore
          storePassword=${{ secrets.KEYSTORE_PASSWORD }}
          keyAlias=${{ secrets.KEY_ALIAS }}
          keyPassword=${{ secrets.KEY_PASSWORD }}
          EOF

      - name: Create Play Publisher key
        run: echo '${{ secrets.PLAY_PUBLISHER_KEY }}' > frontend/play-publisher-key.json

      - name: Install frontend dependencies
        working-directory: frontend
        run: npm ci

      - name: Build frontend
        working-directory: frontend
        run: npm run build

      - name: Sync Capacitor
        working-directory: frontend
        run: npx cap sync android

      - name: Build release AAB
        working-directory: frontend/android
        run: ./gradlew bundleRelease -PversionName=${{ steps.version.outputs.version_name }} -PversionCode=${{ steps.version.outputs.version_code }}

      - name: Upload AAB artifact
        uses: actions/upload-artifact@v4
        with:
          name: release-aab
          path: frontend/android/app/build/outputs/bundle/release/*.aab
          retention-days: 90

      - name: Publish to internal track
        working-directory: frontend/android
        run: ./gradlew publishBundle -PversionName=${{ steps.version.outputs.version_name }} -PversionCode=${{ steps.version.outputs.version_code }}
```
