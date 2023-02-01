package main

import (
	"embed"
	"flag"
	"github.com/gorilla/pat"
	comcfg "github.com/jphautin/MailHog/config"
	"github.com/jphautin/mailhog-gui/config"
	"github.com/jphautin/mailhog-gui/web"
	"log"
	gohttp "net/http"
	"os"
)

// content holds our static web server content.
//
//go:embed assets/*
var content embed.FS

var conf *config.Config
var comconf *comcfg.Config
var exitCh chan int

func configure() {
	comcfg.RegisterFlags()
	config.RegisterFlags()
	flag.Parse()
	conf = config.Configure()
	comconf = comcfg.Configure()
	// FIXME hacky
	web.APIHost = conf.APIHost
}

func main() {
	configure()

	// FIXME need to make API URL configurable

	if comconf.AuthFile != "" {
		web.AuthFile(comconf.AuthFile)
	}

	exitCh = make(chan int)
	cb := func(r gohttp.Handler) {
		web.CreateWeb(conf, r.(*pat.Router), content)
	}
	go web.Listen(conf.UIBindAddr, cb)

	for {
		select {
		case <-exitCh:
			log.Printf("Received exit signal")
			os.Exit(0)
		}
	}
}
