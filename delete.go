package main

import (
	"context"
	"fmt"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"log"
	"os"
	"strings"
	"time"
)

const (
	boldText  = "\033[1m"
	redText   = "\033[31m"
	resetText = "\033[0m"
)

// terminateInstance destroys the instance with id
func terminateInstance(client *ec2.Client, id string) {

	// inputs
	terminateInput := &ec2.TerminateInstancesInput{
		InstanceIds: []string{id},
	}

	_, err := client.TerminateInstances(context.TODO(), terminateInput)
	if err != nil {
		log.Fatalf("failed to terminate instance, %v", err)
	}

	log.Printf("Terminated instance with ID: %q\n", id)
}

// describeSG returns sg ids created by this tool and associated with the cluster name
func describeSG(client *ec2.Client, k3scfg *cfg) (ids []string) {

	// inputs
	var tagK3sdeploycluster = "tag:" + tagK3sdeploycluster
	var tagKey = "tag:" + tagK3sdeploy

	describeInput := &ec2.DescribeSecurityGroupsInput{
		Filters: []types.Filter{
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

	result, err := client.DescribeSecurityGroups(context.TODO(), describeInput)
	if err != nil {
		log.Fatalf("failed to describe security group, %v", err)
	}

	// loop over results to get matching instance id
	for _, v := range result.SecurityGroups {
		ids = append(ids, *v.GroupId)
	}

	return ids
}

// deleteSG destroys the sg with id
func deleteSG(client *ec2.Client, id string) {

	// inputs
	sgInput := &ec2.DeleteSecurityGroupInput{
		GroupId: &id,
	}

	// delete SG
	_, err := client.DeleteSecurityGroup(context.TODO(), sgInput)
	if err != nil {
		log.Fatalf("failed to delete security group, %v", err)
	}

	log.Printf("Deleted security group with ID: %q\n", id)
}

// terminateSequence uses cluster name and k3sdeploy=true tags to identify which instances/sgs are
// associated to the cluster and destroys them one at a time.
func terminateSequence(awscfg aws.Config, k3scfg *cfg) {
	usrInput := "NO"
	fmt.Printf("\n%s%s%s\n", boldText, strings.Repeat("#", 150), resetText)
	fmt.Printf("\nThe '-d' flag was found. This %sDESTROYS THE CLUSTER and BASTION%s.\n", redText, resetText)
	fmt.Printf("Are you sure you want to continue with the %sDESTROY%s?. Only %s'YES'%s will be accepted.\n%s%sCONTINUE DESTROY?%s:", redText, resetText, boldText, resetText, boldText, redText, resetText)
	fmt.Scanln(&usrInput)
	if usrInput == "YES" {
		//continue
	} else {
		fmt.Println("Cancelling.")
		os.Exit(1)
	}

	// Using the Config value, create the s3 client
	client := ec2.NewFromConfig(awscfg)

	// lookup instances
	idsIn, _, _, _ := describeInstance(client, k3scfg, "", "")

	// lookup sgs
	idsSG := describeSG(client, k3scfg)

	// exit early if nothing found
	if len(idsIn) == 0 && len(idsSG) == 0 {
		fmt.Printf("\nNo resources found associated with the %q cluster. Exiting.\n", k3scfg.clusterName)
		os.Exit(0)
	}

	fmt.Printf("\n%s%s%s\nThe following resources were found to belong to the %s%q%s cluster.\n", boldText, strings.Repeat("#", 20), resetText, boldText, k3scfg.clusterName, resetText)
	fmt.Printf("\nThe instances that will be %sDESTROYED%s are:\n", redText, resetText)

	for _, v := range idsIn {
		fmt.Println("  ", v)
	}

	fmt.Printf("\nAssociated security groups that will also be %sDESTROYED%s are:\n", redText, resetText)
	for _, v := range idsSG {
		fmt.Println("  ", v)
	}
	fmt.Printf("%s%s%s\n", boldText, strings.Repeat("#", 20), resetText)

	usrInput = "NO"
	fmt.Printf("\nThere is no going back. Only %s'YES'%s will be accpeted.\n%s%sFINALIZE DESTROY?%s:", boldText, resetText, boldText, redText, resetText)
	fmt.Scanln(&usrInput)

	if usrInput == "YES" {
		log.Printf("Destroying cluster %q\n", k3scfg.clusterName)
		if len(idsIn) != 0 || len(idsSG) != 0 {
			// destroy instances
			for _, v := range idsIn {
				terminateInstance(client, v)

				// wait
				numChecks := 45
				log.Println("Waiting on instance state of 'terminated'.")
				for i := 1; i <= numChecks; i++ {
					_, inState, _, _ := describeInstance(client, k3scfg, "", v)
					if inState[0] == 48 {
						break
					}
					time.Sleep(time.Second * 2)
				}
			}
			// destory sgs
			for _, v := range idsSG {
				deleteSG(client, v)
			}
		}
		if len(idsIn) == 0 {
			fmt.Printf("\nNo instances in a running state found associated with the %q cluster. Skipping.\n", k3scfg.clusterName)
		}
		if len(idsSG) == 0 {
			fmt.Printf("\nNo security grups found associated with the %q cluster. Skipping.\n", k3scfg.clusterName)
		}

	}

	os.Exit(0)
}
