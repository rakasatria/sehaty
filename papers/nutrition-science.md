---
title: Nutrition and exercise science — the activity factor was the real problem
date: 2026-09-10
tags: [sehaty, research, evidence, nutrition, energy]
---

# What was wrong, and it was not what I expected

Six parallel research agents, ~400 retrievals, primary sources read as full text. **The
headline: I had been worrying about the wrong term.** The BMR equation was the smaller of
two extrapolation problems. The activity multiplier is bigger by roughly 3:1, has no
traceable derivation in any population, and its bottom two rungs were physiologically
impossible.

## 1. The activity ladder had no source and an impossible floor

```
1.2 → 1.375 → 1.55 → 1.725 → 1.9
   +0.175   +0.175   +0.175   +0.175
```

Exactly equidistant to three decimals; 1.55 is the exact midpoint. **No empirical
distribution of human activity produces five equally-spaced category means.** It is linear
interpolation between two borrowed anchors, and only 1.2, 1.55 and 1.9 were ever candidate
real numbers.

It is **not** in Harris & Benedict 1918, which contains no activity multipliers at all.

**Both anchors were misapplied.** 1.2 comes from Black et al. (1996), *Eur J Clin Nutr*
50(2):72-92, verbatim: *"A separate analysis of data from **non-ambulant subjects**, and from
elite endurance athletes… established the **limits** of human daily energy expenditure at
around 1.2 × BMR and 4.5 × BMR."* That is the bedbound floor of the species, and it was
being assigned to office workers. And 1.55 is FAO/WHO/UNU's **light** occupational value,
labelled "moderately active, 3-5 days/week."

**The killer number:** Westerterp (2013), *Front Physiol* 4:90 — subjects **confined to a
respiration chamber**, physically unable to leave a sealed room, measure **PAL 1.40 ± 0.06,
never below 1.30.** Free-living: women 1.70 ± 0.23 (n=301), men 1.77 ± 0.28 (n=346). FAO
puts the sedentary floor at 1.40 and the modal adult at 1.60.

**The error it contributes.** Black (2000), *Int J Obes* 24(9):1119-1130: within-subject CV
of estimated BMR **8.5%**; between-subject variation in PAL **15%**. PAL contributes **~76%
of the variance**, the BMR equation ~24%.

Worked, 35 y woman, 70 kg, 165 cm: **one tier = 244 kcal/day; the full span = 977.** A ±10%
BMR error is ±217. **One tier of misclassification exceeds the worst case of the entire
equation.**

**And users cannot self-classify.** Ishikawa-Takata (2008), against DLW: self-reported
"light" and "moderate" were **not significantly different**. Prince (2008) systematic
review: self-report vs direct measurement **r = 0.37**; self-report averaged **44% higher**
than accelerometry, range −78% to +500%; agreement κ = 0.48 in non-obese and **−0.024 in
obese** — worse than chance. Both biases add rather than cancel.

**Implemented:** floor 1.40, then 1.55 / 1.70 / 1.85 / 2.00, unevenly spaced because the
even spacing was the tell. A test now rejects any rung below 1.40.

## 2. Mifflin-St Jeor → Henry/Oxford

Mifflin 1990 (n=498, Reno, Nevada) contains **no SEE, no individual accuracy statistic, no
Bland-Altman, and no race descriptor anywhere.** Its own authors: *"there is a variability of
30% in REE that cannot be explained on the basis of the variables assessed"*, and *"their
clinical utility can only be assessed by testing in other populations."*

**The mechanical signature:** MSJ predicts a distribution ~35% narrower than reality (SD 192
vs 297 in the derivation males). It regresses hard to the mean — which is why it wins on
group bias and loses on individuals, systematically over-predicting low-RMR people and
under-predicting high-RMR people.

**Tropical bias, and the fix.** Henry & Rees (1991): FAO/WHO/UNU **overestimates tropical
BMR by ~8%, up to 11.5%**. Henry (2005): the Schofield database behind it was **3,388 of
7,173 (47%) Italian**, only 13% tropical. Oxford rebuilt on 10,552 values, **excluded all
Italians, raised tropical representation to 38%**. Malaysian adults (Ismail 1998, n=656):
FAO overestimates by 13% (M) and 9% (F).

**The Indonesian mechanism.** Gurrici et al. (1998), *Eur J Clin Nutr* 52(11):779-783 —
Palembang vs Dutch, deuterium dilution: **Indonesians carry 4.8 percentage points more body
fat at the same weight, height, age and sex.** More fat means less fat-free mass, which is
what drives resting expenditure. **A weight/height equation cannot see this.**

