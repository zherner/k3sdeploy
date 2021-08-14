package main

import (
	"context"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"strings"
	"time"
)

// sshExtractKubeConfig will shell in to the cluster main via bastion to
// pull out the kubeconfig and replace 'default' with cluster main in local copy
func sshExtractKubeConfig(ipBastion, ipClusterMain, clusterName string) []byte {
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

	// replace default with cluster name and loopback ip with cluster main ip
	kubecfg := strings.Replace(string(out), "default", clusterName, -1)
	return []byte(kubecfg)
}

// sshExtractToken ssh in to cluster main via the bastion to extract the k3s cluster token value
// needed by worker nodes to join the cluster.
func sshExtractToken(awscfg aws.Config, k3scfg *cfg, idBastion, idMain string) string {
	log.Println("Getting K3s token.")

	// Using the Config value, create the s3 client
	client := ec2.NewFromConfig(awscfg)

	// lookup instances
	_, _, _, ipBastion := describeInstance(client, k3scfg, "-bastion", idBastion)

	// loop waiting for instance state
	var ipClusterMain []string
	var inState []int32
	numChecks := 45

	log.Println("Waiting on instance state of 'ready'.")
	for i := 1; i <= numChecks; i++ {
		_, inState, ipClusterMain, _ = describeInstance(client, k3scfg, "-main", idMain)
		if inState[0] == 16 {
			break
		}
		time.Sleep(time.Second * 2)
	}
	if ipClusterMain[0] == "" {
		log.Fatalf("failed to get instance state of 'ready' for instance with id %q\n", idMain)
	}

	// sleep a few seconds to give instance time to be able to run ssh command

	// shell command via ssh proxy to get token
	ctx, cancel := context.WithTimeout(context.Background(), 80000*time.Millisecond)
	defer cancel()

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
	var out []byte
	var err error
	numSSHChecks := 10
	for i := 1; i <= numSSHChecks; i++ {
		cmd := exec.CommandContext(ctx, "ssh",
			"-A",
			"-o", "StrictHostKeyChecking=no",
			"ec2-user@"+ipBastion[0],
		)
		_, err := cmd.Output()
		if err == nil {
			break
		}
		time.Sleep(time.Second * 2)
	}

	for i := 1; i <= numSSHChecks; i++ {
		cmd := exec.CommandContext(ctx, "ssh",
			"-A",
			"-o", "StrictHostKeyChecking=no",
			"-J", "ec2-user@"+ipBastion[0],
			"ec2-user@"+ipClusterMain[0],
			"sudo cat /var/lib/rancher/k3s/server/node-token",
		)
		out, err = cmd.Output()
		if err == nil {
			break
		}
		time.Sleep(time.Second * 2)
	}

	if err != nil {
		log.Fatalf("failed to get k3s token from k3s main %q via bastion %q , %v", ipClusterMain[0], ipBastion[0], err)
	}
	if len(out) < 50 {
		log.Fatalf("Something went wrong, expecting long token string.\n")
	}

	// get kubeconfig and write to file
	err = ioutil.WriteFile("./k3s_kubeconfig", sshExtractKubeConfig(ipBastion[0], ipClusterMain[0], k3scfg.clusterName), 0644)
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
