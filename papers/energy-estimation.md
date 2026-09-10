---
title: Energy estimation — the confidence band was wrong
date: 2026-09-10
tags: [sehaty, research, evidence, nutrition]
---

# Mifflin-St Jeor: what it can and cannot tell an Indonesian

Sehaty showed **±10%, "about 70% of people"**. That was wrong for what the app does, and
wrong for who uses it. It now shows **±25%**, framed as a starting prior awaiting
calibration.

## Why ±10% was wrong twice over

**It describes the wrong quantity.** 70–82% within ±10% is Frankenfield's figure for
**measured RMR** in US community adults (2003, n=130: 78%; 2013, n=337: 82%, but **87%
non-obese vs 75% obese**). Sehaty estimates **TDEE** — a predicted BMR multiplied by a
guessed activity factor. RMR error SD ≈10%, PAL misclassification by one FAO category
≈12%. Compounded, total SD ≈12–13%, so **±10% captures roughly 55–60%**, and 95% coverage
needs about **±25%**.

Mifflin 1990 itself (n=498 US adults) had **R² = 0.71** — 29% of between-person variance is
unexplained by weight, height, age and sex *by construction*. Validation RMSEs run
**155–314 kcal/day**.

**And it describes the wrong people.** The 2005 ADA review that popularised MSJ carried its
own warning: "older adults and US-residing ethnic minorities were underrepresented both in
the development of predictive equations and in validation studies."

## There is no Indonesian validation. At all.

Searched several ways in Europe PMC. **A genuine void, not a search failure.** The nearest
evidence:

| population | n | MSJ within ±10% |
|---|---|---|
| US community adults (Frankenfield 2013) | 337 | 82% |
| **Korea** (Ndahimana 2018) | 109 | **69%** |
| **China** (Xue 2019) — range across equations | 315 | **17.5–59.1%** |
| Athletes (O'Neill 2023 meta, n=1430) | — | **52.2%** |
| Severe obesity (Utah, n=780) — best of 11 equations | — | **≤67.8%** |

**The mechanism, which is why this cannot be patched.** Wouters-Adriaens & Westerterp 2008:
measured REE in Asians vs whites was **5.87 vs 7.00 MJ/d (~1400 vs ~1670 kcal)** — a 16%
gap that **disappeared entirely after adjusting for fat-free mass**. Asians carry more fat
and less FFM at the same BMI. An equation taking only weight, height, age and sex cannot
see body composition, so it inherits the difference structurally.

And the **direction of bias is inconsistent** across Asian cohorts — Chinese studies mostly
under-predict, Singapore over-predicts in the underweight, Korea shows a small negative
bias. So there is no constant to correct by.

Worth knowing: the FAO/WHO/UNU alternative is no escape. Henry 2005 found the Schofield
database behind it was **3,388 of 7,173 subjects (47%) Italian**, with very few from the
tropics.

## Switching equations does not rescue it

For general adults the spread between equations is small next to the error they share — in
Korea, best to worst of seven established equations was 70% vs 61%. **Cunningham and
Katch-McArdle are worse, not better**, without DXA-grade fat-free mass (Katch-McArdle was
*last* in the Korean data at 61%). Ten-Haaf is meaningfully better **for athletes only**
(80.2% vs MSJ's 52.2%).

MSJ remains a reasonable default. It just cannot carry a ±10% band.

## Metabolic adaptation is smaller than feared

Not the Biggest Loser story. In ordinary dieting, measured RMR runs **50–100 kcal/day
(≈3–5%) below prediction**, SD ≈110 kcal/d, and it **largely disappears after weight
stabilises**. Nunes 2022 (33 studies, n=2,528): present in 27/33, but "values may be small
or non-statistically significant when higher-quality methodological designs are used."

The larger effect is **equation drift**: Dahle 2021 found all four common equations shift
toward **over-prediction within the first month** of weight loss and stay there, because
they cannot track changed body composition.

## The fix, and it is cheap

**TEE is highly repeatable within a person** over time (IAEA DLW database, n=348) even
though it varies enormously between people. So after two to three weeks of logged weight, a
TDEE back-calculated from the observed trajectory **beats any equation for that individual
by a wide margin**.

That reframes the equation entirely: it is a **seed**, not a measurement. `EnergyEstimate`
now carries `Provisional: true` so a future self-calibrated figure can say it is not.

## What Sehaty now says

±25%, and the caveat names the reason: built on 498 American adults, never tested on
Indonesians, and destined to be replaced by what the weight log actually does.

Tests assert the caveat keeps saying all of that, and that the band stays ~50% wide.

See also [[habit-formation]], [[design-harms]], [[therapeutic-manner]], [[provenance]].