**Reported honestly, the evidence also pushes back:** Song (2014), 96 Singaporean Chinese
men — MSJ biased only **+2.4%, 75% within ±10%**, better than Harris-Benedict and FAO. And
Soares (1998): FFM-adjusted BMR did **not** differ between 96 Indians and 81 Australians.

**There is no published validation of any RMR equation in healthy Indonesian adults.** The
only Indonesian comparison found uses n=12 adolescent basketball players measured by BIA.

**Implemented:** Henry/Oxford, weight-based, by sex and age band. Even its own SE is ~156
kcal/day for young men — **±310 for 95% coverage is the irreducible floor of any
weight-based equation.**

## 3. "95% within ±20%" was unsupported

Best published limits of agreement, in the most favourable study (Thom 2020, 125 UK women):
**±21.1%.** Typical: −24.8% to +24.7% (severe obesity), ±29.8% (over-65 with obesity),
−37.0% to +31.1% (Tehran).

**And the band is offset, not centred.** Kfir 2023, **n=3,001**: bias **−12.6%**, LoA **+5.8%
to −26.5%**. A symmetric envelope does not contain the truth for a large share of people.

The "70% within ±10%" figure is the Academy of Nutrition and Dietetics number for **obese**
individuals specifically, with asymmetric failure (21% under vs 9% over). Published range
across populations: **17% to 87%** — 52.2% in athletes, 43.1% in over-65s with obesity,
17.3% in Chinese nursing-home residents.

**Implemented:** ±25%, framed as a starting guess pending replacement.

## 4. Protein 1.6 g/kg is softer than its reputation

Morton (2018), *Br J Sports Med* 52(6):376-384, 49 RCTs, n=1,863. **The breakpoint of 1.62
g/kg had p = 0.079 — it failed its own significance test**, CI 1.03 to 2.20. The authors
write that *"it may be prudent to recommend ~2.2 g protein/kg/d"*. Total FFM effect of
supplementation across whole trials: **+0.30 kg**.

**In a deficit, 1.6 is clearly too low.** Longland (2016), *Am J Clin Nutr* 103(3):738-746,
40 men at a ~40% deficit, 4-compartment body composition: **2.4 g/kg gained 1.2 ± 1.0 kg
LBM; 1.2 g/kg gained 0.1 ± 1.0 kg.** Helms (2014): 2.3-3.1 g/kg FFM for lean, restricted,
resistance-trained athletes.

**COI:** senior author declares US National Dairy Council support, *"This agency has
supported trials reviewed in this analysis."*

**Implemented:** 1.6 g/kg baseline, **2.2 g/kg when the goal is fat loss**.

## 5. The deficit rule was right, for a reason worth recording

Garthe (2011), n=24 elite athletes, DXA — the empirical base for the whole heuristic, and it
is **n=13** in the arm that matters:

| | slow | fast |
|---|---|---|
| deficit | **−469 kcal/d (19%)** | −791 (30%) |
| protein | **1.6 g/kg** | 1.4 |
| **LBM** | **+1.0 kg** | −0.3 kg |

Murphy & Koehler (2022) put the ceiling at **~500 kcal/day** — an *absolute* threshold that
diverges from a percentage above ~2,500 kcal maintenance. Their cohort was sedentary,
untrained, mean age 51-60, **protein not modelled at all**.

**Already implemented:** `min(20% of TDEE, 500 kcal)`. Correct by luck rather than judgement
— it was chosen before this evidence arrived.

**Two corrections worth carrying:** the famous "0.7% vs 1.4% bodyweight/week" contrast never
happened — those were *targets*; achieved rates were 0.7 ± 0.4 vs **1.0 ± 0.4**. And
Garthe's squat improved **equally in both arms**; the slow group's apparent strength
advantage is a duration artefact (8.5 vs 5.3 weeks).

## 6. Self-reported intake

Anchor on **Freedman (2014)**, *Am J Epidemiol* 180(2):172-188 — five validation studies with
recovery biomarkers, **n=2,265**: under-reporting **15% on a 24-hour recall, 28% on an FFQ**.
Correlation of reported to *true* energy: **0.26** for a single recall.

Not Lichtman 1992 (the famous 47%) — that is **n=10**, selected for a history of diet
resistance, with an SD larger than the mean on its exercise figure.

