#!/usr/bin/env bash
# Setup Firebase for the existing GCP project.
# Prerequisites: gcloud CLI authenticated with project owner permissions.
#
# Usage: bash scripts/setup-firebase.sh
#
# What this script does:
#   1. Enables the Firebase Management API on the GCP project
#   2. Adds Firebase to the existing GCP project
#   3. Creates a Firebase Android app for the package name
#   4. Downloads google-services.json into frontend/android/app/
#
# After running:
#   - Commit google-services.json locally (it's gitignored — keep it that way)
#   - Base64-encode it and add as GOOGLE_SERVICES_JSON secret in GitHub Actions
#     base64 -w0 frontend/android/app/google-services.json | gh secret set GOOGLE_SERVICES_JSON

set -euo pipefail

PROJECT_ID="tiny-tracker-f97da"
PACKAGE_NAME="software.maleficent.tinytracker"
APP_DISPLAY_NAME="tiny tracker"
OUTPUT_PATH="frontend/android/app/google-services.json"

REPO_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$REPO_ROOT"

echo "=== Firebase Setup for $PROJECT_ID ==="
echo ""

# ─── Step 1: Enable Firebase Management API ─────────────────────────
echo "[1/4] Enabling Firebase Management API..."
gcloud services enable firebase.googleapis.com --project="$PROJECT_ID" --quiet
echo "  ✓ firebase.googleapis.com enabled"

# ─── Step 2: Add Firebase to the GCP project ────────────────────────
echo ""
echo "[2/4] Checking Firebase project status..."

ACCESS_TOKEN=$(gcloud auth print-access-token)
FIREBASE_PROJECT=$(curl -s \
    "https://firebase.googleapis.com/v1beta1/projects/${PROJECT_ID}" \
    -H "Authorization: Bearer $ACCESS_TOKEN" \
    -H "X-Goog-User-Project: $PROJECT_ID")

if echo "$FIREBASE_PROJECT" | python3 -c "import sys,json; d=json.load(sys.stdin); assert 'projectId' in d" 2>/dev/null; then
    echo "  ✓ Firebase already enabled on project"
else
    # Try to add Firebase via API (requires ToS acceptance)
    echo "  Adding Firebase..."
    RESPONSE=$(curl -s -X POST \
        "https://firebase.googleapis.com/v1beta1/projects/${PROJECT_ID}:addFirebase" \
        -H "Authorization: Bearer $ACCESS_TOKEN" \
        -H "X-Goog-User-Project: $PROJECT_ID" \
        -H "Content-Type: application/json")

    OP_NAME=$(echo "$RESPONSE" | python3 -c "import sys,json; print(json.load(sys.stdin).get('name',''))" 2>/dev/null || true)

    if [ -n "$OP_NAME" ]; then
        echo "  Waiting for Firebase provisioning..."
        for i in $(seq 1 30); do
            sleep 2
            OP_STATUS=$(curl -s \
                "https://firebase.googleapis.com/v1beta1/$OP_NAME" \
                -H "Authorization: Bearer $ACCESS_TOKEN" \
                -H "X-Goog-User-Project: $PROJECT_ID")
            DONE=$(echo "$OP_STATUS" | python3 -c "import sys,json; print(json.load(sys.stdin).get('done', False))" 2>/dev/null || echo "False")
            if [ "$DONE" = "True" ]; then
                echo "  ✓ Firebase enabled on project"
                break
            fi
            if [ "$i" -eq 30 ]; then
                echo "  ⚠ Timed out. Check: https://console.firebase.google.com/project/$PROJECT_ID"
            fi
        done
    else
        ERROR=$(echo "$RESPONSE" | python3 -c "import sys,json; print(json.load(sys.stdin).get('error',{}).get('message',''))" 2>/dev/null || true)
        if echo "$ERROR" | grep -qi "permission"; then
            echo ""
            echo "  ✗ Firebase requires Terms of Service acceptance (one-time manual step)."
            echo "    Go to: https://console.firebase.google.com/"
            echo "    Click 'Add project' → choose '$PROJECT_ID' → accept ToS → continue."
            echo "    Then re-run this script."
            exit 1
        elif [ -n "$ERROR" ]; then
            echo "  ✗ Error: $ERROR"
            exit 1
        fi
    fi
fi

# ─── Step 3: Create Android app ─────────────────────────────────────
echo ""
echo "[3/4] Creating Firebase Android app ($PACKAGE_NAME)..."

ACCESS_TOKEN=$(gcloud auth print-access-token)

