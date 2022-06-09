package main

import (
	"github.com/upbound/official-providers/testing/pkg"

	log "github.com/sirupsen/logrus"
)

func main() {
	if err := pkg.RunTest(); err != nil {
		log.Fatal(err.Error())

	}
}
