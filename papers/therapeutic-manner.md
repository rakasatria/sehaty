---
title: Therapeutic manner — evidence-based as harm avoidance, not as change technology
date: 2026-09-10
tags: [sehaty, research, evidence, motivational-interviewing, sdt]
---

# Manner: what it can and cannot do

The most consequential review of the set. It **corrects two things** that were built into
the agent on my say-so, and it names the central risk of the entire product.

## The central risk, measured

**Butler et al. (2013), *BMJ* 346:f1191.** 53 GPs and nurses, 27 Welsh practices,
**1,827 patients**. Behaviour Change Counselling, developed from MI by Rollnick, who
co-authored the trial.

| outcome | result |
|---|---|
| recall of discussing the behaviour | **OR 12.44** (5.85–26.46) |
| intention to change | **OR 2.88** (2.05–4.05) |
| self-reported sustained change, 3mo | OR 1.36 |
| **actual beneficial behaviour change** | **OR 1.12 (0.90–1.39) — null** |
| biometrics at 12 months | nothing |

Authors: *"Enduring behaviour change and improvements in biometric measures are unlikely
after a single routine consultation with a clinician trained in behaviour change
counselling without additional intervention."*

Corroborated in our exact domain — **Braun et al. (2026)**, *Health Educ Behav*: dietitian
reflections produced participant change talk at **OR 7.55 (6.20–9.18)**, and *"use of MI
was not associated with changes in diet or ambivalence."* Complex reflections correlated
**negatively** with change in ambivalence (ρ = −.31).

**The consequence is an evaluation discipline, not a design change.** A good conversational
agent will produce excellent engagement, sentiment, recalled intent and self-reported
behaviour. Butler shows all four can look spectacular beside a null primary outcome.
**Instrument only the things that can embarrass us**: logged intake against a plausibility
check, objective activity where obtainable, and retention among the people most likely to
leave.

## The mechanism has collapsed

Three independent process meta-analyses agree, and they disconfirm the theory:

- **Magill et al. (2014)**, *JCCP* 82(6):973–983, N=1,004 — change talk → outcome
  **r = .06 (CI −.09 to .21), null**. Sustain talk → outcome **r = −.24**.
- **Magill et al. (2018)**, *JCCP* 86(2):140–157, N=3,025 — MI-consistent skill produces
  more change talk (r=.55) **and** more sustain talk (r=.40). Doing it well produces more
  of both sides. **"The relational hypothesis was NOT supported."**
- **Pace et al. (2017)**, *Psych Addict Behav* 31(5):524–533, N=2,614 — *"Therapist global
  ratings were not significantly related to clinical outcomes."*

Miller himself (2023, *Behav Cogn Psychother* 51(6):616–632): *"When directly compared,
sustain talk tends to be a better predictor of outcome than change talk alone."*

**So the defensible rule is negative: do not trigger the righting reflex.** Not: evoke
change talk.

## Implemented: the righting reflex, operationally

MI-nonadherent behaviours, all negatively correlated with outcome: **disagreeing, arguing,
correcting, shaming, criticising, providing unsolicited advice.** And *"arguing against
sustain talk tends to entrench it."*

**Decisional balance is repudiated by its own founder:** *"Experimental trials show that
doing a decisional balance intervention with people who are ambivalent tends to decrease
their commitment to change."* Pros-and-cons with an undecided person is counterproductive.

**Discord vs sustain talk:** discord is about the relationship and *"often contains the
word 'you'"* — "you're not listening", "you can't tell me what to do". Signal to stop
pushing, not to explain better.

All of the above is now in `internal/agent/agent.go`, placed **ahead** of the positive
guidance, because it is better evidenced.

## Corrections to what I previously reported

**Rubak 2005 is routinely miscited, including by me.** BMI **0.72 (CI 0.33–1.11)** is
0.72 **BMI units** — a raw weighted mean difference — **not d = 0.72**, and it rests on
**6 RCTs**. DARE's appraisal: single-reviewer selection, **no heterogeneity assessment**,
"not possible to judge the robustness of the authors' conclusions."

