# Sehaty

A personal health agent: training, nutrition and weight, reachable from an AI assistant
over [MCP](https://modelcontextprotocol.io) (Telegram planned). Multi-person, and built so
that one person's health data is never reachable from another's session.

> **Status: early.** The MCP server, encrypted storage and exercise catalog work and are
> tested. Voice notes, meal photos, Telegram, the dashboard and AI-assisted planning are
> not built yet. See [Status](#status).

## Why it exists

Health data is about as personal as data gets, and most fitness apps answer "where does
this live?" with "our servers". Sehaty answers "a machine you control, encrypted at rest,
with the key somewhere the backup can't reach."

## Design

**Deterministic tools, not a chatbot.** Sets, reps, rest and prescriptions come from code,
not from a model deciding each time. An assistant is a good interface; it is not a good
place to keep the number of reps you did.

**Isolation before encryption.** Every query is filtered by `profile_id` in a single data
access layer nothing bypasses. Encryption protects a stolen disk; isolation protects you
from a bug.

**Encrypted at rest, in layers that do different jobs:**

| layer | cipher | protects | authenticated |
|---|---|---|---|
| database file (incl. WAL) | Adiantum, via [ncruces/go-sqlite3](https://github.com/ncruces/go-sqlite3) | every table | no — length preserving |
| document bodies | AES-256-GCM, AAD-bound to profile/key/version | clinical + prescriptive text | yes |
| media blobs | AES-256-GCM, AAD-bound to profile/kind/hash | photos, voice notes | yes |

`SEHATY_KEY` is a master; each layer gets an independent HKDF-SHA256 subkey, so no two
ciphers ever run under the same bytes.

**What this does not defend against.** The key is in the server's environment, so a running
server can always decrypt. This protects backups, a stolen disk and a copied image. It does
not protect against someone who has compromised the running process. That limit is the
price of Sehaty being able to answer you without you typing a passphrase.

## Quick start

The exercise dataset is a submodule, so clone recursively:

```bash
git clone --recurse-submodules https://github.com/rakasatria/sehaty.git
cd sehaty
go build -o sehaty ./cmd/sehaty
```

Generate a key and keep a copy somewhere that is **not** the server:

```bash
openssl rand -base64 32 | tr -d '\n' > /etc/sehaty/key
chmod 0440 /etc/sehaty/key
```

**Losing this key means losing every encrypted record. There is no recovery path, and that
is the point.**

```bash
SEHATY_KEY_FILE=/etc/sehaty/key \
SEHATY_DATA=/var/lib/sehaty \
SEHATY_EXERCISES=third_party/exercises-dataset/data/exercises.json \
./sehaty
```

Then point an MCP client at `http://127.0.0.1:8765/mcp`.

## Configuration

| variable | default | notes |
|---|---|---|
| `SEHATY_KEY_FILE` | — | path to the base64 master key. **Preferred over `SEHATY_KEY`** |
| `SEHATY_KEY` | — | the key itself. Visible in `docker inspect` and `ps` — use the file |
| `SEHATY_DATA` | `/var/lib/sehaty` | root of all mutable state |
| `SEHATY_DB` | `$SEHATY_DATA/sehaty.db` | encrypted SQLite |
| `SEHATY_MEDIA` | `$SEHATY_DATA/media` | blobs, `<profile>/<kind>/<ab>/<sha256>` |
| `SEHATY_EXERCISES` | `/opt/sehaty/exercises.json` | read-only dataset |
| `SEHATY_PASSPHRASE` | — | gates registration. Unset means **registration closed** |
| `SEHATY_MCP_HOST` | `127.0.0.1` | **MCP is a tool-execution surface — do not publish it** |
| `SEHATY_MCP_PORT` | `8765` | |

**The key must live outside `SEHATY_DATA`.** Sehaty refuses to start otherwise: a key inside
the directory you back up travels in the same archive as the ciphertext it unlocks. Note
that this check cannot see *your* backup tool — if you image the whole machine, put the key
somewhere that image does not reach.

## Tools

`register` · `list_profiles` · `get_profile` · `plan_session` · `find_exercises` ·
`log_set` · `log_cardio` · `log_weight` · `progress` · `cardio_protocol`

## Status

**Works:** MCP server, encrypted storage, profile isolation, training/cardio/weight/food
logging, versioned encrypted documents, exercise catalog with equipment filtering, media
store.

**Not built:** AI-assisted planning, difficulty ratings for the full dataset (only a handful
are rated, so difficulty filtering is effectively inert), voice notes, meal photos, Bahasa
Indonesia, Telegram, dashboard.

## Safety

Sehaty is not a doctor and does not diagnose. It will not set a calorie target below a safe
floor, and prescriptions from a real clinician are stored as documents rather than
second-guessed.

## Credits

Exercise data from [hasaneyldrm/exercises-dataset](https://github.com/hasaneyldrm/exercises-dataset)
(MIT, © Hasan Emir Yıldırım) — 1,324 exercises.

## Licence

MIT — see [LICENSE](LICENSE).
