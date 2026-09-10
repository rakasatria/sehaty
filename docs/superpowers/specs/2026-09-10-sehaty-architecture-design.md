# Sehaty architecture — design

**Date:** 2026-09-10
**Status:** agreed in brainstorm, pending implementation plan

Sehaty is a private, self-hosted personal health record that people talk to through
Telegram and read through a Mini App. It records training, food and weight, and helps
someone keep a habit.

**A note on pronouns.** Throughout this document, *the owner* is the person whose record it
is. *Sehaty* is the software. Where a quoted line from `soul/` says "you", it is addressing
Sehaty — those files are written as instructions to the agent, not to a person.

This document covers five subsystems agreed section by section: the capability layer, the
programme, the soul, evidence refresh, and administration. It assumes what already exists —
encrypted storage, the tool surface with its refusals, the agent loop, the Mini App, and a
half-built scheduler.

---

## 1. The founding constraints

Everything below is downstream of four rules that do not bend.

**The model never produces a number.** Every figure a person sees is computed in Go, next
to the data, by code that can be tested. A model asked to multiply a weight produces
something indistinguishable from a correct answer.

**`null` is not zero.** "Not recorded" and "none" are different facts and are presented
differently. Collapsing them turns *we have no idea* into *you did nothing*.

**This data never trains anything.** `provider.data_collection = "deny"` on every request,
asserted by a test against the wire.

**Nobody reads anyone else's record.** Not another user, not an admin, not the person
running the server. Tools confine to one profile in one place; ciphertext is AAD-bound to
`profile | key | version` so bytes cannot be moved between records even by someone holding
the master key.

### The language boundary

**Go decides what is true. TypeScript decides how it feels.**

Go owns storage, encryption, every tool and every refusal, all arithmetic, all formatting
(rounding is a truth claim), authentication, the agent loop, MCP, Telegram and the
scheduler. TypeScript owns the Mini App: layout, motion, navigation, gesture, haptics — and
computes nothing.

The interface is one authenticated JSON endpoint returning display-ready values. **TS types
are generated from the Go structs** so the contract cannot drift by hand.

---

## 2. Capability layer

Tools declare what they need. The assessment is derived from that declaration, rather than
maintained as a parallel list that will drift from it.

```go
type Requirement struct {
    Field   string   // age, height_cm, sex, weight_log, equipment, schedule
    Because string   // the true reason, shown to the person
    Hard    bool     // false = proceeds without it, and says so
}

type Capability struct {
    Tool    string
    Needs   []Requirement
    Unlocks string
}
```

| tool | hard | soft | unlocks |
|---|---|---|---|
| `log_food`, `log_set`, `log_weight` | — | — | works from minute one |
| `estimate_energy` | age, height, sex, a logged weight | — | an energy estimate |
| `plan_session` | equipment, experience | injuries | a session |
| `draft_programme` | goal, schedule, equipment, experience | injuries, diet preference | a programme |

**Three readers, one registry.** The tool refuses on unmet hard requirements. The brief
derives what is still unknown, in unlock order, with the true reason attached. The Mini App
shows what is locked and why.

`storage.Assessment` is deleted. `Missing()` becomes the union of unmet requirements.

**Hard versus soft.** Hard means refuse — a number would have to be invented. Soft means
proceed *and name the gap*: "aku belum tahu ada cedera atau nggak, jadi ini asumsi badanmu
sehat." That is a different failure from silence, and it keeps day-one friction low, which
matters when only ~10% of beginners are still training at 52 weeks.

**Why the rationale is structural.** A meaningful rationale is one of only three things
shown to reliably help someone take something on (Deci et al. 1994). Deriving the question
from a tool requirement makes the reason true by construction instead of composed.

---

## 3. The programme

### Shape

A **rotation**, not a calendar. An ordered list of sessions; whenever the owner trains, they do the
next one. A gap cannot create a debt, and there is no state in which the owner is behind.

This is deliberate. The best-designed study of lapses (Kirchner et al. 2012, 1,001 lapse
episodes) found guilt did **not** predict relapse — collapse in self-efficacy did. A
structure that cannot tell someone they are behind cannot erode it.

### Storage

```
programme            profile_id, name, status (draft|active|retired)
programme_session    programme_id, position, name
programme_movement   session_id, exercise, sets, rep_low, rep_high
cursor               profile_id, programme_id, next_position
```

**No stored loads. No progression counters. No week-and-day.**

