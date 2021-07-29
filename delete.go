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
)

const (
	boldText  = "\033[1m"
	redText   = "\033[31m"
	resetText = "\033[0m"
)

func describeInstance(client *ec2.Client, k3scfg *cfg) (ids []string) {

	// inputs
	var tagName = "tag:Name"
	var tagKey = "tag:" + tagK3sdeploy

	describeInput := &ec2.DescribeInstancesInput{
		Filters: []types.Filter{
			{
				Name:   &tagName,
				Values: []string{k3scfg.name},
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

	// loop over results to get matching instance id
	for _, v := range result.Reservations {
		for _, k := range v.Instances {
			ids = append(ids, *k.InstanceId)
		}
	}

	return ids
}

func terminateInstance(client *ec2.Client, id string) {

	// inputs
	//terminateInput := &ec2.TerminateInstancesInput{
	//	InstanceIds: []string{v},
	//}

	//_, err := client.TerminateInstances(context.TODO(), terminateInput)
	//if err != nil {
	//	log.Fatalf("failed to terminate instance, %v", err)
	//}

	log.Printf("Terminated instance with ID: %q\n", id)
}

func describeSG(client *ec2.Client, k3scfg *cfg) (ids []string) {

	// inputs
	var tagName = "tag:Name"
	var tagKey = "tag:" + tagK3sdeploy

	describeInput := &ec2.DescribeSecurityGroupsInput{
		Filters: []types.Filter{
			{
				Name:   &tagName,
				Values: []string{k3scfg.name},
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

//func deleteSG(client *ec2.Client, id string) {
//
//	// inputs
//	sgInput := &ec2.CreateSecurityGroupInput{
//		Description: &Description,
//		GroupName:   &k3scfg.name,
//		TagSpecifications: []types.TagSpecification{
//			{
//				ResourceType: types.ResourceType("security-group"),
//				Tags: []types.Tag{
//					{
//						Key:   &tagName,
//						Value: &k3scfg.name,
//					},
//					{
//						Key:   &tagSource,
//						Value: &tagSourceValue,
//					},
//					{
//						Key:   &tagK3sdeploy,
//						Value: &tagTrueValue,
//					},
//				},
//			},
//		},
//		VpcId: &vpcID,
//	}
//
//	// delete SG
//	_, err := client.CreateSecurityGroup(context.TODO(), sgInput)
//	if err != nil {
//		log.Fatalf("failed to delete security group, %v", err)
//	}
//
//	log.Printf("Deleted security group with ID: %q\n", id)
//}

func terminateSequence(awscfg aws.Config, k3scfg *cfg) {
	usrInput := "NO"
	fmt.Printf("\n\n" + strings.Repeat("!", 150) + "\n")
	fmt.Printf("\nThe '-d' flag was found. This %sDESTROYS THE CLUSTER%s.\n", redText, resetText)
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
	idsIn := describeInstance(client, k3scfg)

	// lookup sgs
	idsSG := describeSG(client, k3scfg)

	fmt.Printf("\nThe instances that will be %sDESTROYED%s are:\n", redText, resetText)
	for _, v := range idsIn {
		fmt.Println(v)
	}

	fmt.Printf("\nAssociated security groups that will also be %sDESTROYED%s are:\n", redText, resetText)
	for _, v := range idsSG {
		fmt.Println(v)
	}

	usrInput = "NO"
	fmt.Printf("\nThere is no going back. Only %s'YES'%s will be accpeted.\n%s%sFINALIZE DESTROY?%s:", boldText, resetText, boldText, redText, resetText)
	fmt.Scanln(&usrInput)
	if usrInput == "YES" {
		for _, v := range idsIn {
			// print creating
			log.Printf("Destroying cluster %q\n", k3scfg.name)
			terminateInstance(client, v)
		}
	}

	// terminate instance

	//// use one for min and max since we want to create one instance at a time in each subnet
	//one := int32(1)
	//j := 0

	//// loop over instance count and spread instances over the number of subnets provided.
	//for i := int32(1); i <= k3scfg.count; i++ {
	//	runInput := &ec2.RunInstancesInput{
	//		ImageId:          aws.String(amiID),
	//		InstanceType:     types.InstanceTypeT2Micro,
	//		KeyName:          &k3scfg.key,
	//		MinCount:         &one,
	//		MaxCount:         &one,
	//		SecurityGroupIds: id,
	//		SubnetId:         &subnets[j],
	//		UserData:         b64(userData),
	//	}

	// if times iterated through subnets is equal to len of subnets, reset index j
	// so that we can somewhat evenly spread instances to subnets.
	//if j+1 == len(subnets) {
	//	j = 0
	//} else {
	//	j++
	//}

	os.Exit(0)
}
