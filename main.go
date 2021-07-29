package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"strconv"
)

// to be inputs
var (
	region = "us-east-2"
)

// cfg is the config object for service
type cfg struct {
	count   int32
	name    string
	key     string
	subnets string
	del     bool
}

func getK3sConfig() *cfg {
	// define input flags, empty default values so that ENV vars can be
	// used to set flags
	count := flag.String("c", "", "The number of k3s cluster instances.")
	name := flag.String("n", "", "The name of the k3s cluster")
	key := flag.String("k", "", "The name of the ssh key to ues when provisioning instances.")
	subnets := flag.String("s", "", "Comma separated list of subnets-ids to place instances in.")
	// delete input
	delName := flag.String("d", "", "The name of the cluster to terminate.")

	// usage function prints flags
	usage := func() {
		fmt.Printf("Usage:\n")
		flag.PrintDefaults()
	}

	// parse inputted flags
	flag.Parse()

	// do delete if specified
	if *delName != "" {
		c := cfg{
			name: *delName,
			del:  true,
		}

		return &c
	}

	// init some defaults
	var ok bool
	var n int32

	// if flag was not passed in get from ENV
	if *count == "" {
		envName := "K3S_COUNT"
		*count, ok = os.LookupEnv(envName)
		if !ok {
			usage()
			log.Fatalf("missing required input for %q from command line flag or ENV %q variable.\n", "count", envName)
		}
		num, _ := strconv.Atoi(*count)
		n = int32(num)
	}
	if *name == "" {
		envName := "K3S_NAME"
		*name, ok = os.LookupEnv(envName)
		if !ok {
			usage()
			log.Fatalf("missing required input for %q from command line flag or ENV %q variable.\n", "name", envName)
		}
	}
	if *key == "" {
		envName := "K3S_KEY"
		*key, ok = os.LookupEnv("envName")
		if !ok {
			usage()
			log.Fatalf("missing required input for %q from command line flag or ENV %q variable.\n", "key", envName)
		}
	}
	if *subnets == "" {
		envName := "K3S_SUBNETS"
		*subnets, ok = os.LookupEnv("envName")
		if !ok {
			usage()
			log.Fatalf("missing required input for %q from command line flag or ENV %q variable.\n", "subnets", envName)
		}
	}

	// convert count flag string to int32
	num, _ := strconv.Atoi(*count)
	n = int32(num)

	c := cfg{
		count:   n,
		name:    *name,
		key:     *key,
		subnets: *subnets,
	}
	return &c
}

func main() {

	// parse commandline inputs
	k3scfg := getK3sConfig()

	// auth using defaults
	awscfg := initAWS()

	// print caller info for debug
	//getCallerId(cfg)

	// delete cluster
	if k3scfg.del {
		terminateSequence(awscfg, k3scfg)
	}

	// create cluster
	createInstance(awscfg, k3scfg)
}
