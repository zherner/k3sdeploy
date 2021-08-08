package main

import (
	"context"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"strings"
	"time"
)

func sshExtractKubeConfig(ipBastion, ipClusterMain string) []byte {
	// shell command via ssh proxy to get kubeconfig

	ctx, cancel := context.WithTimeout(context.Background(), 15000*time.Millisecond)
	defer cancel()

	log.Println("Getting K3s kubeconfig.")

	cmd := exec.CommandContext(ctx, "ssh",
		"-A",
		"-o", "StrictHostKeyChecking=no",
		"-J", "ec2-user@"+ipBastion,
		"ec2-user@"+ipClusterMain,
		"sudo cat /etc/rancher/k3s/k3s.yaml",
	)
	// finally get token as output
	out, err := cmd.Output()
	if err != nil {
		log.Fatalf("failed to get k3s token from k3s main %q via bastion %q , %v", ipClusterMain, ipBastion, err)
	}
	if len(out) < 50 {
		log.Fatalf("Something went wrong, expecting long token string.\n")
	}
	return out
}

func sshExtractToken(awscfg aws.Config, k3scfg *cfg) string {
	// TODO: loop to get instance status instead of sleeping
	// sleep to wait for instances to be available
	time.Sleep(time.Second * 60)

	// Using the Config value, create the s3 client
	client := ec2.NewFromConfig(awscfg)

	// init
	var ipBastion string
	var ipClusterMain string

	// filter inputs must be prepended with "tag:"
	var tagK3sdeploycluster = "tag:" + tagK3sdeploycluster
	var tagKey = "tag:" + tagK3sdeploy
	var tagTagName = "tag:" + tagName

	describeInput := &ec2.DescribeInstancesInput{
		Filters: []types.Filter{
			{
				Name:   &tagTagName,
				Values: []string{k3scfg.clusterName + "-bastion"},
			},
			{
				Name:   &tagK3sdeploycluster,
				Values: []string{k3scfg.clusterName},
			},
			{
				Name:   &tagKey,
				Values: []string{tagTrueValue},
			},
		},
	}

	result, err := client.DescribeInstances(context.TODO(), describeInput)
	if err != nil {
		log.Fatalf("failed to describe instance, %v", err)
	}

	// loop over results to get matching instance id in a running state
	for _, v := range result.Reservations {
		for _, k := range v.Instances {
			// 16 - running, 0 - pending
			if *k.State.Code == 16 || *k.State.Code == 0 {
				ipBastion = *k.PublicIpAddress
			}
		}
	}

	// main
	describeInput = &ec2.DescribeInstancesInput{
		Filters: []types.Filter{
			{
				Name:   &tagTagName,
				Values: []string{k3scfg.clusterName + "-main"},
			},
			{
				Name:   &tagK3sdeploycluster,
				Values: []string{k3scfg.clusterName},
			},
			{
				Name:   &tagKey,
				Values: []string{tagTrueValue},
			},
		},
	}

	result, err = client.DescribeInstances(context.TODO(), describeInput)
	if err != nil {
		log.Fatalf("failed to describe instance, %v", err)
	}

	// loop over results to get matching instance id in a running state
	for _, v := range result.Reservations {
		for _, k := range v.Instances {
			// 16 - running, 0 - pending
			if *k.State.Code == 16 || *k.State.Code == 0 {
				ipClusterMain = *k.PrivateIpAddress
			}
		}
	}

	// shell command via ssh proxy to get token

	ctx, cancel := context.WithTimeout(context.Background(), 15000*time.Millisecond)
	defer cancel()

	log.Println("Getting K3s token.")

	//cmd := exec.CommandContext(ctx, "ssh",
	//	"-A",
	//	"-o AddKeysToAgent=yes",
	//	"-o StrictHostKeyChecking=no",
	//	"-J ec2-user@"+ipBastion,
	//	"ec2-user@"+ipClusterMain,
	//	"sudo cat /var/lib/rancher/k3s/server/node-token",
	//)

	// breaking this up in to two steps to avoid the fingerprint confirm
	// interaction, which should be turned off with StrictHostKeyChecking=no
	// but still being prompted.
	cmd := exec.CommandContext(ctx, "ssh",
		"-A",
		"-o", "StrictHostKeyChecking=no",
		"ec2-user@"+ipBastion,
	)
	// have to run something to auto accept the key
	out, err := cmd.Output()
	cmd = exec.CommandContext(ctx, "ssh",
		"-A",
		"-o", "StrictHostKeyChecking=no",
		"-J", "ec2-user@"+ipBastion,
		"ec2-user@"+ipClusterMain,
		"sudo cat /var/lib/rancher/k3s/server/node-token",
	)
	// finally get token as output
	out, err = cmd.Output()
	if err != nil {
		log.Fatalf("failed to get k3s token from k3s main %q via bastion %q , %v", ipClusterMain, ipBastion, err)
	}
	if len(out) < 50 {
		log.Fatalf("Something went wrong, expecting long token string.\n")
	}

	// get kubeconfig and write to file
	err = ioutil.WriteFile("./k3s_kubeconfig", sshExtractKubeConfig(ipBastion, ipClusterMain), 0644)
	if err != nil {
		log.Fatalf("Failed to write kubeconfig.\n")
	}

	return strings.TrimSuffix(string(out), "\n")
}

// sshAgent uses os exec command to add key to ssh-agent
func sshAgent(keyPath string) {
	_, err := os.Stat(keyPath)
	if os.IsNotExist(err) {
		log.Fatalf("the key file %q does not exist, %v", keyPath, err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3000*time.Millisecond)
	defer cancel()

	if err := exec.CommandContext(ctx, "ssh-add", keyPath).Run(); err != nil {
		log.Fatalf("failed to add key %q to ssh agent, %v", keyPath, err)
	}

	log.Printf("Added key %q to ssh agnet", keyPath)
}