**Hypnotherapy: I declined it for the wrong reason.** I cited safety. **The safety data are
reassuring; the efficacy data are what fail.** Functional Imagery Training is the
imagery-based, non-clinical alternative with the best weight-loss trial found across eight
topics — worth considering on its merits rather than dismissed on mine.

## Cochrane reversals worth knowing

- **MI for smoking** — Lindson et al. (2019), CD006936.pub4, 37 studies, >15,000
  participants. MI vs no treatment **RR 0.84 (0.63–1.12)**, point estimate favouring
  control. *"Insufficient evidence to show whether or not MI helps people to stop
  smoking."* The 2015 version reported a significant RR 1.26. Tightened criteria plus
  GRADE turned it null.
- **MI for alcohol in young adults** — Foxcroft et al. (2016), CD007025.pub4, **84 trials,
  22,872 participants**. *"No substantive, meaningful benefits."* No dose–response.
- **A harm signal** — Holden et al. (2024), *Braz J Phys Ther* 28(4):101091. 46 adults,
  sub-acute low back pain. Primary outcome null; MI arm **worse on every secondary
  outcome**: disability MD 19.4 (8.5–30.3), self-efficacy MD −11.3 (−20.2 to −2.5).

**Frost et al. (2018)**, *PLoS ONE* 13(10):e0204890 — umbrella review, 104 reviews, 155
meta-analytic comparisons. **128 of 155 (83%) graded LOW or VERY LOW.** Only **7%** yielded
moderate-quality evidence of small, mostly short-term benefit. The authors' own term for
the field: **"research waste."**

## SDT: competence carries it, and the engine does not predict

Ng et al. (2012), 184 datasets. Needs → autonomous motivation: **competence β = .35,
autonomy β = .13**, relatedness β = .15. The construct the theory is named for contributes
.13.

Ntoumanis et al. (2021), full Table 1: physical health at end of intervention
**g = 0.042 (CI −0.151 to 0.234), p = .67, null.** I² of 89–99%. **Egger's test significant
for the two outcomes the positive conclusion rests on.** Adequate allocation concealment
*reduced* effect sizes; quasi-experimental trials produced larger effects than RCTs. In the
prospective mediation analysis, **none of the needs predicted health-behaviour effect sizes
at follow-up.**

**Mathiesen et al. (2023)**, *Syst Rev* 12(1):158 — 11 RCTs, **6,059 participants**, trial
sequential analysis. **No effect on quality of life, mortality, serious adverse events,
diabetes distress, depression, or HbA1c.** Benefit only on the theory's own questionnaires.

**What to build to instead** is older and better: **Deci, Eghrari, Patrick & Leone (1994)**,
*J Personality* 62(1):119–142 — an actual experiment. Three things promote internalisation:
**a meaningful rationale, acknowledging the person's feelings, and conveying choice.** Now
implemented.

Note the perverse finding in Ntoumanis's meta-regression: several *competence-support*
techniques (identify barriers, problem-solving, plans appropriate to ability) yielded
**smaller** psychological-health effects, and "being positive that a person can succeed" was
**fully confounded with non-specific rewards**.

## What this justifies

Warmth is **harm avoidance**, and harm is documented: judgement suppresses honest
disclosure; stigmatising language is both more harmful and less motivating; discrimination
predicts weight gain; technique-without-warmth is iatrogenic. A record that is not honest is
worthless, so the manner protects the data.

**It does not justify any claim that talking well produces weight loss.** It does not for
trained clinicians in properly powered trials, and there is no reason to expect an agent to
do better.

## Claims retired

- That the agent delivers MI
- That warmth prevents the "what-the-hell" spiral
- That hypnotherapy was declined on safety grounds

See also [[habit-formation]], [[design-harms]].