**BMI predicts under-reporting.** EPIC (n=35,955): OR highest vs lowest BMI quartile **3.52
men, 4.80 women**. And **under-reporting rises the moment someone starts dieting** — 22.9%
general vs **38.8% among self-reported dieters** (n=18,150 NHANES).

**The finding that should shape the product:** Goris (2000) decomposed 37% under-reporting
into **26% genuine undereating and 12% under-recording**. Two-thirds of the "error" was
people actually eating less because they were logging. **A log is partly a measurement and
partly an intervention.**

**A BMI-based plausibility screen will under-detect Indonesian under-reporters** — Japanese
under-reporters showed no BMI difference but significantly higher body fat, and Asian
populations carry more fat at a given BMI. Screen on body fat or waist if possible.

**What a logged total can legitimately do** (Subar 2015): *"do not use self-reported energy
intake as a measure of true energy intake"* but *"do use it for energy adjustment of other
self-reported constituents."* **Ratios and densities survive; absolute totals do not.**

## 7. TKPI — better than feared for our purpose

Measured from the official Kemenkes PDF, fraction of imputed/borrowed cells per column:

| column | flagged | column | flagged |
|---|---|---|---|
| **Energi** | **0.1%** | Natrium | 23.1% |
| **Protein** | **0.1%** | Kalium | 28.4% |
| **Lemak** | **0.2%** | Niasin | 31.7% |
| **Karbohidrat** | **1.4%** | β-karoten | 23.6% |

**Energy and macronutrients are 97-99% Indonesian analytical measurements.** The borrowing
problem is a micronutrient problem, and Sehaty is a calorie-and-macro app.

**Real risks that remain:**
- **Composite dishes were not updated between 2009 and 2020** and trace to a single 1993
  source. A nasi padang figure is one analysis of one recipe as prepared then.
- **No retention or yield factors** — raw-to-cooked conversion has no Indonesian factor set.
- **Fibre mixes crude and dietary** and TKPI says you cannot tell which is which.
- **Carbohydrate is by difference**, absorbing every other column's error.
- **panganku.org publishes TKPI 2017** — the edition containing papaya at 12 g fat/100 g and
  a straight energy transposition between two guavas. The 2020 errata must be applied.

**Table choice moves the answer by a third:** NutriSurvey-Indonesia vs TKPI 2017 — 453 items
differ, NutriSurvey averaging **34.3% higher**. Nasi: 130.0 vs 179.7 kcal/100 g.

## 8. Programming — mostly asserted, not established

- **"10-20 sets/week"**: the three-level model in Schoenfeld 2017 was **p = 0.074, never
  significant**. The "12-20" figure rests on 6 studies, all young trained men.
- **"Train each muscle 2×/week"**: the 2016 meta-analysis states its volume-matched analysis
  *"could not be carried out… due to inadequate sample size"* — **and drew the conclusion
  anyway.** Schoenfeld 2019 (25 studies): frequency does not meaningfully affect hypertrophy
  when volume is equated.
- **"Progressive overload is fundamental"**: **it has never been isolated as an experimental
  variable.** No trial compares progressive against non-progressive with everything else
  equated.
- **The famous "−2% to +59% individual response"** used the contralateral untrained arm as
  control, and cross-education is well documented. It is an upper bound. Best-controlled
  estimate with real non-training controls: 29% low responders for size, **7% for strength**.
- Two prominent "high volume is counterproductive" papers (Barbalho 2019, 2020) are
  **RETRACTED**.
- **Strength is load-specific** — SMD 0.60 (0.38-0.82), SUCRA 98.2%. The most solid finding
  in the literature.
- **Ethnicity is not reported in ANY major meta-analysis.** Not evidence of no difference;
  evidence that nobody looked.

**The defensible summary** (Currier 2023, 178 studies, n=5,097): all twelve prescriptions
beat control; higher load maximises strength; ***"all prescriptions comparably promoted
muscle hypertrophy."*** **Adherence is the variable with the large effect size. Almost
everything an app can tune is a rounding error against showing up.**

## Still to do

- Verify our `tkpi-2020.json` is the 2020 edition **with errata applied**, not 2017.
- Composite dishes: consider refusing them entirely rather than serving 1993 figures.
- Self-calibrated TDEE from weight trajectory — Sanghvi (2015) reached **215 kcal/day
  individual error using no diet data at all**, over two years. Nothing static comes close.

See also [[energy-estimation]], [[habit-formation]], [[design-harms]], [[therapeutic-manner]].
