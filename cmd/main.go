package main

import (
	"fmt"
	"net/http"
	"time"

	streamer "github.com/readium/r2-streamer-go"
	"github.com/readium/r2-streamer-go/internal/config"
	"github.com/readium/r2-streamer-go/internal/logging"
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

func init() {
	cnf := config.NewConfig()
	cnf.BindFlags()
	logging.InitLogging()
}

func main() {
	bind := fmt.Sprintf("%s:%d", viper.GetString("bind-address"), viper.GetInt("bind-port"))
	conf := streamer.R2GoConfig{
		Bind:            bind,
		Origins:         viper.GetStringSlice("origins"),
		PublicationPath: viper.GetString("publication-path"),
		StaticPath:      viper.GetString("static-path"),
		SentryDSN:       viper.GetString("sentry-dsn"),
		CacheDSN:        viper.GetString("cache-dsn"),
	}
	s := streamer.NewR2GoServer(conf)

	server := &http.Server{
		ReadTimeout:    10 * time.Second,
		WriteTimeout:   10 * time.Second,
		MaxHeaderBytes: 1 << 20,
		Addr:           bind,
		Handler:        s.Init(),
	}
	logrus.Printf("Starting HTTP Server listening at %q", "http://"+server.Addr)
	if err := server.ListenAndServe(); err != http.ErrServerClosed {
		logrus.Printf("%v", err)
	} else {
		logrus.Println("Goodbye!")
	}
}