# Check if Android app already exists
EXISTING_APPS=$(curl -s \
    "https://firebase.googleapis.com/v1beta1/projects/${PROJECT_ID}/androidApps" \
    -H "Authorization: Bearer $ACCESS_TOKEN" \
    -H "X-Goog-User-Project: $PROJECT_ID")

EXISTING_APP_ID=$(echo "$EXISTING_APPS" | python3 -c "
import sys, json
data = json.load(sys.stdin)
for app in data.get('apps', []):
    if app.get('packageName') == '$PACKAGE_NAME':
        print(app['name'].split('/')[-1])
        break
" 2>/dev/null || true)

if [ -n "$EXISTING_APP_ID" ]; then
    echo "  ✓ Android app already exists (ID: $EXISTING_APP_ID)"
    APP_NAME="projects/$PROJECT_ID/androidApps/$EXISTING_APP_ID"
else
    RESPONSE=$(curl -s -X POST \
        "https://firebase.googleapis.com/v1beta1/projects/${PROJECT_ID}/androidApps" \
        -H "Authorization: Bearer $ACCESS_TOKEN" \
        -H "X-Goog-User-Project: $PROJECT_ID" \
        -H "Content-Type: application/json" \
        -d "{\"packageName\": \"$PACKAGE_NAME\", \"displayName\": \"$APP_DISPLAY_NAME\"}")

    OP_NAME=$(echo "$RESPONSE" | python3 -c "import sys,json; print(json.load(sys.stdin).get('name',''))" 2>/dev/null || true)

    if [ -n "$OP_NAME" ]; then
        echo "  Waiting for app creation..."
        for i in $(seq 1 20); do
            sleep 2
            OP_STATUS=$(curl -s \
                "https://firebase.googleapis.com/v1beta1/$OP_NAME" \
                -H "Authorization: Bearer $ACCESS_TOKEN" \
                -H "X-Goog-User-Project: $PROJECT_ID")
            DONE=$(echo "$OP_STATUS" | python3 -c "import sys,json; print(json.load(sys.stdin).get('done', False))" 2>/dev/null || echo "False")
            if [ "$DONE" = "True" ]; then
                APP_NAME=$(echo "$OP_STATUS" | python3 -c "import sys,json; print(json.load(sys.stdin).get('response',{}).get('name',''))" 2>/dev/null || true)
                EXISTING_APP_ID=$(echo "$APP_NAME" | awk -F/ '{print $NF}')
                echo "  ✓ Android app created (ID: $EXISTING_APP_ID)"
                break
            fi
            if [ "$i" -eq 20 ]; then
                echo "  ✗ Timed out waiting for app creation"
                exit 1
            fi
        done
    else
        ERROR=$(echo "$RESPONSE" | python3 -c "import sys,json; print(json.load(sys.stdin).get('error',{}).get('message',''))" 2>/dev/null || true)
        echo "  ✗ Error creating app: $ERROR"
        exit 1
    fi
fi

# ─── Step 4: Download google-services.json ───────────────────────────
echo ""
echo "[4/4] Downloading google-services.json..."

# Need the full app name path
if [ -z "${APP_NAME:-}" ]; then
    APP_NAME="projects/$PROJECT_ID/androidApps/$EXISTING_APP_ID"
fi

ACCESS_TOKEN=$(gcloud auth print-access-token)
CONFIG_RESPONSE=$(curl -s \
    "https://firebase.googleapis.com/v1beta1/${APP_NAME}/config" \
    -H "Authorization: Bearer $ACCESS_TOKEN" \
    -H "X-Goog-User-Project: $PROJECT_ID")

# The config response has configFileContents as base64
CONFIG_B64=$(echo "$CONFIG_RESPONSE" | python3 -c "import sys,json; print(json.load(sys.stdin).get('configFileContents',''))" 2>/dev/null || true)

if [ -z "$CONFIG_B64" ]; then
    echo "  ✗ Failed to get config. Response:"
    echo "$CONFIG_RESPONSE" | python3 -m json.tool 2>/dev/null || echo "$CONFIG_RESPONSE"
    exit 1
fi

echo "$CONFIG_B64" | base64 -d > "$OUTPUT_PATH"
echo "  ✓ Saved to $OUTPUT_PATH"

echo ""
echo "=== Setup Complete ==="
echo ""
echo "Next steps:"
echo "  1. The file is gitignored — do NOT commit it to the repo"
echo "  2. Add it as a GitHub Actions secret:"
echo "     base64 -w0 $OUTPUT_PATH | gh secret set GOOGLE_SERVICES_JSON"
echo "  3. For local builds, the file is already in place"
