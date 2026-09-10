#!/usr/bin/env python3
"""Extract the Indonesian food composition table (TKPI) from panganku.org.

The Kemenkes PDF is 140 scanned page-images at ~72 DPI — OCR on a dense numeric
table there produces plausible-looking wrong numbers, which for nutrition data is
worse than no data. panganku.org serves the same table as real HTML text.

Polite by construction: one request per second, identifying User-Agent, resumable
so a re-run never re-fetches what it already has.
"""
import html, json, os, re, sys, time, urllib.parse, urllib.request

BASE = "https://www.panganku.org/id-ID"
UA = ("SehatyDataImport/0.1 (personal health agent; one-time import; "
      "contact: raka.satria@hotmail.com)")
HDRS = {"User-Agent": UA, "Content-Type": "application/x-www-form-urlencoded"}
DELAY = 1.0
OUT = os.path.join(os.path.dirname(os.path.abspath(__file__)), "tkpi.jsonl")
LOG = os.path.join(os.path.dirname(os.path.abspath(__file__)), "extract.log")

GROUPS = ["Serealia", "Umbi Berpati", "Kacang-Kacangan", "Sayuran", "Buah",
          "Daging", "Ikan/Kerang/Udang dll", "Telur", "Susu", "Minyak/Lemak",
          "Konfeksioneri", "Bumbu", "Minuman Non Alkohol"]


def log(msg):
    line = f"{time.strftime('%H:%M:%S')} {msg}"
    print(line, flush=True)
    with open(LOG, "a") as f:
        f.write(line + "\n")


def post(path, fields, tries=3):
    body = urllib.parse.urlencode(fields).encode()
    for attempt in range(tries):
        try:
            req = urllib.request.Request(f"{BASE}/{path}", data=body, headers=HDRS)
            return urllib.request.urlopen(req, timeout=60).read().decode("utf-8", "replace")
        except Exception as e:
            if attempt == tries - 1:
                raise
            # Back off rather than hammering a government site that is struggling.
            time.sleep(3 * (attempt + 1))
    return ""


def cells(chunk):
    out = [html.unescape(re.sub(r"<[^>]+>", "", c)).strip()
           for c in re.findall(r"<t[dh][^>]*>(.*?)</t[dh]>", chunk, re.S)]
    return [c for c in out if c and c != ":"]


def list_group(group):
    h = post("cari_nutrisi", {"kategori": group})
    m = re.search(r'<table id="data".*?</table>', h, re.S)
    if not m:
        return []
    rows = []
    for r in re.findall(r"<tr[^>]*>(.*?)</tr>", m.group(0), re.S):
        c = cells(r)
        if len(c) >= 5 and re.fullmatch(r"[A-Z]{2}\d{3,4}", c[1]):
            rows.append({"code": c[1], "name": c[2], "group": c[3], "type": c[4]})
    return rows


# "Energi (Energy) : 357 Kal" -> ("energi_energy", 357.0, "Kal")
VALUE = re.compile(r"^:?\s*(-?[\d.,]+)\s*(\S+)?$")


def slug(label):
    s = re.sub(r"\s+", "_", label.strip().lower())
    return re.sub(r"[^a-z0-9_]+", "", s.replace("(", "").replace(")", ""))


def detail(code):
    h = post("view", {"haha": code})
    rec, nutrients = {"code": code}, {}
    for t in re.findall(r"<table.*?</table>", h, re.S):
        for r in re.findall(r"<tr[^>]*>(.*?)</tr>", t, re.S):
            c = cells(r)
            if len(c) < 2:
                continue
            label, value = c[0], c[-1]
            key = slug(label)
            m = VALUE.match(value)
            if m:
                num = m.group(1).replace(",", ".")
                try:
                    nutrients[key] = {"value": float(num), "unit": m.group(2) or ""}
                except ValueError:
                    nutrients[key] = {"raw": value}
            elif key in ("kode", "nama", "nama_latin", "asal", "kategori",
                         "tipe_bahan", "keterangan"):
                rec[key] = value
    rec["nutrients"] = nutrients
    return rec


def main():
    done = set()
    if os.path.exists(OUT):
        with open(OUT) as f:
            for line in f:
                try:
                    done.add(json.loads(line)["code"])
                except Exception:
                    pass
        log(f"resuming: {len(done)} already extracted")

    index = []
    for g in GROUPS:
        try:
            rows = list_group(g)
        except Exception as e:
            log(f"GROUP FAIL {g}: {e}")
            continue
        log(f"{g}: {len(rows)} foods")
        index.extend(rows)
        time.sleep(DELAY)

    with open(os.path.join(os.path.dirname(OUT), "index.json"), "w") as f:
        json.dump(index, f, ensure_ascii=False, indent=1)
    log(f"index: {len(index)} foods across {len(GROUPS)} groups")

    todo = [r for r in index if r["code"] not in done]
    log(f"fetching {len(todo)} details (~{len(todo) * DELAY / 60:.0f} min)")

    ok = fail = 0
    with open(OUT, "a") as f:
        for i, row in enumerate(todo, 1):
            try:
                rec = detail(row["code"])
                rec.setdefault("nama", row["name"])
                rec.setdefault("kategori", row["group"])
                rec.setdefault("tipe_bahan", row["type"])
                f.write(json.dumps(rec, ensure_ascii=False) + "\n")
                f.flush()
                ok += 1
            except Exception as e:
                fail += 1
                log(f"FAIL {row['code']}: {e}")
            if i % 50 == 0:
                log(f"  {i}/{len(todo)}  ok={ok} fail={fail}")
            time.sleep(DELAY)
    log(f"DONE ok={ok} fail={fail} -> {OUT}")


if __name__ == "__main__":
    main()
