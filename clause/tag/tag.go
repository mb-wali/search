// Package tag provides a wrapper around the normal tag clause that filters to just those owned by the user
package tag

import (
	"context"
	"errors"
	"fmt"

	"github.com/cyverse-de/querydsl"
	"github.com/cyverse-de/querydsl/clause"
	basetag "github.com/cyverse-de/querydsl/clause/tag"
	"github.com/cyverse-de/search/elasticsearch"
	"github.com/mitchellh/mapstructure"
	"gopkg.in/olivere/elastic.v5"
)

const (
	typeKey = "tag"
)

var (
	documentation = clause.ClauseDocumentation{
		Summary: "Searches based on a set of provided tag IDs",
		Args: map[string]clause.ClauseArgumentDocumentation{
			"tags": {Type: "[]string", Summary: "The tag UUIDs to search for"},
		},
	}
)

func TagProcessor(ctx context.Context, args map[string]interface{}) (elastic.Query, error) {
	userRaw := ctx.Value("user")
	if userRaw == nil {
		return nil, errors.New("No user was passed in the context")
	}

	user, ok := userRaw.(string)
	if !ok {
		return nil, errors.New("Couldn't turn user into a string")
	}

	esRaw := ctx.Value("elasticer")
	if esRaw == nil {
		return nil, errors.New("No elasticsearch connection was passed in the context")
	}

	es, ok := esRaw.(*elasticsearch.Elasticer)
	if !ok {
		return nil, errors.New("Couldn't turn elasticsearch connection interface into a real object")
	}

	var realArgs basetag.TagArgs
	err := mapstructure.Decode(args, &realArgs)
	if err != nil {
		return nil, err
	}

	// {"bool": {"must": {"term": {"id": <tag ids>}}, "filter": {"term": {"creator": <user>}}}} against tag type
	// error if the number is different than the number passed in, for now
	var tags []interface{}
	for _, tag := range realArgs.Tags {
		tags = append(tags, tag)
	}

	query := elastic.NewBoolQuery().Must(elastic.NewTermsQuery("id", tags...)).Filter(elastic.NewTermQuery("creator", user))

	res, err := es.Search().Type("tag").Size(0).Query(query).Do(ctx)
	if err != nil {
		return nil, err
	}

	if int64(len(realArgs.Tags)) != res.Hits.TotalHits {
		return nil, fmt.Errorf("When querying for tags, got %d rather than the full number passed, %d", res.Hits.TotalHits, len(realArgs.Tags))
	}

	return basetag.TagProcessor(ctx, args)
}

func Register(qd *querydsl.QueryDSL) {
	qd.AddClauseType(typeKey, TagProcessor, documentation)
}
