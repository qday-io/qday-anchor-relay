#!/bin/bash
# Digest-poll updater for qday-anchor-relay + qday-anchor-portal
# (same pattern as update_explorer_frontend.sh). Cron it, e.g.:
#   */5 * * * * ~/deploy/qday-anchor-relay/scripts/update_anchor_relay.sh >> ~/deploy/qday-anchor-relay/update.log 2>&1

cd ~/deploy/qday-anchor-relay || exit

IMAGES=(
  "ghcr.io/qday-io/qday-anchor-relay:testnet"
  "ghcr.io/qday-io/qday-anchor-portal:testnet"
)

get_current_digest() {
    docker inspect --format='{{index .RepoDigests 0}}' "$1" 2>/dev/null | cut -d'@' -f2
}

get_latest_digest() {
    docker pull "$1" >/dev/null 2>&1
    docker inspect --format='{{index .RepoDigests 0}}' "$1" | cut -d'@' -f2
}

CHANGED=0
for IMAGE in "${IMAGES[@]}"; do
    CURRENT=$(get_current_digest "$IMAGE")
    LATEST=$(get_latest_digest "$IMAGE")
    if [ "$CURRENT" != "$LATEST" ]; then
        echo "$(date -u +%FT%TZ) new version of $IMAGE detected"
        CHANGED=1
    fi
done

if [ "$CHANGED" = "1" ]; then
    echo "$(date -u +%FT%TZ) updating stack..."
    # relay self-migrates at startup — nothing else to refresh.
    docker compose down
    docker compose up -d
    echo "$(date -u +%FT%TZ) update completed."
fi
