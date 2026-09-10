---
title: Prompt provenance — every rule, and where it came from
date: 2026-09-10
tags: [sehaty, evidence, audit]
---

# Provenance audit

Every claim in `SystemPrompt`, tagged. **PAPER** cites literature. **RULE** is Raka's
standing instruction. **DATA** follows from the dataset. **JUDGEMENT** is mine and
unsourced — which is the category that matters, because four such claims were stated
confidently today and all four were reversed on checking.

Nothing may enter the soul unsourced without carrying this tag.

## Sourced to papers

| rule | source |
|---|---|
| Praise the person, never award tokens | praise d=+0.33; symbolic tangible rewards d=−0.34 |
| A missed day does not derail a habit | Lally et al. 2010 — automaticity barely moved, recovered quickly |
| Protect self-efficacy, not mood, after a gap | Kirchner, Shiffman & Wileyto 2012 — 1,001 lapses; guilt did not predict relapse, self-efficacy collapse did |
| Favour consistency; intensity costs adherence | Burnet et al. 2020 — intensity −3.3% (CI −6.1 to −0.5); frequency n.s. |
| Never disagree, argue, correct, shame, criticise, or give unasked advice | MI-nonadherent behaviours, all negatively correlated with outcome |
| Arguing against a defence entrenches it | Miller 2023 — "arguing against sustain talk tends to entrench it" |
| No pros-and-cons with an undecided person | Miller 2023 — decisional balance *decreases* commitment in the ambivalent |
| Talk about *you* means stop pushing | Miller 2023 — discord vs sustain talk; "often contains the word 'you'" |
| Give a reason, acknowledge feelings, offer choice | Deci, Eghrari, Patrick & Leone 1994 — the three that promote internalisation |
| Make them feel capable first | Ng et al. 2012 — competence β=.35 vs autonomy β=.13 |
| Give real choices, not softened wording | Smit et al. 2019 — language null; actual choice worked |
| Non-judgement protects the record | judgement suppresses honest disclosure; stigma both more harmful and less motivating |
| Mifflin-St Jeor, ×PAL | Mifflin et al. 1990 |
| Protein 1.6 g/kg | Morton et al. 2018 — **note disclosed dairy-industry funding supporting trials inside the analysis** |

## Sourced to Raka's standing rules

| rule | origin |
|---|---|
| Never invent a number | the founding constraint of the whole system |
| Data must never train a model | `provider.data_collection = "deny"` |
| Short, no emoji, no exclamation | stated preference |
| Reply in the language they wrote in | stated preference |
| Expert in nutrition and exercise science | stated today |
| May estimate energy, may not prescribe it | stated today, reversing the earlier absolute |
| The assessment happens at first contact | stated today |

## Follows from the data

| rule | why |
|---|---|
| Resolve food against TKPI before logging | the table is the only source of composition |
| Composite dishes are not in the table | TKPI lists ingredients |
| Flagged entries are uncertain | 83 of 1,142 carry a verification flag from source disagreement |

## JUDGEMENT — mine, unsourced, and therefore suspect

These are the ones to be uneasy about. Each is plausible. So were the four that turned out
to be wrong.

| claim | status |
|---|---|
| **"A plate of nasi goreng spans 250–700 kcal"** | **I asserted this range. I have never seen a source for it.** It is used to justify refusing to guess a portion — the refusal is right, the number needs checking or removing. |
| **"±10% covers roughly 70% of individuals"** | **Currently shown to the user.** Being verified now; the previous researcher ran out of budget before closing it. If it is wrong, the app is displaying a false confidence band. |
| No supplements, no crash diets, never train through pain | Taken from the ai-fitness-coach reference repo, not from literature. Almost certainly correct and entirely unverified by me. |
| One question per message | Reasonable. Untested. |
| Never open with a questionnaire | Reasonable, and now partly overridden by the consultation design anyway. |
| Admitting uncertainty builds trust | Asserted in MANNER. The manner researcher was asked to check it; not yet confirmed. |
| Treat a return as continuation, never resumption | Follows from the self-efficacy finding, but the specific behaviour — saying nothing about the gap — is my inference, not a tested intervention. |
| Warmth should be unconditional | Judgement. Defensible via the stigma literature; not directly tested. |

## What the audit is for

The evidence says something uncomfortable about all of this: **manner is well-evidenced as
harm-avoidance and unsupported as change technology.** Butler et al. 2013, N=1,827 —
recall of the conversation OR 12.44, actual behaviour change OR 1.12, null.

So the point of this table is not to make the prompt more persuasive. It is to make sure
that when it is wrong, it is wrong in a way somebody can see.

See also [[habit-formation]], [[design-harms]], [[therapeutic-manner]].
