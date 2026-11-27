---
name: composer-lyrics
description: Work on the lyrics open in Composer through its `composer` MCP server - fix lyric text and language, write transliterations (romanization) and translations, and resolve the Languages tab's "lines need review" transliteration alignment warnings. Use when the user asks to fix lyrics, transliterate, romanize, translate, or align/fix transliteration boundaries in Composer, or when the composer MCP tools are missing or fail to connect.
---

# Composer lyrics

Composer runs in the user's browser. Composer Bridge relays MCP calls to the tab that has the project open, and every write is one undo step there. You can fix lyric text with `edit_lines`, but never timing: if timing looks wrong, tell the user.

## When the tools are missing or fail

Probe the endpoint (the port may differ, see `~/.composer-bridge/port.txt`):

```bash
curl -s -o /dev/null -w '%{http_code}\n' -X POST localhost:7777/mcp \
  -H 'Content-Type: application/json' -d '{"jsonrpc":"2.0","id":1,"method":"ping"}'
```

- Connection refused: Composer Bridge isn't running. Ask the user to start it.
- `404`: the running bridge predates MCP support. It needs a newer build; an older copy may still be holding the port.
- `200` but a tool says "Composer isn't connected": in Composer, Advanced settings, turn on "Let AI agents edit through Composer Bridge". The row should say Connected.
- Only `list_projects` is listed: Composer hasn't connected since the client started. Reconnect the server (`/mcp` in Claude Code).

## Workflow

1. `list_projects`. With more than one project open, pass `projectId` on every call.
2. `get_project` for the song language and issue counts, then `get_lines` with `withIssues: true` (follow `nextOffset`). If the language is missing or wrong for what is sung, fix it with `set_project_language` first: auto-align and default transliteration tags read it. It is not an undo step, so say what it was.
3. Wrong lyrics (typos, wrong kanji, syllable splits that don't match how it's sung): `edit_lines`, written like `get_lines` text, with spaces between words and `splitCharacter` (usually `|`) between syllables. Matching syllables keep their timing; new ones get timing Composer spreads, so mention that the user may want to re-sync them. Then redo the affected lines' transliteration and translations.
4. Missing or `transliteration-source-changed`: `set_transliterations`. Write the reading as sung, with two spaces between words of the original and one space inside a word: `Mizu tamari ni  ashi o  tsukkonde`. Send 20 to 50 lines per call.
5. `transliteration-alignment-needs-review`, `transliteration-does-not-fit-syllables` or `transliteration-syllables-unmapped`: `align_transliteration` with one slice per syllable, in order. The slices must spell the reading; spaces and dashes don't matter.
6. `translation-source-changed:<lang>` or a new language: `set_translations` with a BCP 47 tag. One natural translation per line; use `backgroundText` for background vocals.
7. Finish with `get_project`: `linesWithIssues` should be 0. Report what changed and that the user can undo each call in Composer.

## Slicing a reading across syllables

Each slice is what is sung during that slot's original text. Read the `syllables` from `get_lines`.

- `水溜りに|足を|突っ込ん|で` + `Mizu tamari ni  ashi o  tsukkonde` -> `["Mizu tamari ni", "ashi o", "tsukkon", "de"]`
- `貴方|は|大きな|あ|く|び|をする` + `Anata wa ookina akubi o suru` -> `["Anata", "wa", "ookina", "a", "ku", "bi", "o suru"]`
- Particles stay with the slot that holds them (に ni, を o, は wa). A small っ doubles the next consonant inside the slot that holds it (突っ込ん tsukkon).
- Kana, hangul blocks and Chinese characters each map to their own reading, so one-character slots get one-syllable slices.
- When a split is uncertain, send `dryRun: true` first and check that `remainingIssues` is empty.

## Errors

- `Unknown lineId` or "no longer open in this tab": the user switched projects. Call `list_projects` and `get_lines` again; never reuse ids across projects.
- "didn't answer in time" or "disconnected before answering": the write may have landed. Read the lines before retrying.
- "in the middle of an edit": the user is dragging something. Wait a moment and retry.
