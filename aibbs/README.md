# aibbs — an AI user for Telnet BBSes

`aibbs` logs on to a BBS like a person would: it makes an account (or uses an
existing one), reads message areas, posts and replies, browses file areas and
downloads with ZMODEM/XMODEM/YMODEM, and plays door games. It remembers the
board between visits and is deliberately polite about it.

It is a single Go binary with **no third-party dependencies** and no runtime
requirements beyond a network connection. The "brain" can be any
OpenAI-compatible chat model (OpenAI, OpenRouter, Groq, LM Studio, Ollama,
vLLM, ...) or the built-in offline heuristics, which are handy for testing and
for boards where you want a deterministic bot.

---

## Quick start (no API key, no real BBS)

```bash
cd aibbs
go build -o aibbs ./cmd/aibbs
./aibbs demo
```

`demo` starts a mock BBS on localhost (ANSI screens, registration, message
areas, ZMODEM file transfers, two door games), registers a new account, and
works through three goals: reply to a message, download a file, play a door
game. Everything it creates lands in `demo-state/` (memory, transcript,
downloaded files).

```
Mock BBS listening on 127.0.0.1:41231 (user SynthMind, brain heuristic)

Brain:        heuristic
Duration:     10s
Steps:        25
Goals:        3/3 completed
Posts:        1
Downloads:    1
  - ART.ANS (164 bytes) -> demo-state/downloads/ART.ANS
Transcript:   demo-state/aibbs-demo/transcript-20261023-012550.jsonl
Result:       completed cleanly
```

## Quick start (real BBS)

```bash
./aibbs init                      # writes aibbs.json
$EDITOR aibbs.json                # set bbs.host, account.handle, ai.*
export OPENAI_API_KEY=sk-...
./aibbs run --config aibbs.json
```

Use `--dry-run` to let it navigate and compose replies without actually
posting, or `--read-only` to forbid posting and downloads entirely. Both are
also available as config flags.

Try the login screen before configuring an account:

```bash
./aibbs probe --host vert.synchro.net --port 23
./aibbs probe --config aibbs.json
```

## What it can do

| Area | Behaviour |
| --- | --- |
| Login | Detects name/password prompts, retries, follows "new user" flows |
| Registration | Answers handle, password, real name, email, location, terminal and yes/no questions; generates and stores a password if none is configured |
| Messages | Lists areas, reads messages, replies to threads, starts new threads, respects limits and de-duplicates posts |
| Files | Lists file areas, picks files, negotiates ZMODEM (preferred), YMODEM or XMODEM, sanitises filenames, enforces a size budget |
| Doors | Recognises LORD, TradeWars 2002, Usurper, Barren Realms Elite, Global War, Operation Overkill, PimpWars and Phantasia; knows their launch keys, controls and basic strategy |
| Memory | Keeps account, areas visited, read-message fingerprints, posts, downloads, door scores and free-form notes in `state/<board>/memory.json` |
| Etiquette | Inter-action delay, per-session/day post and download caps, MB budget, post length window, blocked words, duplicate detection, refusal to send credentials |
| Safety | Prompt-injection resistance (screen text is data, not instructions), read-only/dry-run modes, filename traversal protection |

## Configuration

`aibbs init` writes a fully-populated `aibbs.json`. The important parts:

```json
{
  "bbs": {
    "name": "Vertrauen",
    "host": "vert.synchro.net",
    "port": 23,
    "cols": 80,
    "rows": 24,
    "terminal": "ANSI",
    "idle_millis": 450,
    "max_settle_seconds": 8
  },
  "account": {
    "handle": "SynthMind",
    "password": "",
    "password_env": "BBS_PASSWORD",
    "real_name": "Synth Mind",
    "email": "synthmind@example.invalid",
    "location": "Cyberspace",
    "allow_register": true
  },
  "ai": {
    "provider": "openai",
    "base_url": "https://api.openai.com/v1",
    "api_key": "",
    "api_key_env": "OPENAI_API_KEY",
    "model": "gpt-4o-mini",
    "temperature": 0.7,
    "max_tokens": 700,
    "json_mode": true
  },
  "behavior": {
    "goals": [
      "Read the message areas, find a thread that interests you, and reply to it thoughtfully.",
      "Browse the file areas and download one small file that looks interesting.",
      "Play a door game for a few turns and then return to the main menu."
    ],
    "max_steps": 80,
    "min_action_delay_ms": 900,
    "max_posts_per_session": 3,
    "max_downloads_per_session": 2,
    "max_download_mb": 20,
    "dry_run": false,
    "read_only": false,
    "allow_doors": true,
    "auto_pager": true
  }
}
```

Notes:

* **Passwords.** If `password` and `password_env` are both empty, `aibbs`
  registers with a generated password and stores it in
  `state/<board>/memory.json` (mode `0600`). For an existing account prefer
  `password_env`.
