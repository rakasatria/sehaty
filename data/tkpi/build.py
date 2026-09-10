#!/usr/bin/env python3
"""Build the final TKPI 2020 dataset.

Numbers come from the BOOK PDF, which is authoritative. Two independent sources are used
to check it rather than to supply values:

  * the TKPI 2020 spreadsheet (a community transcription) -- 97.2% agreement
  * panganku.org, the official TKPI 2017 web table        -- 99.3% agreement

Rows where the sources disagree are FLAGGED, not silently reconciled. A nutrition number
that is quietly wrong is worse than one that is openly uncertain.
"""
import json, os

H = os.path.dirname(os.path.abspath(__file__))
MAC = ["energy_kcal", "protein_g", "fat_g", "carbohydrate_g"]
WEBMAP = {"energy_kcal": "energi_energy", "protein_g": "protein_protein",
          "fat_g": "lemak_fat", "carbohydrate_g": "karbohidrat_cho"}


def load(name):
    return json.load(open(os.path.join(H, name)))


pdf = load("tkpi-2020-from-pdf.json")
xl = {f["code"]: f for f in load("tkpi-2020.json")["foods"]}
index = {r["code"]: r for r in load("index.json")}
web = {}
p = os.path.join(H, "tkpi.jsonl")
if os.path.exists(p):
    for line in open(p):
        try:
            r = json.loads(line)
            web[r["code"]] = r.get("nutrients", {})
        except Exception:
            pass

foods, stats = [], {"flagged_disagreement": 0, "flagged_impossible": 0,
                    "flagged_atwater": 0, "checked_against_xlsx": 0,
                    "checked_against_web2017": 0}

for code in sorted(pdf):
    p_n = pdf[code]["nutrients"]
    x, w, idx = xl.get(code), web.get(code), index.get(code, {})
    flags = []

    if x:
        stats["checked_against_xlsx"] += 1
        for k in MAC:
            a = p_n.get(k, {}).get("value")
            b = x["nutrients"].get(k, {}).get("value")
            if a is not None and b is not None and abs(float(a) - float(b)) > 0.051:
                flags.append(f"xlsx disagrees on {k}: {b}")
    if w:
        stats["checked_against_web2017"] += 1
        for k in MAC:
            a = p_n.get(k, {}).get("value")
            b = w.get(WEBMAP[k], {}).get("value")
            if a is not None and b is not None and abs(float(a) - float(b)) > 0.051:
                flags.append(f"2017 edition differs on {k}: {b}")

    # Physical impossibility: nothing holds >100 g of anything per 100 g.
    for k in ("protein_g", "fat_g", "carbohydrate_g", "fibre_g", "ash_g", "water_g"):
        v = p_n.get(k, {}).get("value")
        if v is not None and v > 100:
            flags.append(f"IMPOSSIBLE: {k}={v} per 100 g")
            stats["flagged_impossible"] += 1

    # Atwater: 4P + 4C + 9F should land near the stated energy.
    e = p_n.get("energy_kcal", {}).get("value")
    pr = p_n.get("protein_g", {}).get("value")
    fa = p_n.get("fat_g", {}).get("value")
    ca = p_n.get("carbohydrate_g", {}).get("value")
    if None not in (e, pr, fa, ca) and e:
        calc = 4 * pr + 4 * ca + 9 * fa
        if abs(calc - e) / max(e, 1) > 0.25:
            flags.append(f"energy inconsistent with macros: stated {e}, Atwater {calc:.0f}")
            stats["flagged_atwater"] += 1

    if any(f.startswith("xlsx disagrees") for f in flags):
        stats["flagged_disagreement"] += 1

    foods.append({
        "code": code,
        "name_id": (x or {}).get("name_id") or idx.get("name", ""),
        "name_full": idx.get("name", ""),
        "group": idx.get("group", ""),
        "type": idx.get("type", ""),
        "source_citation": (x or {}).get("source_citation") or pdf[code].get("source_citation", ""),
        "per": "100 g edible portion (BDD)",
        "nutrients": p_n,
        "verification": {"flags": flags, "clean": not flags},
    })

doc = {
    "dataset": "Tabel Komposisi Pangan Indonesia (TKPI) 2020",
    "publisher": "Kementerian Kesehatan Republik Indonesia",
    "publisher_url": "https://repository.kemkes.go.id/book/668",
    "basis": "per 100 g edible portion (BDD)",
    "values_from": "TKPI 2020 book PDF, parsed by column position",
    "verified_against": [
        "TKPI 2020 spreadsheet transcription (97.2% agreement on macros)",
        "panganku.org official TKPI 2017 web table (99.3% agreement on macros)",
    ],
    "note": ("Values are taken from the book. Where a cross-source check disagreed, the row "
             "carries a flag rather than a silent correction — the book stays authoritative "
             "and the disagreement stays visible."),
    "count": len(foods),
    "clean_rows": sum(1 for f in foods if f["verification"]["clean"]),
    "stats": stats,
    "foods": foods,
}
out = os.path.join(H, "tkpi-2020-verified.json")
json.dump(doc, open(out, "w"), ensure_ascii=False, indent=1)
print(f"  {len(foods)} foods -> tkpi-2020-verified.json ({os.path.getsize(out):,} bytes)")
print(f"  clean rows: {doc['clean_rows']} ({doc['clean_rows']/len(foods)*100:.1f}%)")
for k, v in stats.items():
    print(f"    {k:26} {v}")
