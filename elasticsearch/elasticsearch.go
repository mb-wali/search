// Package elasticsearch contains simple wrappers for the elastic library for use in this service
package elasticsearch

import (
	"github.com/pkg/errors"
	"gopkg.in/olivere/elastic.v5"
)

// Elasticer is a type used to interact with Elasticsearch
type Elasticer struct {
	es      *elastic.Client
	baseURL string
	index   string
}

// NewElasticer returns a pointer to an Elasticer instance that has already tested its connection
// by making a WaitForStatus call to the configured Elasticsearch cluster
func NewElasticer(elasticsearchBase string, user string, password string, elasticsearchIndex string) (*Elasticer, error) {
	c, err := elastic.NewClient(elastic.SetSniff(false), elastic.SetURL(elasticsearchBase), elastic.SetBasicAuth(user, password))

	if err != nil {
		return nil, errors.Wrap(err, "Failed to create elastic client")
	}

	wait := "10s"
	err = c.WaitForYellowStatus(wait)
	if err != nil {
		return nil, errors.Wrapf(err, "Cluster did not report yellow or better status within %s", wait)
	}

	return &Elasticer{es: c, baseURL: elasticsearchBase, index: elasticsearchIndex}, nil
}

// Search returns an *elastic.SearchService set to the right index, for further use
func (e *Elasticer) Search() *elastic.SearchService {
	return e.es.Search().Index(e.index)
}

// Scroll returns an *elastic.ScrollService set to the right index, for further use
func (e *Elasticer) Scroll() *elastic.ScrollService {
	return e.es.Scroll().Index(e.index)
}

// Close calls out to the Stop method of the underlying elastic.Client
func (e *Elasticer) Close() {
	e.es.Stop()
}
