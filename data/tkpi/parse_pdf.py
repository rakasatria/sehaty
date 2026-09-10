#!/usr/bin/env python3
"""Parse TKPI 2020 from the book PDF by COLUMN POSITION (first occurrence wins).

Token counting fails on this table: many rows have blank cells mid-row (a missing
RETINOL, say), so counting tokens silently shifts every value after the gap into the
wrong nutrient. pdftotext -layout preserves column geometry, so each page's unit header
row -- (g) (Kal) (g) ... -- gives the true column boundaries, and a blank cell stays blank.
"""
import json, os, re, sys

HERE = os.path.dirname(os.path.abspath(__file__))
TXT = os.path.join(HERE, "tkpi2020.txt")

KEYS = ["water_g","energy_kcal","protein_g","fat_g","carbohydrate_g","fibre_g","ash_g",
        "calcium_mg","phosphorus_mg","iron_mg","sodium_mg","potassium_mg","copper_mg",
        "zinc_mg","retinol_mcg","beta_carotene_mcg","carotene_total_mcg","thiamin_mg",
        "riboflavin_mg","niacin_mg","vitamin_c_mg","edible_portion_pct"]
UNITS = ["g","kcal","g","g","g","g","g","mg","mg","mg","mg","mg","mg","mg",
         "mcg","mcg","mcg","mg","mg","mg","mg","%"]

CODE = re.compile(r"^\s*([A-Z]{2}\d{3,4})\b")
NUMTOK = re.compile(r"(-|\d+(?:[.,]\d+)?)")


def unit_columns(line):
    """Start offsets of each (g)/(Kal)/(mg)/(mcg) token on a units header line."""
    return [m.start() for m in re.finditer(r"\((?:g|Kal|mg|mcg|%)\)", line)]


def parse():
    pages = open(TXT, encoding="utf-8", errors="replace").read().split("\f")
    foods, cols = {}, None
    pending_name = []
    for page in pages:
        lines = page.split("\n")
        # A page that restates the units header re-establishes the geometry.
        for ln in lines:
            if "(Kal)" in ln:
                c = unit_columns(ln)
                if len(c) >= 21:
                    cols = c
                break
        if not cols:
            continue
        # Column boundaries: midpoint between adjacent unit-token starts.
        bounds = []
        for i, c in enumerate(cols):
            lo = cols[i-1] + (c - cols[i-1]) // 2 if i else c - 6
            hi = c + (cols[i+1] - c) // 2 if i + 1 < len(cols) else c + 14
            bounds.append((lo, hi))

        for ln in lines:
            m = CODE.match(ln)
            if not m:
                t = ln.strip()
                # Food names sit on their own line(s) just above the data row.
                if t and not re.search(r"\(Kal\)|KODE|BDD|TUNGGAL|KOMPOSISI|^\d+\s*$", t):
                    pending_name.append(t)
                    pending_name[:] = pending_name[-2:]
                continue
            code = m.group(1)
            vals = [None] * len(KEYS)
            for tm in NUMTOK.finditer(ln, m.end()):
                s = tm.start()
                for i, (lo, hi) in enumerate(bounds[:len(KEYS)]):
                    if lo <= s < hi:
                        raw = tm.group(1)
                        vals[i] = None if raw == "-" else float(raw.replace(",", "."))
                        break
            if sum(v is not None for v in vals) < 5:
                continue
            sumber = ln[m.end():bounds[0][0]].strip() if bounds else ""
            if code in foods:
                # First occurrence wins. The composition table comes first; later
                # sections (the errata table, the name index) mention the same codes in
                # a different layout, and parsing those overwrote good rows with junk.
                continue
            foods[code] = {
                "code": code,
                "name_id": " ".join(pending_name).strip(" ,"),
                "source_citation": re.sub(r"\s+", " ", sumber),
                "nutrients": {k: {"value": v, "unit": u}
                              for k, u, v in zip(KEYS, UNITS, vals) if v is not None},
            }
            pending_name = []
    return foods


if __name__ == "__main__":
    f = parse()
    out = os.path.join(HERE, "tkpi-2020-from-pdf.json")
    json.dump(f, open(out, "w"), ensure_ascii=False, indent=1)
    print(f"  parsed {len(f)} foods from the PDF -> {os.path.basename(out)}")
    for c in ("AR001", "AR003", "CP082"):
        r = f.get(c)
        if r:
            n = r["nutrients"]
            print(f"   {c} {r['name_id'][:26]:<26} "
                  f"energy={n.get('energy_kcal',{}).get('value')} "
                  f"prot={n.get('protein_g',{}).get('value')} "
                  f"carb={n.get('carbohydrate_g',{}).get('value')} "
                  f"calcium={n.get('calcium_mg',{}).get('value')}")
