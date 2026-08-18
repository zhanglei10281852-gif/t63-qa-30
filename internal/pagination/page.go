package pagination

import (
	"fmt"
	"strconv"
)

type Query struct {
	Limit  int
	Offset int
	Sort   string
	Desc   bool
}

func Parse(limit, offset, sort string, desc bool) (Query, error) {
	q := Query{Limit: 25, Sort: sort, Desc: desc}
	if limit != "" {
		value, err := strconv.Atoi(limit)
		if err != nil || value < 1 || value > 100 {
			return Query{}, fmt.Errorf("limit must be between 1 and 100")
		}
		q.Limit = value
	}
	if offset != "" {
		value, err := strconv.Atoi(offset)
		if err != nil || value < 0 {
			return Query{}, fmt.Errorf("offset must be non-negative")
		}
		q.Offset = value
	}
	return q, nil
}

type Result[T any] struct {
	Items  []T `json:"items"`
	Total  int `json:"total"`
	Limit  int `json:"limit"`
	Offset int `json:"offset"`
}
