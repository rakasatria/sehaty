---
title: Health-app design harms — what the evidence actually supports
date: 2026-09-10
tags: [sehaty, research, evidence, design]
---

# Design harms: three verdicts moved

Commissioned with the brief to contradict as readily as confirm. Three of eight
design decisions moved, and **two of them reversed things already written into the
agent's prompt as settled.**

## Praise — I had this backwards

**Verbal praise ENHANCES intrinsic motivation: d = +0.33.**
**Symbolic tangible rewards — badges, points, streaks — UNDERMINE it: d = −0.34.**

The overjustification argument is real and attaches to *tokens*, not *words*. The
prompt had banned both. Corrected: praise the person for what they did; never award
them anything. Praise for *logging* stays banned separately — that is praise for
operating software, and it teaches nothing.

## Notifications — right conclusion, wrong reason

I believed frequent prompts fatigue people. **Three micro-randomised trials — the only
design that isolates the prompt itself — found no wear-out.**

- Bidargaddi et al. (2018), *JMIR Mhealth Uhealth* 6:e10123. N=1,255, six randomisation
  points/day, **89 days**. Notification raised next-24h engagement 3.9% (RR 1.039,
  CI 1.01–1.08), with **no attenuation across 89 days (p=.84)**.
- Bell et al. (2023), *JMIR Mhealth Uhealth* 11:e38342. Drink Less, 350 users, 30 days.
  Opening within the hour rose **3.5×** (CI 2.91–4.25); no change over time; **no
  difference in time to disengagement** — no retention benefit, no retention harm.
- Morrison et al. (2017), *PLoS ONE* 12:e0169162. N=77. Frequent notifications increased
  exposure "without deterring engagement"; **intelligent tailoring was no better than
  fixed daily**.

The alarm-fatigue analogy is invalid: Drew et al. (2014), *PLoS ONE* 9:e110274 —
2,558,760 alarms, **187 per bed per day, 88.8% false positives**. That is low positive
predictive value at extreme volume, not volume itself.

**The real reason for restraint: prompts are weak-to-null where measured objectively.**
Choudhry et al. (2017), *JAMA Intern Med* 177:624–631 — **N=53,480**, block-randomised,
pharmacy-claims outcome, 12 months: **null for every reminder device** (pillbox OR 1.03,
CI 0.95–1.13). Redfern et al. (2024), *Cochrane* 3:CD011851 — 18 RCTs, 8,136 participants,
could not pool adherence, rated **very uncertain**.

**What IS supported for scheduler design:**
- **Individualised or decreasing frequency beats fixed frequency.** Head et al. (2013),
  *Soc Sci Med* 97:41–48, overall d=0.329 (CI 0.274–0.385).
- **Timing carries most of the effect.** Bidargaddi: weekend 8.7% (CI 1.01–1.17) versus
  weekday 2.5% (CI 0.98–1.07, **null**), peaking ~12:30pm weekends.

JITAI is a framework, not evidence: Hardeman et al. (2019), *IJBNPA* 16:31 — 14 JITAIs,
only 6 randomised, 3–4 weeks, "**no study was sufficiently powered to detect any effects**."

## Self-weighing — weekly default, and it costs less than assumed

Benn et al. (2016), *Health Psychol Rev* 10:187–203. Meta-analysis, 29 tests. **No
association** with affect (r+=.02, N=7,352), body attitudes (r+=−.01, N=5,879) or
disordered eating (r+=.02, N=8,650). Small negative association with psychological
functioning: **r+=−.08 (CI −0.14 to −0.03)**.

The moderators are the story, and they support both camps at once:
- **Age moderated disordered eating: R²=.68, β=.83, p<.01** — more negative in younger samples
- Obesity status moderated body attitudes (r+=.18 overweight/obese vs −.06 n.s. normal-weight)
- **Study design moderated affect (Q=9.53, p=.002)** — RCTs positive, correlational negative

**The adolescent signal is longitudinal, not just cross-sectional.** Neumark-Sztainer et al.
(2006), *J Adolesc Health* 39:811–818 — Project EAT, **5-year, N=2,516**: baseline frequent
self-weighing predicted disordered eating at follow-up in **both female cohorts**, adjusted.
Their explicit recommendation is that obesity-prevention messaging *avoid* prompting
frequent self-weighing.

Pacanowski (2023): **d = 0.84** for negative affective lability against an affect-neutral
active control, in normal-weight women (mean BMI 23.2).

**Two widely-repeated claims that do not hold:**
1. "Weigh daily" rests on Steinberg et al. (2015), *J Acad Nutr Diet* 115:511–518 — 6.1 kg
   greater loss (CI −10.2 to −2.1) — but **N=47 and weighing frequency was self-selected,
   not randomised**. Confounded by motivation. Madigan's subgroup test, the only
   meta-analytic comparison, found **no significant difference between daily and weekly**.
2. Zheng et al. (2015), *Obesity* 23:256–265, the "no adverse outcomes" review, is
   **narrative, unpooled, and restricted to treatment-seeking adults** — the restriction
   people drop when citing it.

**The unresolvable core, honestly:** every "no harm" trial studied overweight adults
volunteering for weight loss; every harm signal comes from adolescents, young adults,
women and normal-weight samples. Design and population are confounded — **except
Pacanowski 2023, which ran the safety camp's design in the harm camp's population and
found harm.**

## Self-monitoring — downgraded

The isolated behaviour is null. **Accountability and the surrounding programme carry the
effect**, not the act of recording. Burke, Wang & Sevick (2011) is the correct citation.

## Not verifiable, do not cite on our authority

Dombrowski 2012 (paywalled); Ho & Intille 2005 (404); the "41% harmful ED content" figure
(advocacy report quoted inside a peer-reviewed paper); Künzler 2019 and Mishra 2021 numeric
results (abstracts only). No primary studies exist on LLM emergency recognition, no
peer-reviewed audit of LLM harm in eating-disorder contexts, and no AI-coach weight-
management trials at all.

See also [[habit-formation]].
