# papers

The evidence behind Sehaty's design decisions, and — more usefully — the evidence
against them.

Each file was commissioned with the same brief: **report contradicting evidence as
readily as supporting evidence, and say plainly when a claim is popular but weakly
supported.** That instruction earns its keep. The first review overturned three
things that were already written into the agent's system prompt as though they were
settled.

## Why this lives in the repo rather than a wiki

Because the claims are in the code. `internal/agent/agent.go` tells a language model
that a missed day does not derail a habit, that competence matters more than
autonomy, that intensity costs adherence where frequency does not. Those are
empirical assertions being made to a person about their own health, and they should
be checkable by whoever reads the code next — including whoever decides to change
them.

When a decision here contradicts a paper, the paper wins or the decision gets a
written reason. When neither is true, it is an assumption, and it should say so.

## Files

| file | covers |
|---|---|
| `habit-formation.md` | time to automaticity, consistency vs intensity, the lapse and the return, implementation intentions, autonomy-supportive language, cue-based design |

## The standing rule

Nothing in here is cited in what a user sees. Sehaty does not quote studies at
people, does not say "research shows", and does not use evidence as a rhetorical
device. The papers decide what the software does; the software just does it.
