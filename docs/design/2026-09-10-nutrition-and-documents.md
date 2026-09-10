# Sehaty — Nutrition, Documents and Tool Design

**Status:** design decisions, researched 10 Sep 2026. Not yet implemented.
**Informs:** `log_food`, the document tool surface, the food-data layer, and revisions to
the 11 tools already shipped.

Four parallel research streams fed this: clinical document modelling, versioned markdown
storage, MCP tool conventions, and nutrition data sources with an Indonesian focus. Where a
number or a claim is load-bearing it is cited. Where a decision is judgement rather than
evidence, it says so.

---

## 1. The decision that shapes everything else

**Sehaty never computes a nutrition target.** Not a calorie goal, not a macro split, not a
deficit — whether the number would be safe or not.

This is not caution for its own sake. Targets belong to the clinician who examined the
person. The software's job is to record what they prescribed, faithfully, and to report
honestly what was eaten against it. Every other decision below follows from that.

The corollary matters as much: **the software never fills a gap.** A blank macro header
stays blank. An unreadable line stays unreadable. Both become recorded facts (§3), not
inferences.

---

## 2. Documents

### 2.1 Versioning stays as built — full copy per version

Full-copy-per-version is correct at this scale, not a compromise. 50 documents × 100
versions × 10 KB is ~50 MB across years.

The decisive argument is encryption. Bodies are AES-256-GCM ciphertext, so deltas cannot be
computed at the storage layer — you would decrypt, diff, re-encrypt, and then reconstructing
version *N* means chaining every prior version. One corrupt intermediate silently destroys
everything after it. Full copies are independently recoverable and each ciphertext is
verifiable in isolation against its AAD.

