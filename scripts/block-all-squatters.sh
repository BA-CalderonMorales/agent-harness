#!/bin/bash
# block-all-squatters.sh - Block all identified email squatters
# Run after: gh auth refresh -h github.com -s user

SQUATTERS=(
    superman32432432
)

echo "========================================"
echo "Blocking All Identified Squatters"
echo "========================================"
echo ""

for user in "${SQUATTERS[@]}"; do
    echo "▶ Blocking $user..."
    if gh api -X PUT "/user/blocks/$user" >/dev/null 2>&1; then
        echo "  ✓ Blocked $user"
    else
        echo "  ✗ Failed to block $user (check auth scope)"
    fi
done

echo ""
echo "========================================"
echo "Done"
echo "========================================"
