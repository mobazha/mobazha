package core

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/mobazha/mobazha/pkg/contracts"
)

// placeholderPattern finds a protected term on the way back from an engine. The
// inner spaces are tolerated on purpose: engines like to tidy brackets, and a
// term that comes back as "[[ 1 ]]" is still a term.
var placeholderPattern = regexp.MustCompile(`\[\[\s*(\d+)\s*\]\]`)

// protectGlossary hides the terms an engine must not touch behind placeholders.
//
// A model can be asked to leave a word alone; a machine translation service
// cannot be asked anything. So for those the terms are taken out of the text
// before it is sent and put back afterwards — which also gives them their
// preferred spelling, for the entries that have one.
//
// Only terms that actually appear are replaced, and they are numbered in the
// order the sentence mentions them, so the placeholders read in the same order
// as the text they came from.
func protectGlossary(text string, glossary []contracts.TranslationTerm) (string, []contracts.TranslationTerm) {
	protected := make([]contracts.TranslationTerm, 0, len(glossary))
	type found struct {
		term contracts.TranslationTerm
		at   int
	}
	candidates := make([]found, 0, len(glossary))
	for _, term := range glossary {
		source := strings.TrimSpace(term.Source)
		if source == "" {
			continue
		}
		if at := strings.Index(text, source); at >= 0 {
			candidates = append(candidates, found{term: term, at: at})
		}
	}
	sort.SliceStable(candidates, func(i, j int) bool { return candidates[i].at < candidates[j].at })

	out := text
	for _, candidate := range candidates {
		source := strings.TrimSpace(candidate.term.Source)
		out = strings.ReplaceAll(out, source, fmt.Sprintf("[[%d]]", len(protected)+1))
		protected = append(protected, candidate.term)
	}
	return out, protected
}

// restoreGlossary puts the terms back. A term with a preferred spelling comes
// back as that spelling; one without comes back exactly as the seller wrote it.
func restoreGlossary(text string, protected []contracts.TranslationTerm) string {
	if len(protected) == 0 {
		return text
	}
	return placeholderPattern.ReplaceAllStringFunc(text, func(match string) string {
		groups := placeholderPattern.FindStringSubmatch(match)
		if len(groups) < 2 {
			return match
		}
		index, err := strconv.Atoi(groups[1])
		if err != nil || index < 1 || index > len(protected) {
			// Not ours: an engine that invented its own brackets keeps them.
			return match
		}
		term := protected[index-1]
		if target := strings.TrimSpace(term.Target); target != "" {
			return target
		}
		return strings.TrimSpace(term.Source)
	})
}
