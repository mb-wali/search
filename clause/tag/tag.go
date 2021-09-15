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

type key int

var userKey key
var elasticerKey key

func NewUserElasticContext(ctx context.Context, user string, elasticer *elasticsearch.Elasticer) context.Context {
	return context.WithValue(context.WithValue(ctx, userKey, user), elasticerKey, elasticer)
}

func UserElasticFromContext(ctx context.Context) (string, *elasticsearch.Elasticer, bool) {
	u, ok := ctx.Value(userKey).(string)
	e, ok2 := ctx.Value(elasticerKey).(*elasticsearch.Elasticer)
	return u, e, (ok && ok2)
}

var (
	documentation = clause.ClauseDocumentation{
		Summary: "Searches based on a set of provided tag IDs",
		Args: map[string]clause.ClauseArgumentDocumentation{
			"tags": {Type: "[]string", Summary: "The tag UUIDs to search for"},
		},
	}
)

func TagProcessor(ctx context.Context, args map[string]interface{}) (elastic.Query, error) {
	user, es, ok := UserElasticFromContext(ctx)
	if !ok {
		return nil, errors.New("Couldn't turn user into a string, or couldn't make elasticsearch connection into a real object")
	}
	if user == "" {
		return nil, errors.New("No user was passed in the context")
	}
	if es == nil {
		return nil, errors.New("No elasticsearch connection was passed in the context")
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
