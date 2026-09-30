#!/usr/bin/env bash
# bazel-farm.yml's authorize job: may this pull_request_target run hand a
# fork PR's head to bazel.yml with the RBE secrets? Runs from a checkout of
# the base branch (the run's own commit), never the PR; reads event facts
# from the environment only (the workflow never interpolates them into a
# script). Writes allowed=true|false to $GITHUB_OUTPUT and the reason to the
# log and step summary. Exits non-zero only on a broken invocation.
#
# Allowed only when all hold:
#   EVENT_NAME is pull_request_target and ACTION is opened, synchronize,
#     reopened or ready_for_review;
#   BASE_REPO is REPOSITORY and HEAD_REPO is another repository (a fork);
#   BASE_REF is DEFAULT_BRANCH;
#   HEAD_SHA is a full 40-hex commit id (what bazel.yml checks out);
#   PR_AUTHOR and SENDER are both on ALLOWLIST (case-insensitive). SENDER is
#     who triggered this event: the pusher on synchronize, so a collaborator
#     on a listed author's fork cannot push code that runs with secrets.
#
# Inputs (environment): ALLOWLIST (file path), EVENT_NAME, ACTION,
# REPOSITORY, BASE_REPO, HEAD_REPO, BASE_REF, DEFAULT_BRANCH, HEAD_SHA,
# PR_AUTHOR, SENDER, GITHUB_OUTPUT; GITHUB_STEP_SUMMARY optional.

set -euo pipefail

: "${GITHUB_OUTPUT:?GITHUB_OUTPUT is not set}"
allowlist="${ALLOWLIST:?ALLOWLIST is not set}"
if [[ ! -f "$allowlist" ]]; then
    echo "::error::allowlist $allowlist is missing" >&2
    exit 1
fi

decide() {
    allowed=false
    reason="$1"
}

# A login: GitHub's charset, lowercased. Anything else never matches.
login() {
    local v="${1:-}"
    if [[ "$v" =~ ^[A-Za-z0-9]([A-Za-z0-9-]{0,38})$ ]]; then
        printf '%s' "${v,,}"
    fi
}

listed() {
    local want line
    want="$(login "${1:-}")"
    [[ -n "$want" ]] || return 1
    while IFS= read -r line || [[ -n "$line" ]]; do
        line="${line%%#*}"
        line="${line//[[:space:]]/}"
        [[ -n "$line" ]] || continue
        if [[ "$(login "$line")" == "$want" ]]; then
            return 0
        fi
    done < "$allowlist"
    return 1
}

allowed=true
reason="author and sender are on the allowlist"
author="${PR_AUTHOR:-}"
sender="${SENDER:-}"
if [[ "${EVENT_NAME:-}" != pull_request_target ]]; then
    decide "event is not pull_request_target"
elif ! [[ "${ACTION:-}" =~ ^(opened|synchronize|reopened|ready_for_review)$ ]]; then
    decide "action is not opened, synchronize, reopened or ready_for_review"
elif [[ -z "${REPOSITORY:-}" || "${BASE_REPO:-}" != "$REPOSITORY" ]]; then
    decide "PR base repository is not this repository"
elif [[ -z "${HEAD_REPO:-}" || "${HEAD_REPO,,}" == "${REPOSITORY,,}" ]]; then
    decide "PR head is not a fork (same-repo PRs use pr.yml's remote run)"
elif [[ -z "${DEFAULT_BRANCH:-}" || "${BASE_REF:-}" != "$DEFAULT_BRANCH" ]]; then
    decide "PR does not target the default branch"
elif ! [[ "${HEAD_SHA:-}" =~ ^[0-9a-f]{40}$ ]]; then
    decide "head SHA is not a full commit id"
elif ! listed "$author"; then
    decide "PR author is not on the allowlist"
elif ! listed "$sender"; then
    decide "the user who triggered this run is not on the allowlist"
fi

# Logins are printed only after the charset check (log-injection safe).
shown_author="$(login "$author")"
shown_sender="$(login "$sender")"
echo "allowed=$allowed" >> "$GITHUB_OUTPUT"
msg="Bazel farm: allowed=$allowed ($reason; author ${shown_author:-<invalid>}, sender ${shown_sender:-<invalid>})"
echo "$msg"
if [[ -n "${GITHUB_STEP_SUMMARY:-}" ]]; then
    echo "$msg" >> "$GITHUB_STEP_SUMMARY"
fi
