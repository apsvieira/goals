# Privacy Policy for tiny tracker

**Last updated:** 2026-05-23 (revised)

## Overview

tiny tracker ("the App") is a personal goal tracking application. This policy describes how we collect, use, and protect your data.

## Data We Collect

### Account Information

When you sign in with Google, we receive your email address and display name. This is used solely for authentication and identifying your account.

### Goal Data

The App stores the goals you create and your daily completion records. This data is stored locally on your device and synced to our servers when you are signed in.

### Push Notification Tokens

If you enable push notifications, we store a device token provided by Firebase Cloud Messaging (FCM) to deliver notifications.

### Debug Reports (optional)

If you shake your device to report a problem, the App sends us: your user ID, app version, device model, recent technical logs (with goal names replaced by internal identifiers), and your description of the problem. Debug reports are kept for 90 days and then automatically deleted.

### Server-Side Request Logs

When you are signed in and use the App, our backend records technical metadata about each API request — HTTP method, URL path, response status, request duration, an internal request ID, and your user ID — and ships these log lines to PostHog Logs (EU region) so we can diagnose problems and reproduce reported bugs. These logs do not contain the contents of your goals or completions.

### Product Analytics Events

When you are signed in, the App sends a small set of product events to PostHog (EU region) so we can understand which features are used. The events are: goal created, goal completed, goal deleted (archived), sync completed, sync failed, calendar view switched, notification permission changed, debug report submitted, shake-to-report triggered, and page view. Each event is associated with your user ID and contains only event-shape metadata (e.g. a goal ID, a sync duration). Goal names, descriptions, and completion data are never included.

### IP Address and Approximate Location

When the App communicates with our analytics and error-reporting providers, the connection's IP address is transmitted as part of normal internet traffic. PostHog uses this IP to derive your approximate location (country, region, and city) and attaches it to your analytics profile. Sentry stores the IP alongside error reports. We do not use IP or approximate location for advertising or for any purpose other than understanding our user base and diagnosing issues.

## How We Use Your Data

- **Authentication:** Your Google account info identifies you and secures your data.
- **Sync:** Goal and completion data is synced between your devices via our servers.
- **Notifications:** Push tokens are used only to send notifications you have opted into.

We do not sell, share, or use your data for advertising.

## Data Storage

- **On device:** Data is stored in your browser's IndexedDB or app storage on Android.
- **On our servers:** Synced data is stored in a PostgreSQL database hosted on Fly.io.
- **In transit:** All data is transmitted over HTTPS.

## Third-Party Services

- **Google OAuth 2.0:** For sign-in. Subject to [Google's Privacy Policy](https://policies.google.com/privacy).
- **Firebase Cloud Messaging:** For push notifications. Subject to [Google's Privacy Policy](https://policies.google.com/privacy).
- **Fly.io:** Server hosting. Subject to [Fly.io's Privacy Policy](https://fly.io/legal/privacy-policy/).
- **Sentry (sentry.io):** For automated error reporting. When the App encounters an unexpected error, a technical report is sent to Sentry containing: your user ID (a random identifier, not your email), your app version, device model, operating system, the connection's IP address, and a stack trace of the error. Your goal names, completions, and any personal content are not sent to Sentry. Subject to [Sentry's Privacy Policy](https://sentry.io/privacy/).
- **PostHog (posthog.com, EU region):** For product analytics and server-side log diagnostics. We send your user ID (a random identifier, not your email), the curated product events listed above, basic device fields (device type, OS version, app version), and backend request log lines (HTTP method, path, status, duration, request ID, user ID). PostHog additionally derives an approximate location (country, region, city) from the connection's IP address. Your goal names, goal content, and completion data are never sent to PostHog. Data is processed in the EU. Subject to [PostHog's Privacy Policy](https://posthog.com/privacy).

## Data Deletion

You can delete your account and all associated data by contacting us at the email below. Upon request, all server-side data is permanently removed within 30 days.

## Children's Privacy

The App is not directed at children under 13. We do not knowingly collect data from children.

## Contact

For privacy questions or data deletion requests, contact: aps.vieira95@gmail.com

## Changes

We may update this policy. Changes will be posted here with an updated date.
