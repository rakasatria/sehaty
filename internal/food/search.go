package food

import (
	"fmt"
	"strings"

	"github.com/blevesearch/bleve/v2"
	"github.com/blevesearch/bleve/v2/mapping"
	bquery "github.com/blevesearch/bleve/v2/search/query"
)

// The search index is IN MEMORY and rebuilt at startup.
//
// Bleve's default is an on-disk index, which would write a plaintext copy of the food
// names next to an encrypted database — undoing part of the point of encrypting it. At
// 1,142 short strings the index builds in milliseconds, so there is nothing to gain by
// persisting it and a real property to lose.
func newIndex() (bleve.Index, error) {
	text := bleve.NewTextFieldMapping()
	text.Store = false

	keyword := bleve.NewKeywordFieldMapping()
	keyword.Store = false

	doc := bleve.NewDocumentStaticMapping()
	doc.AddFieldMappingsAt("name_id", text)
	doc.AddFieldMappingsAt("name_en", text)
	doc.AddFieldMappingsAt("name_full", text)
	doc.AddFieldMappingsAt("group", text)
	// Lowercased whole name, unanalysed — this is what the prefix query matches, so a
	// food whose NAME BEGINS with the query outranks one that merely mentions it.
	doc.AddFieldMappingsAt("name_lower", keyword)

	m := mapping.NewIndexMapping()
	m.DefaultMapping = doc
	m.DefaultAnalyzer = "standard"

	idx, err := bleve.NewMemOnly(m)
	if err != nil {
		return nil, fmt.Errorf("build search index: %w", err)
	}
	return idx, nil
}

type indexedFood struct {
	NameID    string `json:"name_id"`
	NameEN    string `json:"name_en"`
	NameFull  string `json:"name_full"`
	Group     string `json:"group"`
	NameLower string `json:"name_lower"`
}

func docFor(f Food) indexedFood {
	return indexedFood{
		NameID: f.NameID, NameEN: f.NameEN, NameFull: f.NameFull, Group: f.Group,
		NameLower: strings.ToLower(f.NameID),
	}
}

// Search returns foods matching the query, best first, and whether the match was exact.
//
// EVERY term must match on the exact path (the match queries use AND). A query is an
// attempt to name one food, so returning rows matching only part of it hands back a
// plausible wrong answer — which for nutrition data is worse than returning nothing.
//
// But "returning nothing" was worse still, because Indonesian names food by qualifying
// it. "Nasi putih" is what everybody says; the table calls it "Nasi", has no entry
// containing "putih", and the AND query therefore returned zero — from which the only
// available conclusion was that rice is not in the Indonesian food table. It is entry
// AP001.
//
// So when the strict pass finds nothing and there was more than one word to be strict
// about, the query is loosened: any single term may match, and a food whose name BEGINS
// with one of them ranks far above one that merely mentions it. The second return value
// reports which path produced the results, and it matters — a loosened hit is a
// suggestion to confirm, never something to act on. See resolve, which refuses to pick
// among them even when there is only one.
func (t *Table) Search(q string, limit int) ([]Food, bool) {
	q = strings.TrimSpace(strings.ToLower(q))
	if q == "" || t.index == nil {
		return nil, false
	}
	if limit <= 0 {
		limit = 25
	}

	exact := bleve.NewMatchQuery(q)
	exact.SetOperator(bquery.MatchQueryOperatorAnd)
	exact.SetBoost(3)

	// One edit of slack — enough for a slip, not enough to reach an unrelated food.
	fuzzy := bleve.NewMatchQuery(q)
	fuzzy.SetOperator(bquery.MatchQueryOperatorAnd)
	fuzzy.SetFuzziness(1)
	fuzzy.SetBoost(1)

	prefix := bleve.NewPrefixQuery(q)
	prefix.SetField("name_lower")
	prefix.SetBoost(8)

	if out := t.run(bleve.NewDisjunctionQuery(exact, fuzzy, prefix), limit); len(out) > 0 {
		return out, true
	}

	terms := strings.Fields(q)
	if len(terms) < 2 {
		return nil, false
	}
	clauses := make([]bquery.Query, 0, len(terms)*2)
	for _, term := range terms {
		m := bleve.NewMatchQuery(term)
		m.SetBoost(2)
		p := bleve.NewPrefixQuery(term)
		p.SetField("name_lower")
		p.SetBoost(8)
		clauses = append(clauses, m, p)
	}
	loose := bleve.NewDisjunctionQuery(clauses...)
	loose.SetMin(1)
	return t.run(loose, limit), false
}

func (t *Table) run(q bquery.Query, limit int) []Food {
	res, err := t.index.Search(bleve.NewSearchRequestOptions(q, limit, 0, false))
	if err != nil {
		return nil
	}
	t.mu.RLock()
	defer t.mu.RUnlock()
	out := make([]Food, 0, len(res.Hits))
	for _, h := range res.Hits {
		if f, ok := t.byCode[strings.ToUpper(h.ID)]; ok {
			out = append(out, f)
		}
	}
	return out
}
