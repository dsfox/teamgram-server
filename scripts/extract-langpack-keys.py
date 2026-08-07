"""Extracts the interface keys that need translating from the client.

The English strings live in the client, other languages come from the server.
There is no point translating all twelve thousand keys at once: untranslated ones
fall back to English, so we start with what is visible on every screen.

Usage: python3 server/scripts/extract-langpack-keys.py <Localizable.strings> [prefix ...]
"""
import json
import re
import sys
from pathlib import Path

# A key and value in .strings format: "Key" = "Value";
LINE = re.compile(r'^"([^"]+)"\s*=\s*"((?:[^"\\]|\\.)*)";\s*$')

# The sections a user sees first: sign-in, the chat list, a conversation,
# settings and the common buttons.
DEFAULT_PREFIXES = (
    "Login.", "Localization.", "Common.", "Conversation.", "Chat.", "ChatList.",
    "Settings.", "Notification.", "Contacts.", "Group.", "Profile.", "UserInfo.",
    "Message.", "Media.", "Attachment.", "Compose.", "Weekday.", "Month.",
    "Time.", "Presence.", "Privacy.", "Alert.", "Share.", "Call.", "AccessDenied.",
    "AttachmentMenu.", "PeerInfo.", "DialogList.", "Appearance.", "Passcode.",
    "AuthSessions.", "Web.", "EmptyChat.", "Bot.", "Undo.", "Camera.", "Paint.",
)


def main(path: Path, prefixes):
    strings, plurals = {}, {}

    for line in path.read_text(encoding="utf-8").splitlines():
        match = LINE.match(line.strip())
        if not match:
            continue
        key, value = match.group(1), match.group(2)
        if not key.startswith(tuple(prefixes)):
            continue

        # Plural forms carry a suffix: Key_one, Key_many and so on.
        parts = key.rsplit("_", 1)
        if len(parts) == 2 and parts[1] in ("zero", "one", "two", "few", "many", "other", "any"):
            base, form = parts
            plurals.setdefault(base, {})[form] = value
        else:
            strings[key] = value

    print(json.dumps(
        {"strings": strings, "plurals": plurals},
        ensure_ascii=False, indent=1, sort_keys=True,
    ))


if __name__ == "__main__":
    if len(sys.argv) < 2:
        raise SystemExit(__doc__)
    main(Path(sys.argv[1]), sys.argv[2:] or DEFAULT_PREFIXES)
