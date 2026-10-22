package commandpalette

import (
	"sort"
	"strings"
)

const (
	searchFieldCapacityBase = 2
	exactMatchScore         = 1000
	prefixMatchScore        = 800
	containsMatchScore      = 600
	containsPenaltyLimit    = 100
	fuzzyMatchScore         = 400
	fuzzyPenaltyLimit       = 200
)

type rankedItem struct {
	item  Item
	score int
}

func (m *Model) refilter() {
	query := strings.ToLower(strings.TrimSpace(m.textInput.Value()))
	if query == "" {
		m.filtered = cloneItems(m.items)
	} else {
		terms := strings.Fields(query)
		ranked := make([]rankedItem, 0, len(m.items))
		for _, item := range m.items {
			if score, ok := itemScore(item, terms); ok {
				ranked = append(ranked, rankedItem{item: item, score: score})
			}
		}
		sort.SliceStable(ranked, func(left, right int) bool {
			return ranked[left].score > ranked[right].score
		})
		m.filtered = make([]Item, len(ranked))
		for index := range ranked {
			m.filtered[index] = ranked[index].item
		}
	}
	m.cursor = 0
	m.renderIndex = 0
}

func itemScore(item Item, terms []string) (int, bool) {
	fields := make([]struct {
		value string
		boost int
	}, 0, searchFieldCapacityBase+len(item.Keywords))
	fields = append(fields, struct {
		value string
		boost int
	}{strings.ToLower(item.Label), 450})
	for _, keyword := range item.Keywords {
		fields = append(fields, struct {
			value string
			boost int
		}{strings.ToLower(keyword), 200})
	}
	fields = append(fields, struct {
		value string
		boost int
	}{strings.ToLower(item.Category), 100})

	total := 0
	for _, term := range terms {
		best := -1
		for _, field := range fields {
			if score, ok := fuzzyScore(field.value, term); ok {
				best = max(best, score+field.boost)
			}
		}
		if best < 0 {
			return 0, false
		}
		total += best
	}
	return total, true
}

func fuzzyScore(value, term string) (int, bool) {
	if value == "" || term == "" {
		return 0, false
	}
	if value == term {
		return exactMatchScore, true
	}
	if strings.HasPrefix(value, term) {
		return prefixMatchScore, true
	}
	if index := strings.Index(value, term); index >= 0 {
		return containsMatchScore - min(index, containsPenaltyLimit), true
	}

	valueRunes := []rune(value)
	termRunes := []rune(term)
	termIndex := 0
	first := -1
	for valueIndex, candidate := range valueRunes {
		if candidate != termRunes[termIndex] {
			continue
		}
		if first < 0 {
			first = valueIndex
		}
		termIndex++
		if termIndex == len(termRunes) {
			spanPenalty := valueIndex - first - len(termRunes) + 1
			return fuzzyMatchScore - min(first+spanPenalty, fuzzyPenaltyLimit), true
		}
	}
	return 0, false
}
