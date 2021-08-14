package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
)

// documentation
// https://aws.github.io/aws-sdk-go-v2/docs/code-examples/
// https://pkg.go.dev/github.com/aws/aws-sdk-go-v2/service/ec2

// cfg is the config object for service
type cfg struct {
	count       int32
	clusterName string
	key         string
	keyPath     string
	subnets     string
	del         bool
}

// getK3sConfig parses input flags to set config object for k3s
func getK3sConfig() *cfg {
	// define input flags, empty default values so that ENV vars can be
	// used to set flags
	count := flag.String("c", "", "The number of k3s cluster instances.")
	name := flag.String("n", "", "The name of the k3s cluster")
	key := flag.String("k", "", "The full path to the ssh key to ues when provisioning instances.")
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
			clusterName: *delName,
			del:         true,
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
		count:       n,
		clusterName: *name,
		key:         strings.TrimSuffix(path.Base(*key), filepath.Ext(path.Base(*key))),
		keyPath:     *key,
		subnets:     *subnets,
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

	// add gey to agent
	go sshAgent(k3scfg.keyPath)

	// create cluster
	createCluster(awscfg, k3scfg)
}
