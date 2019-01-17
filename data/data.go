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

// QueryResponder is a type for managing state in an HTTP handler
type QueryResponder struct {
	log    *logrus.Entry
	output *json.Encoder
	writer http.ResponseWriter
	ctx    context.Context
	cfg    *viper.Viper
	user   string
	es     *elasticsearch.Elasticer
}

// GetAllDocumentationHandler outputs documentation from the QueryDSL instance as JSON.
func GetAllDocumentationHandler(w http.ResponseWriter, r *http.Request) {
	docs := make(map[string]interface{})
	docs["clauses"] = qd.GetDocumentation()
	sortFields := make([]string, len(knownFields))

	i := 0
	for k := range knownFields {
		sortFields[i] = k
		i++
	}

	docs["sortFields"] = sortFields

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	json.NewEncoder(w).Encode(docs)
}

func (r *QueryResponder) logAndOutputString(err string) {
	r.log.Error(err)
	r.output.Encode(map[string]string{
		"error": err,
	})
}

func (r *QueryResponder) logAndOutputErr(err error) {
	r.log.Error(err)
	r.output.Encode(map[string]string{
		"error": err.Error(),
	})
}

// getUserGroups fetches the user and its groups with qualified names from data-info, returning the list of users, the response raw if it was non-200, and any error. In a non-failing case, only the first returned value will be non-nil.
func (r *QueryResponder) getUserGroups() ([]string, *http.Response, error) {
	// XXX: go 1.9: use url.PathEscape
	userinfourl := fmt.Sprintf("%s/users/%s/groups?user=%s", r.cfg.GetString("data_info.base"), r.user, url.QueryEscape(r.user))
	req, err := http.NewRequest("GET", userinfourl, nil)
	if err != nil {
		return nil, nil, err
	}

	req = req.WithContext(r.ctx)

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

func (r *QueryResponder) handleScrollId(users []string, sorts []elastic.SortInfo, scrollId string, scroll string) {
	source, err := r.buildSearchSource(users, sorts)
	if err != nil {
		return
	}

	res, err := r.es.Scroll().SearchSource(source).ScrollId(scrollId).Scroll(scroll).Do(r.ctx)
	if err != nil {
		r.writer.WriteHeader(http.StatusBadRequest)
		r.logAndOutputErr(err)
		return
	}

	r.outputSearchResults(res)
}

func (r *QueryResponder) buildSearchSource(users []string, sorts []elastic.SortInfo) (*elastic.SearchSource, error) {
	usersJson, err := json.Marshal(users)
	if err != nil {
		r.writer.WriteHeader(http.StatusInternalServerError)
		r.logAndOutputErr(err)
		return nil, err
	}
	// For each user+permission in the document and each user/group the requesting user is part of, check if the usernames match. If they do, check if the permission in the document is higher than the current calculated value for `perm` -- when it's already at 'own', it can't go higher, if it's 'write' then only 'read' can downgrade it, and otherwise it's either 'read' or null already and can always be used. Once the loops are done, return the calculated `perm`.
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

	return source, nil
}

func (r *QueryResponder) outputSearchResults(res *elastic.SearchResult) {
	type resp struct {
		*elastic.SearchHits
		ScrollId string `json:"scroll_id,omitempty"`
	}

	response := resp{res.Hits, res.ScrollId}

	r.output.Encode(response)
}

// GetSearchHandler returns a function which performs searches after translating an input query
func GetSearchHandler(cfg *viper.Viper, e *elasticsearch.Elasticer, log *logrus.Entry) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		out := json.NewEncoder(w)

		hr := &QueryResponder{es: e, log: log, writer: w, output: out, ctx: ctx, cfg: cfg}

		queries := r.URL.Query()
		user := queries.Get("user")
		if user == "" {
			hr.writer.WriteHeader(http.StatusBadRequest)
			hr.logAndOutputString("The 'user' query parameter must be provided and non-empty")
			return
		}

		hr.user = user

		var v map[string]interface{}
		err := json.NewDecoder(r.Body).Decode(&v)
		if err != nil {
			hr.writer.WriteHeader(http.StatusBadRequest)
			hr.logAndOutputErr(err)
			return
		}
		scrollId, hasScrollId := v["scroll_id"]
		scroll, hasScroll := v["scroll"]
		query, hasQuery := v["query"]
		if !hasQuery && !hasScrollId {
			hr.writer.WriteHeader(http.StatusBadRequest)
			hr.logAndOutputString("Provided body did not contain a 'query' key nor a 'scroll_id' key")
			return
		}
		if hasQuery && hasScrollId {
			hr.writer.WriteHeader(http.StatusBadRequest)
			hr.logAndOutputString("Provided body contained both a 'query' key and a 'scroll_id' key")
			return
		}

		if hasScrollId && !hasScroll {
			hr.writer.WriteHeader(http.StatusBadRequest)
			hr.logAndOutputString("Provided body has a 'scroll_id' but no 'scroll' (scroll timeout)")
			return
		}

		size := extractInt(v, "size", 10)
		from := extractInt(v, "from", 0)

		sorts, err := extractSort(v)
		if err != nil {
			hr.writer.WriteHeader(http.StatusBadRequest)
			hr.logAndOutputErr(err)
			return
		}

		var clauses querydsl.GenericClause
		qjson, _ := json.Marshal(query)
		err = json.Unmarshal(qjson, &clauses)
		if err != nil {
			hr.writer.WriteHeader(http.StatusBadRequest)
			hr.logAndOutputErr(err)
			return
		}

		users, ur, err := hr.getUserGroups()
		if err != nil {
			hr.writer.WriteHeader(http.StatusBadRequest)
			hr.logAndOutputErr(err)
			return
		}
		if ur != nil {
			// passing along the response
			defer ur.Body.Close()
			hr.writer.WriteHeader(ur.StatusCode)
			io.Copy(w, ur.Body)
			return
		}

		if hasScrollId {
			scrollIdString, ok := scrollId.(string)
			if !ok {
				hr.writer.WriteHeader(http.StatusBadRequest)
				hr.logAndOutputString("The scroll ID provided could not be converted to a string")
				return
			}
			scrollString, ok := scroll.(string)
			if !ok {
				hr.writer.WriteHeader(http.StatusBadRequest)
				hr.logAndOutputString("The scroll timeout provided could not be converted to a string")
				return
			}
			hr.handleScrollId(users, sorts, scrollIdString, scrollString)
			return
		}

		clauses.All = append(clauses.All, &querydsl.GenericClause{Clause: &querydsl.Clause{Type: "permissions", Args: map[string]interface{}{"users": users, "permission": "read", "permission_recurse": true, "exact": true}}})

		// Pass in user and elasticsearch connection
		translateCtx := context.WithValue(context.WithValue(hr.ctx, "user", users[len(users)-1]), "elasticer", e)

		translated, err := clauses.Translate(translateCtx, qd)
		if err != nil {
			hr.writer.WriteHeader(http.StatusBadRequest)
			hr.logAndOutputErr(err)
			return
		}
		source, err := hr.buildSearchSource(users, sorts)
		if err != nil {
			return
		}

		var res *elastic.SearchResult
		if hasScroll {
			scrollString, ok := scroll.(string)
			if !ok {
				hr.writer.WriteHeader(http.StatusBadRequest)
				hr.logAndOutputString("The scroll timeout provided could not be converted to a string")
				return
			}
			res, err = e.Scroll().SearchSource(source).Size(size).Query(translated).Scroll(scrollString).Do(hr.ctx)
		} else {
			res, err = e.Search().SearchSource(source).Size(size).From(from).Query(translated).Do(hr.ctx)
		}
		if err != nil {
			hr.writer.WriteHeader(http.StatusBadRequest)
			hr.logAndOutputErr(err)
			return
		}

		hr.outputSearchResults(res)
	}
}

// RegisterRoutes registers the routes associated with this package to the provided router
func RegisterRoutes(r *mux.Router, cfg *viper.Viper, e *elasticsearch.Elasticer, log *logrus.Entry) {
	r.HandleFunc("/documentation", GetAllDocumentationHandler)
	r.Path("/search").Methods("POST").HeadersRegexp("Content-Type", "application/json.*").HandlerFunc(GetSearchHandler(cfg, e, log))
}
