package data

import (
	"strings"

	"gorm.io/gorm"
)

// MatchSearchTerms applies literal, case-insensitive terms to a fixed SQL expression.
func MatchSearchTerms(query *gorm.DB, expression string, terms []string) *gorm.DB {
	escape := strings.NewReplacer("!", "!!", "%", "!%", "_", "!_")
	for _, term := range terms {
		query = query.Where("LOWER("+expression+") LIKE ? ESCAPE '!'", "%"+escape.Replace(strings.ToLower(term))+"%")
	}
	return query
}
