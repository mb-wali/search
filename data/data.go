// Package data contains handlers and logic for data searches for the CyVerse data store
package data

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/gorilla/mux"
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	"io"
	"net/http"
	"net/url"

	"github.com/cyverse-de/querydsl"
	"github.com/cyverse-de/querydsl/clause/created"
	"github.com/cyverse-de/querydsl/clause/label"
	"github.com/cyverse-de/querydsl/clause/metadata"
	"github.com/cyverse-de/querydsl/clause/modified"
	"github.com/cyverse-de/querydsl/clause/owner"
	"github.com/cyverse-de/querydsl/clause/path"
	"github.com/cyverse-de/querydsl/clause/permissions"
	"github.com/cyverse-de/querydsl/clause/size"
	"github.com/cyverse-de/search/clause/tag"

	"github.com/cyverse-de/search/elasticsearch"
	"gopkg.in/olivere/elastic.v5"
)

var qd = querydsl.New()

func init() {
	label.Register(qd)
	path.Register(qd)
	owner.Register(qd)
	permissions.Register(qd)
	metadata.Register(qd)
	created.Register(qd)
	modified.Register(qd)
	size.Register(qd)
	tag.Register(qd)
}

// GetAllDocumentationHandler outputs documentation from the QueryDSL instance as JSON.
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

// getUserGroups fetches the user and its groups with qualified names from data-info, returning the list of users, the response raw if it was non-200, and any error. In a non-failing case, only the first returned value will be non-nil.
func getUserGroups(ctx context.Context, cfg *viper.Viper, user string) ([]string, *http.Response, error) {
	// XXX: go 1.9: use url.PathEscape
	userinfourl := fmt.Sprintf("%s/users/%s/groups?user=%s", cfg.GetString("data_info.base"), user, url.QueryEscape(user))
	req, err := http.NewRequest("GET", userinfourl, nil)
	if err != nil {
		return nil, nil, err
	}

	req = req.WithContext(ctx)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, nil, err
	}
	if resp.StatusCode != 200 {
		return nil, resp, nil
	}
	defer resp.Body.Close()

	var decoded struct {
		User   string
		Groups []string
	}
	err = json.NewDecoder(resp.Body).Decode(&decoded)
	if err != nil {
		return nil, nil, err
	}
	return append(decoded.Groups, decoded.User), nil, nil
}

func extractInt(v map[string]interface{}, field string, default_val int) int {
	extracted, ok := v[field]
	if !ok {
		return default_val
	}
	float, ok := extracted.(float64)
	if !ok {
		return default_val
	}
	return int(float)
}

// GetSearchHandler returns a function which performs searches after translating an input query
func GetSearchHandler(cfg *viper.Viper, e *elasticsearch.Elasticer, log *logrus.Entry) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		out := json.NewEncoder(w)

		queries := r.URL.Query()
		user := queries.Get("user")
		if user == "" {
			w.WriteHeader(http.StatusBadRequest)
			logAndOutputString(log, "The 'user' query parameter must be provided and non-empty", out)
			return
		}

		var v map[string]interface{}
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

		size := extractInt(v, "size", 10)
		from := extractInt(v, "from", 0)

		sorts, err := extractSort(v)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			logAndOutputErr(log, err, out)
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

		users, ur, err := getUserGroups(ctx, cfg, user)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			logAndOutputErr(log, err, out)
			return
		}
		if ur != nil {
			// passing along the response
			defer ur.Body.Close()
			w.WriteHeader(ur.StatusCode)
			io.Copy(w, ur.Body)
			return
		}

		clauses.All = append(clauses.All, &querydsl.GenericClause{Clause: &querydsl.Clause{Type: "permissions", Args: map[string]interface{}{"users": users, "permission": "read", "permission_recurse": true, "exact": true}}})

		// Pass in user and elasticsearch connection
		translateCtx := context.WithValue(context.WithValue(ctx, "user", users[len(users)-1]), "elasticer", e)

		translated, err := clauses.Translate(translateCtx, qd)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			logAndOutputErr(log, err, out)
			return
		}
		usersJson, err := json.Marshal(users)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			logAndOutputErr(log, err, out)
			return
		}
		permFieldScript := `
		String perm = null;
		for (up in params._source.userPermissions) {
			for (user in ` + string(usersJson) + `) {
				if (up.user == user && perm != 'own' && !(up.permission == 'read' && perm == 'write')) {
					perm = up.permission
				}
			}
		}
		perm`
		source := elastic.NewSearchSource().FetchSource(true).ScriptField(elastic.NewScriptField("permission", elastic.NewScriptInline(permFieldScript).Lang("painless")))

		for _, sort := range sorts {
			source = source.SortWithInfo(sort)
		}

		res, err := e.Search().SearchSource(source).Size(size).From(from).Query(translated).Do(ctx)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			logAndOutputErr(log, err, out)
			return
		}

		out.Encode(res.Hits)

	}
}

// RegisterRoutes registers the routes associated with this package to the provided router
func RegisterRoutes(r *mux.Router, cfg *viper.Viper, e *elasticsearch.Elasticer, log *logrus.Entry) {
	r.HandleFunc("/documentation", GetAllDocumentationHandler)
	r.Path("/search").Methods("POST").HeadersRegexp("Content-Type", "application/json.*").HandlerFunc(GetSearchHandler(cfg, e, log))
}
