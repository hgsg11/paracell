#!/usr/bin/env python3

import json
import sys


def deny(reason: str) -> None:
    json.dump(
        {
            "hookSpecificOutput": {
                "hookEventName": "PreToolUse",
                "permissionDecision": "deny",
                "permissionDecisionReason": reason,
            }
        },
        sys.stdout,
    )


def main() -> int:
    payload = json.load(sys.stdin)
    if payload.get("tool_name") != "Bash":
        return 0

    tool_input = payload.get("tool_input") or {}
    command = tool_input.get("command", "")

    normalized = " ".join(command.split())
    if normalized.startswith("git merge"):
        deny("Repository policy blocks git merge. Create or update a PR instead.")
        return 0

    if normalized.startswith("git push origin main"):
        deny("Repository policy blocks pushing directly to origin main. Use a PR instead.")
        return 0

    if normalized.startswith("git push --set-upstream origin main"):
        deny("Repository policy blocks pushing main directly. Use a feature branch and PR instead.")
        return 0

    return 0


if __name__ == "__main__":
    raise SystemExit(main())
