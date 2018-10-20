package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/golang/groupcache"
	cfgroupcache "github.com/poy/cf-groupcache"
	capi "github.com/poy/go-capi"
)

func main() {
	rand.Seed(time.Now().UnixNano())
	log := log.New(os.Stderr, "", log.LstdFlags)
	vcap, appInstance := loadConfig(log)
	route := "http://" + vcap.ApplicationURIs[0]

	p := cfgroupcache.NewHTTPPool(route, appInstance)

	capiClient := capi.NewClient(vcap.CFApi, vcap.ApplicationID, vcap.SpaceID, http.DefaultClient)

	peerManager := cfgroupcache.NewPeerManager(route, vcap.ApplicationID, p, capiClient, log)

	go func() {
		ctx, _ := context.WithTimeout(context.Background(), 10*time.Second)
		peerManager.Tick(ctx)

		for range time.Tick(15 * time.Second) {
			ctx, _ := context.WithTimeout(context.Background(), 10*time.Second)
			peerManager.Tick(ctx)
		}
	}()

	getter := groupcache.GetterFunc(func(ctx groupcache.Context, key string, dest groupcache.Sink) error {
		dest.SetString(fmt.Sprint(rand.Int()))
		return nil
	})
	g := groupcache.NewGroup("example", 1<<20, getter)

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		var value string
		if err := g.Get(nil, strings.Replace(r.URL.Path, "/", "", -1), groupcache.StringSink(&value)); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(err.Error()))
			log.Printf("failed to get: %s", err)
			return
		}

		w.Write([]byte(value))
	})

	log.Fatal(http.ListenAndServe(":"+os.Getenv("PORT"), nil))
}

type Vcap struct {
	ApplicationURIs []string `json:"application_uris"`
	ApplicationID   string   `json:"application_id"`
	SpaceID         string   `json:"space_id"`
	CFApi           string   `json:"cf_api"`
}

func loadConfig(log *log.Logger) (Vcap, string) {
	var vcap Vcap

	if err := json.Unmarshal([]byte(os.Getenv("VCAP_APPLICATION")), &vcap); err != nil {
		log.Fatalf("failed to parse VCAP_APPLICATION: %s", err)
	}

	vcap.CFApi = strings.Replace(vcap.CFApi, "https", "http", 1)

	return vcap, fmt.Sprintf("%s:%s", vcap.ApplicationID, os.Getenv("INSTANCE_INDEX"))
}
