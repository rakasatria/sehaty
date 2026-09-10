# TKPI 2020 — Indonesian food composition

`tkpi-2020.json` — 1,142 foods from the **Tabel Komposisi Pangan Indonesia 2020**,
published by Kementerian Kesehatan Republik Indonesia.

**Source:** <https://repository.kemkes.go.id/book/668>
**Basis:** per 100 g edible portion (BDD). `edible_portion_pct` gives the edible fraction.
**Attribution:** all rights in the underlying table belong to Kemenkes RI. This is a
machine-readable rendering of a published government reference table, retained with its
per-row `source_citation` intact so any value can be traced to the original analysis.

## Why this exists

International food databases do not cover Indonesian food. Measured, not assumed:
USDA FoodData Central returns Western seasoning mixes for *nasi goreng*; Open Food Facts
has 4,693 Indonesian **packaged** products and zero home-cooked dishes. TKPI is the only
reference that knows what tempe, tahu and ikan bakar actually contain.

## How it was built, and why you can trust the numbers

Values come from the **book PDF**, parsed by **column position** rather than by counting
tokens. That distinction matters: many rows have blank cells mid-row (a missing RETINOL,
say), and token counting silently shifts every value after the gap into the wrong nutrient.

Two independent sources were used to **check** the parse, never to supply values:

| cross-check | agreement on macros |
|---|---|
| TKPI 2020 spreadsheet transcription | 97.2% |
| panganku.org — official TKPI **2017** web table | 99.3% |

The 2017 edition differs slightly from 2020 by design, so less than 100% is expected there.

**The spreadsheet is not the source, and should not be used as one.** It is a community
transcription whose own header warns values may have drifted during editing. Checked against
the book it has **19 physically impossible rows** (a macro above 100 g per 100 g) against the
book's 1, and roughly 3.5% of rows carry a wrong macro — several from whole-row column
shifts. `CP082 Tempe pasar` is the clearest: the spreadsheet reports 517 g of carbohydrate,
which is really the **calcium** value in mg; the true carbohydrate is 9.1 g.

## Verification flags

Every food carries a `verification` block. **1,059 of 1,142 rows (92.7%) are clean.**
The rest are flagged rather than corrected — the book stays authoritative and the
disagreement stays visible:

| flag | rows | meaning |
|---|---|---|
| `xlsx disagrees on …` | 39 | the spreadsheet transcription differs — usually the spreadsheet is wrong |
| `2017 edition differs on …` | — | genuine revision between editions |
| `IMPOSSIBLE: …` | 1 | a macro above 100 g per 100 g; `CP070` ash, present in every source, so likely a typo in the book itself |
| `energy inconsistent with macros` | 40 | 4P + 4C + 9F is more than 25% from the stated energy |

**Consuming code should treat a flagged row as uncertain** — surface the flag rather than
presenting the number as measured fact.

## What it does not cover

TKPI is a **composition** table of ingredients, not a dish table. It has tempe, tahu, beras
and ikan in depth, but exactly **one** beverage (coconut water) and no composite warung
dishes. *Nasi goreng* and *gado-gado* must be composed from ingredients or curated
separately with their own citations.

## Reproducing

```bash
python3 parse_pdf.py   # book PDF (pdftotext -layout) -> raw values, by column position
python3 convert.py     # spreadsheet -> JSON, for cross-checking only
python3 extract.py     # panganku.org 2017 web table, 1 req/sec, resumable
python3 build.py       # merge, verify, flag -> tkpi-2020.json
```
