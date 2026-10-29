"""Strip Emoji / pictographs from ingest and enrichment text."""

from __future__ import annotations

import re

_EMOJI_RE = re.compile(
    r"(?:[\U0001F1E0-\U0001F1FF]{2})|"
    r"[\U0001F600-\U0001F64F]|"
    r"[\U0001F300-\U0001F5FF]|"
    r"[\U0001F680-\U0001F6FF]|"
    r"[\U0001F700-\U0001F77F]|"
    r"[\U0001F780-\U0001F7FF]|"
    r"[\U0001F800-\U0001F8FF]|"
    r"[\U0001F900-\U0001F9FF]|"
    r"[\U0001FA00-\U0001FA6F]|"
    r"[\U0001FA70-\U0001FAFF]|"
    r"[\u2600-\u26FF]|"
    r"[\u2700-\u27BF]|"
    r"[\u2300-\u23FF]|"
    r"[\uFE00-\uFE0F]|"
    r"[\u200D]|"
    r"[\u20E3]|"
    r"[\U000E0020-\U000E007F]"
)


def strip_emojis(text: str) -> str:
    if not text:
        return text
    return _EMOJI_RE.sub("", text).replace("\u200b", "").strip()