Git makes the same choice: full snapshots at write time, delta compression only later as a
*physical packing* optimisation for millions of objects. We have dozens.
([Git Internals — Packfiles](https://git-scm.com/book/en/v2/Git-Internals-Packfiles))

**Deletion:** tombstone version (body empty, `deleted` flag) so it stays reversible — the
primary writer is an LLM agent, and agents delete wrong things. A separate hard **purge**
exists for the human's right to destroy, dashboard-only, never exposed as a tool.

**Retention: indefinite.** No auto-pruning. Version history *is* the audit trail for "why
does my diet document say this now?", and pruning destroys exactly the evidence needed when
the agent misbehaves. Health-records retention conventions run 5–10 years and longer, and
since a provider may not keep records that long, the self-hosted copy may eventually be the
only surviving longitudinal record. (Those conventions bind *providers*, not a personal
record — cited as domain evidence, not compliance.)

### 2.2 Document keys are immutable — a consequence we did not foresee

Our AAD binds ciphertext to `profile|key|version`. **The key therefore participates in
authentication**, so renaming a document means re-encrypting every version.

Decision: **accept it.** Re-encrypt on rename, dashboard-only, rare. The alternative — an
internal numeric id for AAD with the slug as a mutable alias — adds a layer for a problem we
do not have.

### 2.3 Preventing the agent from fragmenting the namespace

The real risk is an agent creating `diet-plan`, `dietplan` and `diet_plan` as three
documents. Four layers, cheapest first:

1. **Server-side canonicalisation** — NFKC, lowercase, collapse separators, enforce
   `^[a-z0-9]+(-[a-z0-9]+)*$`. Makes `Diet_Plan` and `diet plan` the same document by
   construction. The *store* owns spelling discipline, not the agent.
2. **Fuzzy-match guard on create** — compare against existing keys with separators
   stripped, so `dietplan` ≈ `diet-plan`. On near-match, refuse with an actionable error
   naming the existing key.
3. **Explicit create intent** — `create_new: true` required to mint a key. An update to a
   nonexistent key fails with the list of existing keys rather than silently creating one.
4. **Seeded vocabulary** — every profile starts with `diet`, `injury-history`,
   `clinical-notes`, `medications`. Agents reuse keys they can see; duplicates arise when
   the namespace is invisible.

### 2.4 No YAML front matter

Front matter exists because in Hugo and Obsidian *the file is the database*. We have a real
database with typed columns. Embedding metadata in an encrypted blob would create a worse,
unqueryable copy of data we already store — and `ListDocuments` deliberately returns no
bodies, so any metadata inside the body is metadata we cannot see without decrypting.

The sharper risk: an LLM rewriting a body will happily "update" an embedded `updated:` date
incorrectly or drop the block entirely. Then blob and columns disagree, and nothing knows
which is right.

**Decision:** body is pure markdown. Add `title`, `tags`, and a one-line `summary` as
columns. Synthesise front matter only on export.

### 2.5 `expected_version` on writes

Required for `mode: replace`. On a single-user box this is barely about concurrency — its
real value is as a fence against the best-documented agent failure: acting on a copy read
earlier in the conversation. A stale agent *cannot* clobber the store; it gets a conflict
naming the current version and must re-read.

This is HTTP's `If-Match` / 412 pattern, repurposed as an agent guardrail.

`mode: append` may omit it — appends commute well enough for a personal record, and
requiring read-before-append adds friction where risk is lowest.

### 2.6 Search: not building it

Dozens of documents with key, title, tags and summary fit in one `document_list` call. An
LLM reading that listing does *better* than keyword search — it matches "the thing my
nutritionist said about iron" semantically, which FTS cannot.

And a plaintext FTS index would leak exactly what the AAD scheme protects.

If search is ever genuinely missed, the answer is **decrypt-and-scan**, not an index: 50
documents × 10 KB is 500 KB, scannable in milliseconds, holding nothing.

---

## 3. Nutrition prescriptions — recording what we cannot read

### 3.1 The pattern that solves the blank macro header

FHIR has [`data-absent-reason`](https://hl7.org/fhir/valueset-data-absent-reason.html): any
element may carry a coded reason for missing data *instead of* a value. Four codes suffice:

| code | meaning |
|---|---|
| `not_specified` | legible, and the sheet simply doesn't say |
| `illegible` | the answer exists on paper; a better photo or a phone call resolves it |
| `ambiguous` | legible but structurally uncertain — verbatim preserved |
| `not_applicable` | doesn't apply to this plan |

`illegible` and `not_specified` are **different facts with different remedies**, and
collapsing them into "missing" throws away the one that is actionable.

### 3.2 The three unknowns on Raka's sheet, stored faithfully

| problem | representation |
|---|---|
| `telur / oats` — AND or OR? | item group with `relation: "unresolved"`, both items stored, `verbatim` preserved |
| 16:00 slot unreadable | slot exists with `time: "16:00"`, `items: []`, `items_absent_reason: "illegible"` |
| protein header blank | `targets.protein_g: {absent_reason: "not_specified"}` |

Each raises an entry in a document-level **`open_questions[]`** list, so the agent can
recite them before the next appointment. Resolution is a **human** act creating a new
version — the software never flips `unresolved` to `one_of` on its own, even though a slash
on a diet sheet almost always means OR. "Almost always" is exactly the inference this design
forbids automating.

The compliance checker can then say honestly: *kcal checkable; protein target not stated
(not zero, not estimated); breakfast satisfied by eggs or oats pending q1; 16:00 slot
unassessable pending q2.*

### 3.3 Structure is a derived index; verbatim is the truth

**Position: the markdown transcription plus the source photo are authoritative. The
structured JSON is a lossy, honest index that only the deterministic checker consumes.**

A field earns structure only if the checker reads it: targets, slot times, item names and
portions, choice relations, exclusions. Rationale, encouragement and conditional advice
("if hungry, add a fruit") stay in the clinician's words, for the agent to *quote* rather
than compute over.

Failure asymmetry decides it: over-structuring fails silently (nuance shredded into an enum
is gone); under-structuring fails loudly ("can't evaluate this"). Choose loud.

This mirrors professional practice — ADIME notes are narrative with a small structured
spine, and FHIR itself carries free-text `instruction` at every level.

### 3.4 Portions in the unit as written

Dietitians write household measures — "1 cup rice", "2 eggs", "2 starch exchanges". Forcing
grams at transcription time is a transcription lie. Store `{value, unit, unit_system}` as
written; `grams_estimate` is a separate, clearly second-class, app-derived field.

**Substitutions are prescribed, never inferred.** When a dietitian writes alternatives they
have done the equivalence maths. The software records the set and never extends it —
"chicken ≈ fish so fish is fine" is precisely the second-guessing forbidden in §1.

---

## 4. Food data — three tiers, chosen on licence as much as coverage

The blunt finding: **no openly-licensed Indonesian food composition dataset exists.**

| source | licence | Indonesian dishes | verdict |
|---|---|---|---|
| USDA FoodData Central | **CC0** | none (measured: "nasi goreng" → 7 Western seasoning mixes) | **embed** — raw ingredients |
| Open Food Facts | **ODbL** (share-alike) | 4,693 Indonesian *packaged* products; zero dishes | **barcode lookup only** |
| TKPI / Kemenkes | **© All Rights Reserved** | ~1,113 items, exactly what's needed | **cannot bulk-ship** |

### 4.1 The tiers

1. **FDC Foundation + SR Legacy embedded in the binary.** CC0, so no strings against our
   MIT licence. Small enough to ship. Covers rice, chicken, egg, oil, tofu, tempeh.
2. **Open Food Facts for barcodes.** Live lookup, or a pre-filtered Indonesian subset from
   the nightly dump. **Kept in its own file with its own licence notice** — ODbL is
   share-alike, so a *derived database* mingling OFF with other sources must itself be
   ODbL. Do not merge it into any redistributed table.
3. **A hand-curated Indonesian dish table (100–300 entries)** — the dishes actually eaten,
   per-100g macros, **a citation on every row** (`TKPI 2019 item F-0123`, or "computed from
   FDC ingredients"). Curating a few hundred referenced values is defensible; shipping all
   1,113 TKPI rows is not clearly legal.

Optionally also ship an **importer** so a user can load the Kemenkes PDF themselves — we
distribute code, they hold the data.

### 4.2 Endpoints, verified live 10 Sep 2026

```
cgi/search.pl                              → 503   dead, as we found in the prototype
search.openfoodfacts.org/search?q=…        → 200   text search (Search-a-licious)
world.openfoodfacts.org/api/v2/product/…   → 200   barcode (v3 is current)
```

Rate limits are **low** — documented at 15 req/min for product reads, 10 for search, with
IP bans for abuse and a required custom `User-Agent`. Design for the bulk dump, not the API.

### 4.3 Portions: use URT, and state the error honestly

Indonesia has a standard for exactly this — **URT (Ukuran Rumah Tangga)**, codified in
Permenkes 41/2014 and used in Kemenkes dietary surveys (1 porsi nasi = 100 g = ¾ gelas =
175 kkal). Ship a URT→gram table, labelled as estimates.

**The honest error is large.** Household measures perform worst on amorphous foods, and rice
dishes are the worst case. Published values for one plate of nasi goreng span **250 kcal to
700+ kcal** — oil alone swings 210–300 kcal. That is roughly **±50%**, a 2–3× spread.

Consequence: store a **range, not a point**, and never render an unweighed composite dish to
three significant figures.

---

## 5. Safety floors — the numbers we refuse on

| rule | value | source |
|---|---|---|
| typical prescribed deficit | 1,200–1,500 kcal/d women; 1,500–1,800 men | [AHA/ACC/TOS 2013](https://www.ahajournals.org/doi/10.1161/01.cir.0000437739.71477.ee) |
| very-low-calorie | **<800 kcal/d only under medical supervision** | AHA/ACC/TOS; [NICE NG246](https://www.nice.org.uk/guidance/ng246/chapter/Physical-activity-and-diet) |
| safe rate of loss | 0.5–1 kg/week | NHS / CDC |
| protein, fat loss + training | 1.6–2.2 g/kg/d | [Morton 2018](https://pubmed.ncbi.nlm.nih.gov/28698222/), [ISSN 2017](https://jissn.biomedcentral.com/articles/10.1186/s12970-017-0177-8) |

**Framing matters.** The "never below 1,200/1,500" figure is the bottom of what major
guidelines *prescribe without supervision* — it is guideline convention, not an RCT-derived
biological threshold. The app should say so. Claiming a "scientifically proven minimum"
would be overstating it.

**Do not compute at all** for: pregnancy or breastfeeding, under-18s, BMI < 18.5 with a
weight-loss request, disclosed eating-disorder history, or conditions where energy and macro
needs change (diabetes on medication, kidney disease, thyroid disorders). Say it needs a
*dokter* or *ahli gizi* and stop.

**Never silently clamp.** Refuse, name the floor, cite it, offer the guideline-compliant
alternative.

A note on eating disorders: 73% of surveyed ED patients reported calorie-tracking apps
contributed to their disorder. If a user repeatedly pushes targets below floors, the right
response is to stop computing and keep logging neutral — no deficit framing at all.

---

## 6. AI food identification — good at *what*, bad at *how much*

Measured, not assumed:

- Identification: **87–93%** accurate.
- Quantity: **~40% mean error** on weight and energy; **42–110%** on macros; **>60% on
  protein** across three models.
- Text descriptions: best result **66.8%** on carbohydrate estimation within tolerance
  ([NutriBench](https://arxiv.org/abs/2407.12843)).
- No published evaluation on Indonesian dishes. Mixed dishes with hidden oil and sauces are
  the documented worst case, so **≥±50% for a photo of nasi goreng** is an informed
  extrapolation, not a measurement.

**Therefore the model identifies; the database quantifies.** The LLM names the dish and
estimates the portion. Macros come from the deterministic tiers in §4. The model must never
emit a kcal number itself.

Presentation rules: `source` becomes an enum (`fdc` | `off-barcode` | `curated` | `user` |
`ai-estimate`) surfaced in every reply; show ranges; below a confidence threshold log
"unidentified — grams and macros unknown" rather than guessing; a user correction is stored
as `source: user` and becomes ground truth.

Accuracy improves markedly when preparation context is supplied — which argues for the
assistant asking *"pakai minyak berapa banyak? ada kerupuk?"* rather than silently guessing.

---

## 7. Tool design — corrections to what we already shipped

### 7.1 `profile` is a privacy boundary enforced on trust

Every tool takes a model-supplied `profile` string. The MCP security guidance is explicit
that a server **must not** treat a client-supplied handle as authentication — identity
should derive from a verified credential.

**Sehaty has no authentication at all.** Anything that reaches `:8765` can read or write any
person's data by naming their profile. Acceptable while there is one profile on a LAN-only
port; not acceptable the moment a second person registers, which is the whole multi-person
premise.

Mitigations, strongest first:

1. Bind identity to a transport credential; derive the allowed profile set from the verified
   token.
2. **Make `profile` a JSON Schema `enum`** of registered ids, regenerated on `register` with
   `listChanged`. A typo becomes a validation error instead of a silent wrong-person write.
   **Never auto-create a profile from an unknown id in a log tool.**
3. **Echo the display name in every write result** — "Logged 5×80 kg bench press for
   **Raka**." The human in the chat is the last-line auditor.
4. Unknown profile → `isError` naming the registered profiles, so the model self-corrects.
5. Audit-log every write server-side.

### 7.2 Duplicate writes are a live risk, not a theoretical one

The 2026-07-28 spec **removed SSE resumability**: a client whose response stream breaks
**must** re-issue the request. A `log_set` that commits and then loses its stream will be
retried.

Client-supplied idempotency keys do not save us — an LLM is an unreliable key generator; it
mints a fresh UUID per retry or embeds `now()`. And keys never catch the *other* case: the
user rephrases and the model logs again.

**Decision: server-side dedup window.** Identical `(profile, tool, salient fields)` within
~5–10 minutes returns the existing record with `status: "duplicate"` and text saying so.
Accept an optional `idempotency_key` for well-behaved clients, but never rely on it.

Every write returns the full created record — id, server timestamp, echoed fields — so the
write is visible, verifiable and correctable.

### 7.3 Annotations, and why silence is dangerous

Defaults are **pessimistic**: an unannotated tool is assumed non-read-only, destructive,
non-idempotent and open-world. Leaving our read tools unannotated marks them as destructive.

| tools | readOnly | destructive | idempotent |
|---|---|---|---|
| `list_profiles`, `get_profile`, `find_exercises`, `progress`, document reads | **true** | — | true |
| `register` | false | **false** | false |
| `update_profile` | false | **true** | true |
| `log_set`, `log_cardio`, `log_weight`, `log_food` | false | **false** | **false** |
| `put_document` | false | **false** (versioned) | false |

All `openWorldHint: false` — closed domain. Annotations are hints, never a security
boundary; set them accurately anyway.

### 7.4 Keep the log tools separate

Evidence on tool-count degradation concerns *dozens to hundreds* of tools — at 11 going on
15 we are far inside the safe zone, and there is no evidence that merging into one
`log(type=…)` would help.

Against merging: each domain has different **required** fields, and a discriminated tool
needs `oneOf`-conditional requirements that pre-2026 MCP schemas cannot express. We would
lose schema-level enforcement and validate in prose — the opposite of "narrow and hard to
misuse".

### 7.5 Descriptions: four sentences, always

Tool descriptions are measurably load-bearing. Convention:

1. **Action** — what it does, to whose data.
2. **Trigger** — when to call it, in user language ("when the user mentions eating, meals,
   or asks to track calories").
3. **Boundary** — when *not* to, and what it does not do. This is where the deterministic
   philosophy gets encoded: *"Does not choose sets or reps — Sehaty prescribes those."*
4. **Outcome** — what comes back, including duplicate behaviour.

Units belong in parameter **names** (`weight_kg`, `calories_kcal`), which survive a model
skimming the description, as well as in schema descriptions.

### 7.6 Absent data

Our choice of `null` over `0` for a missing body weight is **validated** — a fabricated zero
is a false datum a model will average or report. Strengthen it: pair the null with a count
and a ready-made sentence (`{"body_weight_kg": null, "entries_in_range": 0}` → *"No
body-weight entries for Raka in the last 30 days."*). Give the model the sentence and it
will not invent one.

---

## 8. What we will not build

1. Generating or suggesting calorie/macro targets, deficits, or meal plans — ever.
2. Auto-resolving ambiguity: AND/OR inference, gap backfilling, OCR auto-accept.
3. Extending substitution lists by nutritional similarity.
4. Enteral feeding, texture and fluid-consistency modifiers — hospital care.
5. Coded terminologies (SNOMED, NCPT, LOINC) and FHIR CodeableConcept machinery.
6. Structured diagnosis fields — notes stay narrative; Sehaty is not a diagnostician.
7. An exchange-computation engine — store `exchange:starch` as a unit, don't auto-convert.
8. Drug–nutrient interaction checking, micronutrient adequacy, allergy inference.
9. A plaintext full-text search index.
10. A document delete tool — versioning plus tombstones covers it.

---

## 9. Open risks and what needs a decision

**Needs Raka's decision:**

- **TKPI licensing.** All-rights-reserved with no open licence. Curate 100–300 cited rows,
  ship a user-side importer, or both? This is the difference between Sehaty knowing what
  gado-gado contains and not.
- **Authentication.** Adding a bearer token to the MCP endpoint changes how Hermes and any
  future Telegram bot connect. Worth doing before a second person registers.

**Flagged uncertainties from the research itself:**

- The 1,200/1,500 kcal floors are guideline convention, not experimental thresholds — the
  app's refusal text must say so.
- Energy equations were developed in Western populations; Asian validation studies suggest
  overestimation, with no authoritative Indonesian correction factor. Present ±10–20%.
- Activity multipliers (×1.2 … ×1.9) are consumer convention loosely derived from FAO/WHO
  PAL bands, not a citable trial. Label them "conventional estimates".
- OFF's documented rate limit appears to have dropped from 100 to 15 req/min — reinforcing
  the bulk-download design.
- No Indonesian-specific evaluation of LLM food estimation exists.
- FHIR `NutritionIntake` is maturity level 1 (trial use) — patterns borrowed, not the
  resource.

**Still outstanding from earlier work, unrelated to this research:** the master key exists in
exactly one place and is deliberately excluded from backups; gocryptfs boot persistence is
enabled but unwitnessed.
