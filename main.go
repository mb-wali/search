package main

import (
	_ "expvar"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/sirupsen/logrus"
)

var log = logrus.WithFields(logrus.Fields{
	"service": "search",
	"art-id":  "search",
	"group":   "org.cyverse",
})

func init() {
	logrus.SetFormatter(&logrus.JSONFormatter{})
}

func newRouter() *mux.Router {
	r := mux.NewRouter()
	r.Handle("/debug/vars", http.DefaultServeMux)
	return r
}

func main() {
	log.Info("Starting up the search service.")
	r := newRouter()
	log.Fatal(http.ListenAndServe(":60000", r))
}
