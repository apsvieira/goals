# Play Store Release Preparation

> **Status:** OUTDATED

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Prepare tiny tracker for its first Google Play Store release, including build pipeline, store listing automation, and all required compliance artifacts.

**Architecture:** We'll fix the Android build config (signing, minification, versioning), set up the Gradle Play Publisher plugin for automated uploads, create a privacy policy, and prepare store listing metadata. The first app creation and some declarations must be done manually in Play Console, but subsequent releases will be fully automated via Gradle tasks.

**Tech Stack:** Capacitor 8, Gradle 8.13, Gradle Play Publisher plugin, Google Play Developer API (service account auth)

---

## Phase 1: Play Console Setup (Manual — User)

These steps CANNOT be automated and must be done by the user in the Play Console web UI before any API-based tasks work.

### Task 1: Create the app in Google Play Console

This is a one-time manual step. The API cannot create a new app.

**Step 1: Go to Play Console and create app**

1. Open https://play.google.com/console
2. Click "Create app"
3. Fill in:
   - **App name:** `tiny tracker`
   - **Default language:** English (United States)
   - **App or Game:** App
   - **Free or Paid:** Free
4. Accept the declarations and click "Create app"

**Step 2: Note the app's package name**

Confirm it matches `com.tinytracker.app`. If creating via Console, this is set when you upload your first AAB — the package name in the AAB's manifest becomes the app's identity.

---

### Task 2: Set up Google Cloud service account for API access

