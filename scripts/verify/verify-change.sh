#!/usr/bin/env bash
# Verification: Complete change verification
# Purpose: Run all verification steps before committing
# Usage: ./scripts/verify/verify-change.sh [--quick]
#
# This is the main entry point for the verification feedback loop.
# AI agents should run this after making changes to ensure quality.

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"

cd "$PROJECT_ROOT"

QUICK_MODE=false

# Parse arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        --quick)
            QUICK_MODE=true
            shift
            ;;
        *)
            shift
            ;;
    esac
done

echo "=============================================="
echo "  VERIFICATION FEEDBACK LOOP"
echo "  Agent Harness Change Verification"
echo "=============================================="
echo ""

START_TIME=$(date +%s)
FAILED=0

# Step 1: Build
echo "[STEP 1/4] Build Verification"
echo "------------------------------"
if go build ./cmd/agent-harness; then
    echo "[PASS] Build successful"
    rm -f agent-harness
    echo ""
else
    echo "[ABORT] Build failed - fix compilation errors first"
    exit 1
fi

# Step 2: Quality
echo "[STEP 2/4] Quality Verification"
echo "--------------------------------"
QUALITY_FAILED=0

# go vet
if go vet ./...; then
    echo "[PASS] go vet"
else
    echo "[FAIL] go vet found issues"
    QUALITY_FAILED=1
fi

# formatting
if [ "$(gofmt -l . | wc -l)" -eq 0 ]; then
    echo "[PASS] gofmt"
else
    echo "[FAIL] gofmt: the following files need formatting:"
    gofmt -l .
    QUALITY_FAILED=1
fi

if [ $QUALITY_FAILED -eq 1 ]; then
    echo "[ABORT] Quality check failed - fix issues above"
    exit 1
fi
echo ""

# Step 3: Tests (skip in quick mode)
if [ "$QUICK_MODE" = true ]; then
    echo "[STEP 3/4] Test Verification (SKIPPED - quick mode)"
    echo "----------------------------------------------------"
    echo "[SKIP] Use full mode for complete verification"
    echo ""
else
    echo "[STEP 3/4] Test Verification"
    echo "-----------------------------"
    if go test ./...; then
        echo "[PASS] All tests passed"
        echo ""
    else
        echo "[WARN] Some tests failed - review before committing"
        FAILED=1
    fi
    
    # Race detector (only in full mode, can be slow)
    echo "[INFO] Running race detector..."
    if go test -race ./... 2>/dev/null; then
        echo "[PASS] Race detector"
    else
        echo "[WARN] Race conditions detected or test failure"
        FAILED=1
    fi
fi

# Step 4: Build tags verification
echo "[STEP 4/4] Build Tags Verification"
echo "-----------------------------------"
TAGS_FAILED=0

# Test with kairos tag
if go build -tags kairos -o /dev/null ./cmd/agent-harness 2>/dev/null; then
    echo "[PASS] kairos build tag"
else
    echo "[FAIL] kairos build tag"
    TAGS_FAILED=1
fi

if [ $TAGS_FAILED -eq 1 ]; then
    echo "[WARN] Some build tags failed"
    FAILED=1
fi
echo ""

END_TIME=$(date +%s)
DURATION=$((END_TIME - START_TIME))

echo "=============================================="
if [ $FAILED -eq 0 ]; then
    echo "  VERIFICATION: ALL CHECKS PASSED"
    echo "  Duration: ${DURATION}s"
    echo "=============================================="
    echo ""
    echo "Ready to commit. Suggested workflow:"
    echo "  1. git add -A"
    echo "  2. git commit -m 'type(scope): description'"
    echo "  3. git push"
    echo ""
    exit 0
else
    echo "  VERIFICATION: SOME CHECKS FAILED"
    echo "  Duration: ${DURATION}s"
    echo "=============================================="
    echo ""
    echo "Review the failures above before committing."
    echo ""
    exit 1
fi