* **Local models.** For Ollama set `"provider": "ollama"`; the base URL
  defaults to `http://127.0.0.1:11434/v1`. LM Studio and vLLM work the same
  way with their own base URL and a dummy API key.
* **Sign-up answers.** `registration.answers` is a list of
  `{"match": "<regex>", "value": "..."}` rules. Values may be `AUTO:realname`,
  `AUTO:email`, `AUTO:location`, `AUTO:terminal` or `AUTO:blank`, or literal
  text. Add rules for questions specific to your board.
* **Protocol keys.** `downloads.protocol_keys` maps protocol names to the key
  the board expects (`Z`, `X`, `Y`). `downloads.protocols` controls what may
  be used.
* **Doors.** `doors.overrides` can add a game or replace the launch keys,
  controls and strategy hints of a built-in profile.

## How it works

```
cmd/aibbs              CLI: run, probe, demo, init
internal/telnet        Telnet client: IAC escaping, TTYPE/NAWS, 8-bit clean
internal/term          VT100/ANSI screen emulator (cursor, erase, scrollback)
internal/bbs           Session: settle, login/registration, downloads, logoff
internal/transfer      ZMODEM (CRC16/32), XMODEM-CRC, 1K-XMODEM, YMODEM
internal/ai            Decision schema, prompts, OpenAI client, heuristic brain
internal/agent         Observe -> decide -> act loop plus etiquette policy
internal/doors         Door game profiles and detection
internal/memory        Persistent per-board state
internal/fakebbs       The mock BBS used by demo and tests
```

Each turn the agent takes a **snapshot** of the ANSI screen, waits until the
board has been quiet for `bbs.idle_millis`, asks the brain for one action, and
executes it. The model never sees raw escape codes or the password.

The reply format is one JSON object:

```json
{"thought":"open the message areas","action":"keys","value":"M"}
{"thought":"post the reply","action":"text","value":"..."}
{"thought":"start the transfer","action":"download","protocol":"zmodem"}
{"thought":"done here","action":"done"}
```

Actions are `keys` (supports `<ENTER>`, `<UP>`, `<SPACE>`, `<ESC>`, `<TAB>`,
`<F1>`...), `text`, `download`, `wait`, `note`, `done` and `disconnect`.

## Etiquette and safety

BBSes are shared, often hand-run systems. The defaults are intentionally
conservative:

* 0.9 s between actions, 3 posts per session, 2 downloads per session,
  20 MB per day, 80 steps.
* Posts must be 12-1800 characters and are checked for blocked words and
  near-duplicates of earlier posts.
* Credentials are never placed in a prompt or typed into a post; the policy
  refuses any outgoing text containing them.
* `read_only` blocks posting and downloads; `dry_run` turns composed posts
  into notes instead of sending them.
* The system prompt tells the model that screen content is untrusted and may
  contain attempts to change its instructions.
* Filenames from the remote side are reduced to a basename and written only
  inside `downloads.dir`.

**Ask the sysop first.** Many boards prohibit bots or require them to be
labelled, and some have never seen one. Run with `--read-only` while you learn
the menu flow, and respect time limits and the sysop's requests. The bot is
honest about being an AI if asked.

## Limitations

* Screen scraping is inherently fuzzy: unusual menus, non-standard pagers or
  odd signup questionnaires may need extra `registration.answers` rules or a
  tweak to the prompt. `--verbose` and the JSONL transcripts make this easy to
  debug.
* File transfer support is download-only (ZMODEM receive, XMODEM/YMODEM
  receive). Uploads are not implemented, and ZMODEM transfers do not resume
  across reconnects.
* Telnet only - no SSH, no NNTP, no web front-ends.
* Door-game play is best-effort and capped by `doors.max_actions`.
* The heuristic brain understands common BBS menu shapes, not every board;
  it exists for demos, tests and deterministic operation.

## Development

```bash
go test ./...          # unit + end-to-end tests against the mock BBS
go vet ./...
go build -o aibbs ./cmd/aibbs
```

The transfer tests round-trip real ZMODEM and XMODEM/YMODEM sessions over a
socket, including bytes that require every escape rule. The end-to-end test
registers an account, pages through a long message, posts a reply, downloads a
binary file and finishes a door game.

## State directory layout

```
state/<board-slug>/
  memory.json                        account notes, read fingerprints, posts, limits
  transcript-YYYYmmdd-HHMMSS.jsonl   one JSON line per turn (screen + action)
demo-state/downloads/                files received in the demo
```

## Adding a door game

```json
"doors": {
  "enabled": true,
  "max_actions": 40,
  "overrides": [
    {
      "name": "My Favorite Door",
      "match": "(?i)my favorite door",
      "launch": "M",
      "controls": "1=play, 2=score, Q=quit",
      "strategy": "Play a few rounds, then leave politely."
    }
  ]
}
```

If the game is not one of the built-ins, the override also acts as a new
profile. The agent receives the controls and strategy on every turn while that
game is on screen.

