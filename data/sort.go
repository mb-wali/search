package data

import (
	"errors"
	"fmt"

	"github.com/mitchellh/mapstructure"
	"gopkg.in/olivere/elastic.v5"
)

type sortInput struct {
	Order string
	Field string
}

func extractSort(v map[string]interface{}) ([]elastic.SortInfo, error) {
	extracted, ok := v["sort"]
	if !ok {
		return nil, nil
	}

	var sorts []sortInput

	err := mapstructure.Decode(extracted, &sorts)
	if err != nil {
		return nil, err
	}

	var ret []elastic.SortInfo

	for _, sort := range sorts {
		var asc bool

		if sort.Field == "" {
			return nil, errors.New("No field was provided in sort")
		}

		if sort.Order == "ascending" {
			asc = true
		} else if sort.Order == "descending" {
			asc = false

		} else {
			return nil, fmt.Errorf("Order of %q was neither ascending nor descending", sort.Order)
		}

		ret = append(ret, elastic.SortInfo{Ascending: asc, Field: sort.Field})
	}

	return ret, nil
}
