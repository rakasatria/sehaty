#!/usr/bin/env python3
"""Convert the TKPI 2020 spreadsheet into JSON, and cross-check it against panganku.org.

The spreadsheet is a community transcription of the Kemenkes book; its own header warns
that values may have drifted during editing. Every row therefore keeps its SUMBER citation,
and this script reports disagreements against the official web table so they are visible
rather than assumed away.
"""
import json, os, re, zipfile
from xml.etree import ElementTree as ET

HERE = os.path.dirname(os.path.abspath(__file__))
XLSX = "/Users/raka/Downloads/678341848-TKPI-Kemenkes-2020-Perhitungan-Gizi.xlsx"
NS = {"m": "http://schemas.openxmlformats.org/spreadsheetml/2006/main"}
T = "{http://schemas.openxmlformats.org/spreadsheetml/2006/main}t"

# column -> (json key, unit). Values are per 100 g of EDIBLE PORTION (BDD).
COLS = {
    "E": ("water_g", "g"),        "F": ("energy_kcal", "kcal"),
    "G": ("protein_g", "g"),      "H": ("fat_g", "g"),
    "I": ("carbohydrate_g", "g"), "J": ("fibre_g", "g"),
    "K": ("ash_g", "g"),          "L": ("calcium_mg", "mg"),
    "M": ("phosphorus_mg", "mg"), "N": ("iron_mg", "mg"),
    "O": ("sodium_mg", "mg"),     "P": ("potassium_mg", "mg"),
    "Q": ("copper_mg", "mg"),     "R": ("zinc_mg", "mg"),
    "S": ("retinol_mcg", "mcg"),  "T": ("beta_carotene_mcg", "mcg"),
    "U": ("carotene_total_mcg", "mcg"), "V": ("thiamin_mg", "mg"),
    "W": ("riboflavin_mg", "mg"), "X": ("niacin_mg", "mg"),
    "Y": ("vitamin_c_mg", "mg"),
}


def clean(x):
    """Excel stores 77.1 as 77.099999999999994. Round to a sane precision."""
    if x is None or x == "":
        return None
    try:
        v = float(x)
    except ValueError:
        return None
    r = round(v, 4)
    return int(r) if r == int(r) else round(v, 3)


def load_sheet():
    z = zipfile.ZipFile(XLSX)
    ss = ["".join(t.text or "" for t in si.iter(T))
          for si in ET.fromstring(z.read("xl/sharedStrings.xml")).findall("m:si", NS)]
    sh = ET.fromstring(z.read("xl/worksheets/sheet1.xml"))
    out = []
    for r in sh.findall(".//m:row", NS):
        d = {}
        for c in r.findall("m:c", NS):
            col = "".join(ch for ch in c.get("r") if ch.isalpha())
            v = c.find("m:v", NS)
            if v is None:
                continue
            d[col] = ss[int(v.text)] if c.get("t") == "s" and v.text.isdigit() else v.text
        out.append(d)
    return out


def main():
    rows = load_sheet()

    # code -> group/type/full name, from the official web index (better names, has English)
    index = {}
    idx_path = os.path.join(HERE, "index.json")
    if os.path.exists(idx_path):
        for r in json.load(open(idx_path)):
            index[r["code"]] = r

    foods, skipped = [], 0
    for d in rows:
        code = (d.get("B") or "").strip()
        if not re.fullmatch(r"[A-Z]{2}\d{3,4}", code):
            skipped += 1
            continue
        nutrients = {}
        for col, (key, unit) in COLS.items():
            v = clean(d.get(col))
            if v is not None:
                nutrients[key] = {"value": v, "unit": unit}
        web = index.get(code, {})
        foods.append({
            "code": code,
            "name_id": (d.get("C") or "").strip(),
            "name_full": web.get("name", ""),      # includes the English gloss
            "group": web.get("group", ""),
            "type": web.get("type", ""),
            "source_citation": (d.get("D") or "").strip(),
            "edible_portion_pct": clean(d.get("Z")),
            "per": "100g edible portion (BDD)",
            "nutrients": nutrients,
        })

    doc = {
        "dataset": "Tabel Komposisi Pangan Indonesia (TKPI) 2020",
        "publisher": "Kementerian Kesehatan Republik Indonesia",
        "publisher_url": "https://repository.kemkes.go.id/book/668",
        "basis": "per 100 g edible portion (BDD)",
        "converted_from": os.path.basename(XLSX),
        "cross_checked_against": "https://www.panganku.org/id-ID/cari_nutrisi",
        "caveat": ("This spreadsheet is a community transcription of the Kemenkes book, not "
                   "an official Kemenkes file. Its own header states that values may differ "
                   "from the original and that the book is authoritative. Every row keeps its "
                   "SUMBER citation so any value can be traced."),
        "count": len(foods),
        "foods": foods,
    }
    out = os.path.join(HERE, "tkpi-2020.json")
    with open(out, "w") as f:
        json.dump(doc, f, ensure_ascii=False, indent=1)
    print(f"  wrote {len(foods)} foods -> {out}  ({os.path.getsize(out):,} bytes, {skipped} non-data rows skipped)")

    # ── cross-check against whatever the scrape gathered ──
    scraped = {}
    jl = os.path.join(HERE, "tkpi.jsonl")
    if os.path.exists(jl):
        for line in open(jl):
            try:
                r = json.loads(line)
                scraped[r["code"]] = r.get("nutrients", {})
            except Exception:
                pass
    if not scraped:
        print("  no scraped records to cross-check against")
        return

    pairs = [("energy_kcal", "energi_energy"), ("protein_g", "protein_protein"),
             ("fat_g", "lemak_fat"), ("carbohydrate_g", "karbohidrat_cho")]
    checked = mismatch = 0
    examples = []
    for f in foods:
        w = scraped.get(f["code"])
        if not w:
            continue
        for xk, wk in pairs:
            a, b = f["nutrients"].get(xk), w.get(wk)
            if not a or not b or b.get("value") is None:
                continue
            checked += 1
            if abs(float(a["value"]) - float(b["value"])) > 0.05:
                mismatch += 1
                if len(examples) < 8:
                    examples.append(f"{f['code']} {xk}: xlsx={a['value']} web={b['value']}")
    print(f"  cross-checked {checked} values across {len(scraped)} foods "
          f"present in both sources")
    print(f"  disagreements: {mismatch} ({mismatch/checked*100:.2f}%)" if checked else "")
    for e in examples:
        print("    ", e)


if __name__ == "__main__":
    main()