The programme is structure, the set log is truth, and progression is a pure function of the
two. Nothing to keep in sync; a rule change never migrates history; a programme change never
invalidates it.

### Progression

```
advance when all prescribed reps are hit across N consecutive sessions, N ≥ 2
progress reps within the range first, then load
increment small and configurable, default below 2.5 kg where equipment allows
no failure requirement in the rep target
repeated misses reduce load; never a scheduled deload
```

Each clause is evidence-led. RPE/RIR is unusable far from failure — trained men are off by 5.15 reps at a called 5 RIR, and 259 certified coaches by 4.8 — so autoregulation is out.
Self-selected load is badly calibrated (53% of 1RM; habitual "10-rep" loads allowing 16 ± 5
reps), so silence is out too. Load-versus-rep progression is a null, so progress the one
that is objectively logged. The **N-sessions clause is what makes a deterministic rule
defensible**: a single session sits inside the noise floor, where the median retest artefact
on a lower-body 1RM is 5.5 kg — the size of a conventional jump. Increment size has zero
studies behind it. Scheduled deloads have two null trials and one adverse, with participants
reporting lethargy rather than freshness.

### Flow

```
agent drafts   → status=draft, nothing live
owner reviews  → Mini App: edit movements, sets, rep ranges
owner approves → status=active, cursor=0
owner trains   → log sets; cursor advances on completion
next time      → cursor position, loads derived from the log
```

The agent drafts and explains but **cannot activate**. A draft still passes
`guardrails.Screen`, so it cannot prescribe around a stated injury.

---

## 4. The soul

```
internal/agent/soul/
  intent.md                 what Sehaty is for
  limits.md                 what it will never do
  skills/
    taking-a-food-entry.md
    after-a-gap.md
    asking-a-question.md
    declining-a-target.md
    noticing-progress.md
    when-they-push-back.md
prompt.go                   composes intent + limits + skills + generated capabilities
```

Embedded with `go:embed`.

### Provenance, and why it is stripped

Every claim carries its source, visible to a reader and invisible to the model:

```markdown
Never disagree, argue, correct, shame or criticise.   <!-- PAPER: MI-nonadherent -->
Praise the person; never award a token.               <!-- PAPER: +0.33 / −0.34 -->
Emoji in acknowledgement, never on numbers.           <!-- RULE: Raka -->
One question per message.                             <!-- JUDGEMENT: untested -->
```

Stripping saves tokens, and prevents something worse: a model that can see its citations
starts performing them. **Sehaty never says "research shows".** The papers decide what it
does and stay out of what it says.

`JUDGEMENT` is the tag that earns its keep — it marks a rule as somebody's confident guess.
During this design, five such claims were checked and all five were wrong.

### One soul, not one per profile

Character is code; preference is data. A per-profile soul means every user gets a prompt
nobody audited. The profile carries preferences — language, formality, whether to offer
buttons — injected into a reviewed structure.

### Generated capabilities

The capability registry emits what can and cannot be done and why. Nobody maintains it, so
it cannot drift from the tools.

---

## 5. Evidence refresh

A fortnightly job whose output is a **proposal**, never an edit.

```
every 14 days
  ├─ read every claim and tag from soul/skills/
  ├─ prioritise JUDGEMENT, then PAPER rules older than N months
  ├─ search for confirming AND disconfirming evidence
  ├─ write papers/<topic>.md
  └─ open a proposal: claim, evidence, suggested change
        ↓  admin reviews
     approved → skill updated, provenance retagged, CHANGELOG entry
```

**Why it proposes.** An agent editing its own behavioural rules from a fresh search would
introduce, silently and at speed, exactly the error class that produced five wrong claims
here — several because the popular version of a finding differs from what the paper says.

**Telling people, honestly.** Not "my knowledge has been updated", which is unfalsifiable.
Specific: what changed, what Sehaty believed before, and why it stopped believing it. Generated
from `CHANGELOG.md` so what a person is told and what actually changed cannot diverge.
Delivered only when something changed.

**Limitation, stated:** this keeps Sehaty *current*, not *correct*. A sweep finds new
papers; it does not fix a claim nobody thought to question. The `JUDGEMENT` tags are the
only defence against that, and they work only if written honestly.

---

## 6. Administration

### Roles

| role | how | number |
|---|---|---|
| **owner** | the master key holder; the person running the server | one |
| **admin** | promoted by owner or another admin, at a terminal | few |
| **member** | anyone who registers with the passphrase | many |

