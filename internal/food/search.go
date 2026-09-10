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

// Search returns foods matching the query, best first.
//
// EVERY term must match (the match queries use AND). A query is an attempt to name one
// food, so returning rows matching only part of it hands back a plausible wrong answer —
// which for nutrition data is worse than returning nothing. Fuzziness is allowed as a
// separate, lower-scoring clause so a typo still finds something without loosening the
// exact path.
func (t *Table) Search(q string, limit int) []Food {
	q = strings.TrimSpace(strings.ToLower(q))
	if q == "" || t.index == nil {
		return nil
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

	req := bleve.NewSearchRequestOptions(
		bleve.NewDisjunctionQuery(exact, fuzzy, prefix), limit, 0, false)

	res, err := t.index.Search(req)
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
