package data

import (
	"fmt"

	"github.com/mitchellh/mapstructure"
	"gopkg.in/olivere/elastic.v5"
)

type sortInput struct {
	Order string
	Field string
}

var (
	knownFields = map[string]string{
		"creator":      "creator",
		"dateCreated":  "dateCreated",
		"dateModified": "dateModified",
		"fileSize":     "fileSize",
		"id":           "id",
		"label":        "label.keyword",
		"path":         "path.keyword",
	}
)

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

		if knownFields[sort.Field] == "" {
			return nil, fmt.Errorf("Unknown field type %q", sort.Field)
		}

		if sort.Order == "ascending" {
			asc = true
		} else if sort.Order == "descending" {
			asc = false

		} else {
			return nil, fmt.Errorf("Order of %q was neither ascending nor descending", sort.Order)
		}

		ret = append(ret, elastic.SortInfo{Ascending: asc, Field: knownFields[sort.Field]})
	}

	return ret, nil
}
