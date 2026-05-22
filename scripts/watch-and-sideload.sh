#!/usr/bin/env bash
set -euo pipefail

PACKAGE="software.maleficent.tinytracker"
WORKFLOW="android-build.yml"
ARTIFACT="debug-apk"

cd "$(git rev-parse --show-toplevel)"

BRANCH=$(git rev-parse --abbrev-ref HEAD)

command -v gh >/dev/null || { echo "gh CLI not found" >&2; exit 1; }

if command -v adb >/dev/null 2>&1; then
    ADB=adb
elif command -v adb.exe >/dev/null 2>&1; then
    ADB=adb.exe
else
    echo "adb not found (neither 'adb' nor 'adb.exe' on PATH)" >&2
    exit 1
fi

STATE=$("$ADB" get-state 2>&1 | tr -d '\r' || true)
case "$STATE" in
    device)
        ;;
    unauthorized*|*unauthorized*)
        echo "Device is unauthorized — accept the USB debugging prompt on the phone, then re-run." >&2
        exit 1
        ;;
    *)
        echo "No adb device connected (run '$ADB devices'): $STATE" >&2
        exit 1
        ;;
esac

echo "Looking up latest '$WORKFLOW' run on branch '$BRANCH'..."
RUN_JSON=$(gh run list --workflow="$WORKFLOW" --branch="$BRANCH" --limit=1 \
    --json databaseId,status,conclusion,headSha,displayTitle,url)

if [ "$(echo "$RUN_JSON" | jq 'length')" -eq 0 ]; then
    echo "No runs found for '$WORKFLOW' on branch '$BRANCH'" >&2
    exit 1
fi

RUN_ID=$(echo "$RUN_JSON" | jq -r '.[0].databaseId')
STATUS=$(echo "$RUN_JSON" | jq -r '.[0].status')
CONCLUSION=$(echo "$RUN_JSON" | jq -r '.[0].conclusion')
TITLE=$(echo "$RUN_JSON" | jq -r '.[0].displayTitle')
URL=$(echo "$RUN_JSON" | jq -r '.[0].url')
SHA=$(echo "$RUN_JSON" | jq -r '.[0].headSha' | cut -c1-7)

echo "Run $RUN_ID ($SHA): $TITLE"
echo "  $URL"
echo "  status=$STATUS conclusion=$CONCLUSION"

if [ "$STATUS" != "completed" ]; then
    echo "Watching run to completion..."
    gh run watch "$RUN_ID" --exit-status
    CONCLUSION=$(gh run view "$RUN_ID" --json conclusion -q .conclusion)
fi

if [ "$CONCLUSION" != "success" ]; then
    echo "Run concluded with: $CONCLUSION" >&2
    exit 1
fi

TMPDIR=$(mktemp -d)
trap 'rm -rf "$TMPDIR"' EXIT

echo "Downloading artifact '$ARTIFACT'..."
gh run download "$RUN_ID" --name "$ARTIFACT" --dir "$TMPDIR"

APK=$(find "$TMPDIR" -name '*.apk' -type f | head -n1)
if [ -z "$APK" ]; then
    echo "No APK in artifact" >&2
    exit 1
fi
echo "APK: $(basename "$APK") ($(du -h "$APK" | cut -f1))"

echo "Uninstalling $PACKAGE (if present)..."
"$ADB" uninstall "$PACKAGE" >/dev/null 2>&1 || true

echo "Installing..."
"$ADB" install "$APK"

echo "Done."
