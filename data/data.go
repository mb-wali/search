package data

import (
	"encoding/json"
	"github.com/gorilla/mux"
	"net/http"

	"github.com/cyverse-de/querydsl"
	"github.com/cyverse-de/querydsl/clause/label"
	"github.com/cyverse-de/querydsl/clause/owner"
	"github.com/cyverse-de/querydsl/clause/path"
)

var qd = querydsl.New()

func init() {
	label.Register(qd)
	path.Register(qd)
	owner.Register(qd)
}

func GetAllDocumentationHandler(w http.ResponseWriter, r *http.Request) {
	docs := qd.GetDocumentation()
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	json.NewEncoder(w).Encode(docs)
}

func RegisterRoutes(r *mux.Router) {
	r.HandleFunc("/documentation", GetAllDocumentationHandler)
}