### What an admin can do

Approve evidence proposals. Open and close registration. List profiles by name and id.
Revoke tokens. Delete a profile. See service health, model spend, job history.

### What an admin cannot do

**Read anyone's health record.** Not weight, not food, not training, not documents, not
media, not transcripts. Not through a tool, not through the Mini App, not through MCP.

This is the founding rule and it is enforced structurally, not by policy: admin tools live
in a separate registry that takes no profile argument for reading records, and the read
tools confine to the caller's own profile in one place. An admin listing profiles sees ids
and display names — the same thing `list_profiles` already returns — and nothing else.

**Also cannot:** impersonate a member, export records, read the master key, or promote
themselves.

### Promotion happens at a terminal

```
sehatyctl admin grant <profile-id>
sehatyctl admin revoke <profile-id>
sehatyctl admin list
```

Never through chat, and never by a model. Same reasoning as deleting a record: a person at a
shell, where the blast radius is visible and the confirmation is theirs.

### Upgrade path

Admins approve **evidence** changes. **Code** changes remain a deploy — build, test, ship —
because a system that can rewrite its own behaviour from inside itself has no reviewable
boundary. The two are deliberately different: evidence is data and gets a workflow; code is
code and gets a pipeline.

`schema.sql` plus idempotent `ensureColumn` migrations already handle upgrades in place, and
an upgrade must never require a member to do anything.

---

## 7. Scheduler

Partly built: the `reminder` table and its five storage methods exist. Outstanding: the
ticker, delivery, the tools, and interactive setup through Telegram.

**One row per person per kind**, so nothing stacks into three daily check-ins nobody asked
for. Each row carries its own IANA timezone. Delivery is guarded against double-sending
across restarts, and anything missed by more than three hours is dropped rather than
delivered at midnight.

**Design constrained by evidence, including what NOT to build:**

- **No fatigue backoff.** Three micro-randomised trials found no wear-out — Bidargaddi 2018,
  N=1,255 over 89 days, no attenuation (p=.84). Building decay logic would solve a problem
  that does not exist.
- **Individualised or decreasing frequency beats fixed** (d=0.329). This is the one thing
  with a real effect.
- **Timing carries the effect, not the wording.** Weekend prompts 8.7%; weekday prompts
  statistically null.
- **Expect little.** Choudhry 2017, N=53,480 with an objective outcome, found reminders null
  for every device tested. The scheduler should be modest about what it claims.

---

## 8. Out of scope

- Moving the agent loop to TypeScript. The AI SDK earns its place in the browser if the Mini
  App grows a streaming chat surface; the loop stays beside the data and the refusals.
- A control plane. Sehaty is one agent serving one person on one container.
- Self-calibrated TDEE from observed weight trajectory. **Wanted, and the right eventual
  answer** — a person's energy use is highly repeatable, so two or three weeks of logged
  weight beats any equation for that individual. Deferred, not rejected; `EnergyEstimate`
  already carries `Provisional` to make room for it.
- Per-profile souls. Character is code.

---

## 9. Order of work

1. **Capability layer** — everything else derives from it; deletes `storage.Assessment`
2. **Soul split** — behaviour skills with provenance; no behaviour change, large legibility gain
3. **Programme** — schema, cursor, progression rule, drafting, Mini App approval
4. **Scheduler** — finish the ticker and delivery; interactive setup
5. **Administration** — roles, `sehatyctl admin`, the separate admin registry
6. **Evidence refresh** — depends on 2 and 5

---

## 10. What this design is honest about

Butler et al. 2013, N=1,827: behaviour-change counselling produced **OR 12.44** on recall of
the conversation and **OR 1.12 — null** on actual behaviour change.

A well-built conversational agent will produce excellent engagement, high recalled intent
and high self-reported change. Every proximal signal available from inside the app will look
good. The evidence says that is where it stops.

**So the manner is evidence-based as harm avoidance and unsupported as change technology.**
Warmth is what stops this making things worse — judgement suppresses honest disclosure, and
a record that is not honest is worthless. Nothing here justifies a claim that talking well
produces weight loss.

**Evaluation follows from that.** Do not measure Sehaty on engagement, sentiment, recalled
intent or self-reported behaviour. Measure the things that can embarrass it: logged intake
against a plausibility check, objective activity where obtainable, and retention among the
people most likely to leave.
