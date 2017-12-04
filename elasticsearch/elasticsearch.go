// Package elasticsearch contains simple wrappers for the elastic library for use in this service
package elasticsearch

import (
	"gopkg.in/olivere/elastic.v5"
)

// Elasticer is a type used to interact with Elasticsearch
type Elasticer struct {
	Es       *elastic.Client
	baseURL  string
	user     string
	password string
	Index    string
}

// NewElasticer returns a pointer to an Elasticer instance that has already tested its connection
// by making a WaitForStatus call to the configured Elasticsearch cluster
func NewElasticer(elasticsearchBase string, user string, password string, elasticsearchIndex string) (*Elasticer, error) {
	c, err := elastic.NewClient(elastic.SetSniff(false), elastic.SetURL(elasticsearchBase), elastic.SetBasicAuth(user, password))

	if err != nil {
		return nil, err
	}

	err = c.WaitForGreenStatus("10s")
	if err != nil {
		return nil, err
	}

	return &Elasticer{Es: c, baseURL: elasticsearchBase, Index: elasticsearchIndex}, nil
}

// Close calls out to the Stop method of the underlying elastic.Client
func (e *Elasticer) Close() {
	e.Es.Stop()
}
