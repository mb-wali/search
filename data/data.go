package data

import (
	"context"
	"encoding/json"
	"github.com/gorilla/mux"
	"github.com/sirupsen/logrus"
	"net/http"

	"github.com/cyverse-de/querydsl"
	"github.com/cyverse-de/querydsl/clause/label"
	"github.com/cyverse-de/querydsl/clause/owner"
	"github.com/cyverse-de/querydsl/clause/path"

	"github.com/cyverse-de/search/elasticsearch"
)

var qd = querydsl.New()

func init() {
	label.Register(qd)
	path.Register(qd)
	owner.Register(qd)
}

func GetAllDocumentationHandler(w http.ResponseWriter, r *http.Request) {
	docs := make(map[string]interface{})
	docs["clauses"] = qd.GetDocumentation()
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	json.NewEncoder(w).Encode(docs)
}

func logAndOutputString(log *logrus.Entry, err string, out *json.Encoder) {
	log.Error(err)
	out.Encode(map[string]string{
		"error": err,
	})
}

func logAndOutputErr(log *logrus.Entry, err error, out *json.Encoder) {
	log.Error(err)
	out.Encode(map[string]string{
		"error": err.Error(),
	})
}

func GetSearchHandler(e *elasticsearch.Elasticer, log *logrus.Entry) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		var v map[string]interface{}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		out := json.NewEncoder(w)
		err := json.NewDecoder(r.Body).Decode(&v)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			logAndOutputErr(log, err, out)
			return
		}
		query, ok := v["query"]
		if !ok {
			w.WriteHeader(http.StatusBadRequest)
			logAndOutputString(log, "Provided body did not contain a 'query' key", out)
			return
		}
		var clauses querydsl.GenericClause
		qjson, _ := json.Marshal(query)
		err = json.Unmarshal(qjson, &clauses)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			logAndOutputErr(log, err, out)
			return
		}
		translated, err := clauses.Translate(qd)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			logAndOutputErr(log, err, out)
			return
		}
		res, err := e.Es.Search().Query(translated).Do(context.TODO())
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			logAndOutputErr(log, err, out)
			return
		}

		out.Encode(res.Hits)

	}
}

func RegisterRoutes(r *mux.Router, e *elasticsearch.Elasticer, log *logrus.Entry) {
	r.HandleFunc("/documentation", GetAllDocumentationHandler)
	r.Path("/search").Methods("POST").HeadersRegexp("Content-Type", "application/json.*").HandlerFunc(GetSearchHandler(e, log))
}
