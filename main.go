package main

import (
	_ "expvar"
	"net/http"

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

func main() {
	log.Info("Starting up the search service.")
	log.Fatal(http.ListenAndServe(":60000", nil))
}
