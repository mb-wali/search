package main

import (
	_ "expvar"
	"flag"
	"net/http"

	"github.com/cyverse-de/configurate"
	"github.com/spf13/viper"

	"github.com/cyverse-de/search/data"
	"github.com/cyverse-de/search/elasticsearch"

	"github.com/gorilla/mux"
	"github.com/sirupsen/logrus"
)

var log = logrus.WithFields(logrus.Fields{
	"service": "search",
	"art-id":  "search",
	"group":   "org.cyverse",
})

var (
	cfgPath = flag.String("config", "", "Path to the configuration file.")
	cfg     *viper.Viper
)

func init() {
	flag.Parse()
	logrus.SetFormatter(&logrus.JSONFormatter{})
}

func loadConfig(cfgPath string) {
	var err error
	cfg, err = configurate.Init(cfgPath)
	if err != nil {
		log.Fatal(err)
	}
}

func newRouter(e *elasticsearch.Elasticer) *mux.Router {
	r := mux.NewRouter()
	r.Handle("/debug/vars", http.DefaultServeMux)
	data.RegisterRoutes(r.PathPrefix("/data/").Subrouter(), e, log)

	return r
}

func main() {
	log.Info("Starting up the search service.")
	loadConfig(*cfgPath)
	e, err := elasticsearch.NewElasticer(cfg.GetString("elasticsearch.base"), cfg.GetString("elasticsearch.user"), cfg.GetString("elasticsearch.password"), cfg.GetString("elasticsearch.index"))
	if err != nil {
		log.Fatal(err)
	}

	r := newRouter(e)
	listenPortSpec := ":" + "60000"
	log.Infof("Listening on %s", listenPortSpec)
	log.Fatal(http.ListenAndServe(listenPortSpec, r))
}