**Step 1: Create a Google Cloud project (if you don't have one)**

1. Go to https://console.cloud.google.com
2. Create a new project (e.g., "tiny-tracker-play") or use an existing one

**Step 2: Enable the Google Play Android Developer API**

1. In Cloud Console, go to APIs & Services > Library
2. Search for "Google Play Android Developer API"
3. Click Enable

**Step 3: Create a service account**

1. Go to IAM & Admin > Service Accounts
2. Click "Create Service Account"
3. Name: `play-publisher` (or similar)
4. Skip optional permissions steps
5. Click Done

**Step 4: Generate JSON key**

1. Click on the new service account
2. Go to Keys tab > Add Key > Create new key > JSON
3. Save the downloaded file as `play-publisher-key.json` in the project root
4. **IMPORTANT:** This file contains secrets. It must NEVER be committed to git.

**Step 5: Link service account in Play Console**

1. In Play Console, go to Setup > API access
2. Link the Google Cloud project you created
3. Find the service account and click "Manage Play Console permissions"
4. Grant at minimum:
   - **View app information and download bulk reports** (read)
   - **Release to production, exclude devices, and use Play App Signing** (release)
   - **Manage store presence** (listings)
5. Click "Invite user" and confirm
6. **Wait up to 24 hours** for activation (often faster)

**Step 6: Tell Claude the key is ready**

Once `play-publisher-key.json` is saved in the project root, proceed to Phase 2.

---

### Task 3: Enroll in Play App Signing

**Step 1: In Play Console, go to Release > App signing**

Google Play App Signing is required for new apps. When you upload your first AAB:
- Google generates and manages the **app signing key** (the key end-users see)
- Your **upload key** (the `tiny-tracker-release.keystore`) is used to sign what you upload to Google
- Google re-signs the final APK delivered to users

This is automatic for new apps — just be aware that Google holds the real signing key.

---

## Phase 2: Fix Build Configuration

### Task 4: Add release signing config to build.gradle

**Files:**
- Modify: `frontend/android/app/build.gradle`

**Step 1: Write a test — verify release build currently fails**

Run:
```bash
cd frontend && npx cap sync && cd android && ./gradlew assembleRelease 2>&1 | tail -20
```
Expected: Build succeeds but produces an unsigned APK (or signs with debug key). We want a properly signed release.

**Step 2: Add signingConfigs block to build.gradle**

In `frontend/android/app/build.gradle`, add the signing configuration. The keystore password and key alias will be read from environment variables or `keystore.properties` to avoid hardcoding secrets.

```groovy
apply plugin: 'com.android.application'

def keystorePropertiesFile = rootProject.file("keystore.properties")
def keystoreProperties = new Properties()
if (keystorePropertiesFile.exists()) {
    keystoreProperties.load(new FileInputStream(keystorePropertiesFile))
}

android {
    namespace = "com.tinytracker.app"
    compileSdk = rootProject.ext.compileSdkVersion
    defaultConfig {
        applicationId "com.tinytracker.app"
        minSdkVersion rootProject.ext.minSdkVersion
        targetSdkVersion rootProject.ext.targetSdkVersion
        versionCode 1
        versionName "1.0"
        testInstrumentationRunner "androidx.test.runner.AndroidJUnitRunner"
        aaptOptions {
            ignoreAssetsPattern = '!.svn:!.git:!.ds_store:!*.scc:.*:!CVS:!thumbs.db:!picasa.ini:!*~'
        }
    }
    signingConfigs {
        release {
            storeFile file(keystoreProperties['storeFile'] ?: '../../tiny-tracker-release.keystore')
            storePassword keystoreProperties['storePassword'] ?: System.getenv("KEYSTORE_PASSWORD")
            keyAlias keystoreProperties['keyAlias'] ?: System.getenv("KEY_ALIAS")
            keyPassword keystoreProperties['keyPassword'] ?: System.getenv("KEY_PASSWORD")
        }
    }
    buildTypes {
        release {
            signingConfig signingConfigs.release
            minifyEnabled true
            shrinkResources true
            proguardFiles getDefaultProguardFile('proguard-android-optimize.txt'), 'proguard-rules.pro'
        }
    }
}
```

**Step 3: Create keystore.properties (local, gitignored)**

Create `frontend/android/keystore.properties`:
```properties
storeFile=../../tiny-tracker-release.keystore
storePassword=<your-keystore-password>
keyAlias=<your-key-alias>
keyPassword=<your-key-password>
```

**Step 4: Add keystore.properties to .gitignore**

Append to root `.gitignore`:
```
keystore.properties
play-publisher-key.json
```

**Step 5: Run release build to verify signing works**

Run:
```bash
cd frontend/android && ./gradlew assembleRelease 2>&1 | tail -20
```
Expected: `BUILD SUCCESSFUL` with a signed APK at `app/build/outputs/apk/release/app-release.apk`

**Step 6: Commit**

```bash
git add frontend/android/app/build.gradle .gitignore
git commit -m "feat(android): add release signing config with keystore.properties"
```

---

### Task 5: Add ProGuard rules for Capacitor WebView app

**Files:**
- Modify: `frontend/android/app/proguard-rules.pro`

**Step 1: Add rules to preserve WebView JavaScript interface**

Since this is a Capacitor app, the WebView bridge must not be obfuscated.

```proguard
# Capacitor
-keep class com.getcapacitor.** { *; }
-dontwarn com.getcapacitor.**

# Keep JavaScript interface methods
-keepclassmembers class * {
    @android.webkit.JavascriptInterface <methods>;
}

# AndroidX
-keep class androidx.** { *; }
-dontwarn androidx.**
```

**Step 2: Build release to verify ProGuard doesn't break anything**

Run:
```bash
cd frontend/android && ./gradlew assembleRelease 2>&1 | tail -20
```
Expected: `BUILD SUCCESSFUL`

**Step 3: Commit**

```bash
git add frontend/android/app/proguard-rules.pro
git commit -m "feat(android): add ProGuard rules for Capacitor WebView bridge"
```

---

### Task 6: Fix deep link scheme mismatch

**Files:**
- Modify: `frontend/android/app/src/main/AndroidManifest.xml`

The AndroidManifest uses `goaltracker://` but `capacitor.config.ts` uses `tinytracker://`. Align to `tinytracker://`.

**Step 1: Update AndroidManifest.xml**

Change:
```xml
<data android:scheme="goaltracker" />
```
To:
```xml
<data android:scheme="tinytracker" />
```

**Step 2: Verify the app builds**

Run:
```bash
cd frontend && npx cap sync && cd android && ./gradlew assembleDebug 2>&1 | tail -5
```
Expected: `BUILD SUCCESSFUL`

**Step 3: Commit**

```bash
git add frontend/android/app/src/main/AndroidManifest.xml
git commit -m "fix(android): align deep link scheme to tinytracker:// matching capacitor config"
```

---

### Task 7: Build the AAB (Android App Bundle)

**Step 1: Build the frontend**

```bash
cd frontend && npm run build
```
Expected: Vite outputs to `frontend/dist/`

**Step 2: Sync with Capacitor**

```bash
cd frontend && npx cap sync android
```
Expected: Web assets copied to Android project

**Step 3: Build the release AAB**

```bash
cd frontend/android && ./gradlew bundleRelease 2>&1 | tail -10
```
Expected: `BUILD SUCCESSFUL` with AAB at `app/build/outputs/bundle/release/app-release.aab`

**Step 4: Verify the AAB**

```bash
ls -lh frontend/android/app/build/outputs/bundle/release/app-release.aab
```
Expected: File exists, reasonable size (likely 5-15 MB for a WebView app)

---

## Phase 3: Automate Play Store Uploads

### Task 8: Add Gradle Play Publisher plugin

**Files:**
- Modify: `frontend/android/build.gradle` (root)
- Modify: `frontend/android/app/build.gradle` (app)

**Step 1: Add the plugin to root build.gradle**

In `frontend/android/build.gradle`, add to the `dependencies` block inside `buildscript`:
```groovy
classpath 'com.github.triplet.gradle:play-publisher:4.0.0-SNAPSHOT'
```

Actually — GPP v3.x is the latest stable. Add:
```groovy
buildscript {
    repositories {
        // ... existing repos
        maven { url 'https://plugins.gradle.org/m2/' }
    }
    dependencies {
        // ... existing deps
        classpath 'com.github.triplet.gradle:play-publisher:3.11.0'
    }
}
```

**Step 2: Apply the plugin in app/build.gradle**

Add after the first line:
```groovy
apply plugin: 'com.github.triplet.play'
```

Add the play block:
```groovy
play {
    serviceAccountCredentials.set(file("../../play-publisher-key.json"))
    track.set("internal")  // Start with internal testing track
    defaultToAppBundles.set(true)
}
```

**Step 3: Verify the plugin loads**

```bash
cd frontend/android && ./gradlew tasks --group publishing 2>&1 | head -20
```
Expected: Shows `publishBundle`, `publishListing`, `promoteRelease` tasks (only works once service account key is in place)

**Step 4: Commit**

```bash
git add frontend/android/build.gradle frontend/android/app/build.gradle
git commit -m "feat(android): add Gradle Play Publisher for automated store uploads"
```

---

## Phase 4: Store Listing & Compliance

### Task 9: Create privacy policy

**Files:**
- Create: `docs/privacy-policy.md`

**Step 1: Write the privacy policy**

The app collects:
- Google account info (email, name) via OAuth for authentication
- Goal tracking data (goal names, completion dates)
- Device info for push notifications (FCM token)

The privacy policy needs to cover:
- What data is collected
- How it's used (sync, authentication)
- Data storage (server-side PostgreSQL, on-device IndexedDB)
- Third-party services (Google OAuth, FCM)
- Data deletion rights
- Contact information

```markdown
# Privacy Policy for tiny tracker

**Last updated:** 2026-03-23

## Overview

tiny tracker ("the App") is a personal goal tracking application. This policy describes how we collect, use, and protect your data.

## Data We Collect

### Account Information
When you sign in with Google, we receive your email address and display name. This is used solely for authentication and identifying your account.

### Goal Data
The App stores the goals you create and your daily completion records. This data is stored locally on your device and synced to our servers when you are signed in.

### Push Notification Tokens
If you enable push notifications, we store a device token provided by Firebase Cloud Messaging (FCM) to deliver notifications.

## How We Use Your Data

- **Authentication:** Your Google account info identifies you and secures your data.
- **Sync:** Goal and completion data is synced between your devices via our servers.
- **Notifications:** Push tokens are used only to send notifications you have opted into.

We do not sell, share, or use your data for advertising.

## Data Storage

- **On device:** Data is stored in your browser's IndexedDB (or app storage on Android).
- **On our servers:** Synced data is stored in a PostgreSQL database hosted on Fly.io.
- **In transit:** All data is transmitted over HTTPS.

## Third-Party Services

- **Google OAuth 2.0:** For sign-in. Subject to [Google's Privacy Policy](https://policies.google.com/privacy).
- **Firebase Cloud Messaging:** For push notifications. Subject to [Google's Privacy Policy](https://policies.google.com/privacy).
- **Fly.io:** Server hosting. Subject to [Fly.io's Privacy Policy](https://fly.io/legal/privacy-policy/).

## Data Deletion

You can delete your account and all associated data by contacting us. Upon deletion, all server-side data is permanently removed.

## Contact

For privacy questions or data deletion requests, contact: [YOUR_EMAIL]

## Changes

We may update this policy. Changes will be posted here with an updated date.
```

**Step 2: Host the privacy policy**

You need a publicly accessible URL. Options:
- Host it on your app's domain (e.g., `https://your-domain.fly.dev/privacy`)
- Use a GitHub Pages URL from the repo
- Any static URL that doesn't require login

**Step 3: Commit**

```bash
git add docs/privacy-policy.md
git commit -m "docs: add privacy policy for Play Store listing"
```

---

### Task 10: Prepare store listing metadata

**Files:**
- Create: `frontend/android/src/main/play/default-language` as `en-US`
- Create: `frontend/android/src/main/play/listings/en-US/full-description.txt`
- Create: `frontend/android/src/main/play/listings/en-US/short-description.txt`
- Create: `frontend/android/src/main/play/listings/en-US/title.txt`

Gradle Play Publisher reads listing metadata from `src/main/play/` directory. This lets us version-control store listing text.

**Step 1: Create listing files**

`title.txt` (max 30 chars):
```
tiny tracker
```

`short-description.txt` (max 80 chars):
```
Track daily goals with a beautiful hexagon calendar. Simple, offline-first.
```

`full-description.txt` (max 4000 chars):
```
tiny tracker helps you build consistent habits by tracking daily goal completion with a unique hexagon calendar view.

Features:
• Create personal goals and track daily completion
• Beautiful hexagon calendar shows your progress at a glance
• Works completely offline — your data is always available
• Syncs across devices when you sign in with Google
• Clean, minimal interface that stays out of your way
• Free, no ads, no tracking

How it works:
1. Add your goals (exercise, reading, meditation, etc.)
2. Tap to mark each day as complete
3. Watch your hexagon calendar fill up over time

Your data is stored locally on your device and optionally synced to our servers when signed in. We never sell or share your data.
```

**Step 2: Commit**

```bash
git add frontend/android/src/main/play/
git commit -m "feat(android): add Play Store listing metadata"
```

---

### Task 11: Prepare store listing graphics

The Play Store requires these graphics. They CANNOT be automated — you'll need to create them.

**Required:**
| Asset | Dimensions | Format | Notes |
|-------|-----------|--------|-------|
| App icon | 512x512 | PNG, 32-bit, no alpha | High-res version of app icon |
| Feature graphic | 1024x500 | PNG or JPEG | Banner shown at top of listing |
| Phone screenshots | 320-3840px wide, 16:9 or 9:16 | PNG or JPEG | Min 2, max 8. Show the app in use |

**Recommended approach:**
1. Run the app on an emulator or device
2. Take screenshots of: the goal list, the hexagon calendar, adding a goal
3. Optionally frame them with a tool like https://screenshots.pro or Figma
4. Save to `frontend/android/src/main/play/listings/en-US/graphics/`:
   - `icon/icon.png` (512x512)
   - `featureGraphic/feature-graphic.png` (1024x500)
   - `phoneScreenshots/1.png`, `2.png`, etc.

---

## Phase 5: Play Console Manual Declarations (User)

These must be done in the Play Console web UI. No API exists.

### Task 12: Complete Play Console declarations

**Step 1: Content rating (IARC questionnaire)**

1. Play Console > App content > Content rating
2. Start questionnaire
3. For tiny tracker, likely answers:
   - No violence, no sexual content, no gambling, no drugs
   - No user-generated content shared between users
   - No location data collection
4. Result: Likely rated "Everyone" / PEGI 3

**Step 2: Target audience**

1. Play Console > App content > Target audience
2. Target age group: 18+ (simplest — avoids COPPA/children's policy requirements)
3. Not a teacher-approved app

**Step 3: Data safety form**

1. Play Console > App content > Data safety
2. Declarations:
   - **Collects data:** Yes
   - **Email address:** Collected for account management, not shared
   - **App activity (goals/completions):** Collected for app functionality, not shared
   - **Device identifiers (FCM token):** Collected for push notifications, not shared
   - **Data encrypted in transit:** Yes
   - **Users can request data deletion:** Yes
   - **Link to privacy policy:** (your hosted URL)

**Step 4: Ads declaration**

1. Play Console > App content > Ads
2. "No, my app does not contain ads"

**Step 5: App category**

1. Play Console > Grow > Store presence > Main store listing
2. Category: Productivity (or Health & Fitness)

---

## Phase 6: First Release

### Task 13: Upload first AAB and create internal testing release

Once the service account is active (Task 2) and the app is created in Play Console (Task 1):

**Step 1: Build fresh AAB**

```bash
cd frontend && npm run build && npx cap sync android
cd android && ./gradlew bundleRelease
```

**Step 2: Upload to internal testing track**

If Gradle Play Publisher is set up (Task 8):
```bash
cd frontend/android && ./gradlew publishBundle
```

Or upload manually:
1. Play Console > Release > Testing > Internal testing
2. Click "Create new release"
3. Upload the AAB from `app/build/outputs/bundle/release/app-release.aab`
4. Add release notes
5. Review and start rollout

**Step 3: Add internal testers**

1. Play Console > Release > Testing > Internal testing > Testers
2. Create an email list with your email (and any testers)
3. Share the opt-in link

**Step 4: Test the internal release**

1. Open the opt-in link on an Android device
2. Install from Play Store
3. Verify: app opens, login works, goals can be created, sync works

---

### Task 14: Promote to production

Once internal testing is verified:

**Step 1: Ensure all declarations are complete**

Play Console will show a checklist. All items must be green:
- Content rating ✓
- Target audience ✓
- Data safety ✓
- Ads ✓
- Privacy policy ✓
- Store listing (text + graphics) ✓
- App signing ✓

**Step 2: Promote release**

Via Gradle Play Publisher:
```bash
cd frontend/android && ./gradlew promoteRelease --track internal --promote-track production
```

Or manually in Play Console:
1. Release > Production > Create new release
2. "Add from library" — select the tested AAB
3. Add release notes
4. Review and start rollout to production

**Step 3: Wait for review**

Google reviews typically take 1-3 days for new apps. Check status in Play Console — there's no API endpoint for detailed review status.

---

## Phase 7: CI/CD (Optional, post-first-release)

### Task 15: Update GitHub Actions for release builds

**Files:**
- Modify: `.github/workflows/android-build.yml`

Extend the existing workflow to also produce signed AABs and optionally publish to the internal track on tagged releases. This task can be done after the first manual release is successful.

**Step 1: Add secrets to GitHub repo**

In GitHub repo Settings > Secrets:
- `KEYSTORE_BASE64`: base64-encoded keystore file
- `KEYSTORE_PASSWORD`: keystore password
- `KEY_ALIAS`: key alias
- `KEY_PASSWORD`: key password
- `PLAY_PUBLISHER_KEY`: contents of `play-publisher-key.json`

**Step 2: Add release job to workflow**

Add a job triggered on version tags (`v*`) that:
1. Decodes the keystore from base64
2. Writes `keystore.properties`
3. Writes `play-publisher-key.json`
4. Runs `./gradlew publishBundle`

This is an optional optimization — manual `./gradlew publishBundle` from your machine works fine for a personal app.

---

## Summary: What you do vs. what I do

| Task | Who | Automated? |
|------|-----|-----------|
| Create app in Play Console | You | No — one-time manual |
| Create service account + JSON key | You | No — one-time manual |
| Enroll in Play App Signing | You | No — automatic for new apps |
| Fix build.gradle signing config | Claude | Yes |
| Add ProGuard rules | Claude | Yes |
| Fix deep link mismatch | Claude | Yes |
| Build AAB | Claude | Yes |
| Set up Gradle Play Publisher | Claude | Yes |
| Write privacy policy | Claude | Yes (hosting is on you) |
| Write store listing text | Claude | Yes |
| Create screenshots/graphics | You | No — visual assets |
| Content rating questionnaire | You | No — Play Console only |
| Target audience declaration | You | No — Play Console only |
| Data safety form | You | No — Play Console only |
| Upload first AAB | Claude (or you) | Yes, via Gradle task |
| Promote to production | You (confirm) | Semi — Gradle task with your approval |
