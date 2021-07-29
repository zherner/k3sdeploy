package main

import (
	"context"
	"encoding/base64"
	"log"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
)

var (
	userData = `
#!/usr/bin/env bash
curl -sfL https://get.k3s.io | sh -
`
	//subnet         = "subnet-017584c03579c5d3e"
	amiID          = "ami-0233c2d874b811deb"
	tagName        = "Name"
	tagSource      = "source"
	tagSourceValue = "https://github.com/zherner/k3sbase"
	tagK3sdeploy   = "k3sdeploy"
	tagTrueValue   = "true"
)

//
func b64(str string) *string {
	enc := base64.StdEncoding.EncodeToString([]byte(str))
	return &enc
}

// tagInstance takes a slice of instanceIds and tags them
func tagInstance(client *ec2.Client, instances []types.Instance, name string) {

	for _, v := range instances {
		tagInput := &ec2.CreateTagsInput{
			Resources: []string{*v.InstanceId},
			Tags: []types.Tag{
				{
					Key:   &tagName,
					Value: &name,
				},
				{
					Key:   &tagSource,
					Value: &tagSourceValue,
				},
				{
					Key:   &tagK3sdeploy,
					Value: &tagTrueValue,
				},
			},
		}

		_, err := client.CreateTags(context.TODO(), tagInput)
		if err != nil {
			log.Fatalf("failed to tag instance, %v", err)
		}

		log.Printf("Tagged instnace: %q\n", *v.InstanceId)

	}
}

// createSGRules
func createSGRules(client *ec2.Client, id string) {
	// ingress

	// ingress rules
	proto := "TCP"

	cidrs := []string{"10.0.0.0/8", "10.0.0.0/8", "10.0.0.0/8", "0.0.0.0/0"}
	beginPorts := []int32{6443, 2379, 10250, 22}
	endPorts := []int32{6443, 2380, 10250, 22}

	for i, _ := range cidrs {
		sgIngressInput := &ec2.AuthorizeSecurityGroupIngressInput{
			CidrIp:     &cidrs[i],
			FromPort:   &beginPorts[i],
			GroupId:    &id,
			IpProtocol: &proto,
			ToPort:     &endPorts[i],
		}

		_, err := client.AuthorizeSecurityGroupIngress(context.TODO(), sgIngressInput)
		if err != nil {
			log.Fatalf("failed to create ingress security group rule, %v", err)
		}
	}

	// egress
	proto = "TCP"

	cidrs = []string{"0.0.0.0/0"}
	beginPorts = []int32{0}
	endPorts = []int32{65535}

	for i, _ := range cidrs {
		sgEgressInput := &ec2.AuthorizeSecurityGroupEgressInput{
			GroupId: &id,
			IpPermissions: []types.IpPermission{
				{
					FromPort:   &beginPorts[i],
					IpProtocol: &proto,
					IpRanges: []types.IpRange{
						{
							CidrIp: &cidrs[i],
						},
					},
					ToPort: &endPorts[i],
					//UserIdGroupPairs: []types.UserIdGroupPair{
					//	{
					//		GroupId: &id,
					//	},
					//},
				},
			},
		}

		_, err := client.AuthorizeSecurityGroupEgress(context.TODO(), sgEgressInput)
		if err != nil {
			log.Fatalf("failed to create egress security group rule, %v", err)
		}
	}
}

// createSG creates the Security Group with needed SG input and output rules.
func createSG(client *ec2.Client, k3scfg *cfg, vpcID string) []string {
	Description := k3scfg.name + " - SHG"

	// inputs for the SG (not the rules)
	sgInput := &ec2.CreateSecurityGroupInput{
		Description: &Description,
		GroupName:   &k3scfg.name,
		TagSpecifications: []types.TagSpecification{
			{
				ResourceType: types.ResourceType("security-group"),
				Tags: []types.Tag{
					{
						Key:   &tagName,
						Value: &k3scfg.name,
					},
					{
						Key:   &tagSource,
						Value: &tagSourceValue,
					},
					{
						Key:   &tagK3sdeploy,
						Value: &tagTrueValue,
					},
				},
			},
		},
		VpcId: &vpcID,
	}

	// create SG (not rules)
	result, err := client.CreateSecurityGroup(context.TODO(), sgInput)
	if err != nil {
		log.Fatalf("failed to create security group, %v", err)
	}

	log.Printf("Created Security Group with ID: %q\n", *result.GroupId)

	// create SG rules
	createSGRules(client, *result.GroupId)

	log.Printf("Created Security Group ingress and egress rules on for Security Group with ID: %q\n", *result.GroupId)

	// TODO: stop returning one group ID as slice for RunInstancesInput
	return []string{*result.GroupId}
}

// valSubnets validates subnet-ids exist
func valSubnets(client *ec2.Client, k3scfg *cfg) (string, []string) {
	// slice subnets
	subnets := strings.Split(k3scfg.subnets, ",")

	// inputs for describe subnets
	subnetsInput := &ec2.DescribeSubnetsInput{
		SubnetIds: subnets,
	}

	// error if subnet is not found
	result, err := client.DescribeSubnets(context.TODO(), subnetsInput)
	if err != nil {
		log.Fatalf("failed to describe subnet, %v", err)
	}
	for i := 0; i < len(result.Subnets); i++ {
		if i+1 < len(result.Subnets) && *result.Subnets[i].VpcId != *result.Subnets[i+1].VpcId {
			log.Fatalf("Specified subnets %q are not in the same VPC")
		}
	}
	// return only the first VPC id since if subnets are in the same VPC the VPC ids will be the same.
	return *result.Subnets[0].VpcId, subnets
}

// createInstance creates count amount of EC2 instances and attempts to tag them
func createInstance(awscfg aws.Config, k3scfg *cfg) {
	// print creating
	log.Printf("Deploying internal cluster %q with %q instances.\n", k3scfg.name, k3scfg.count)

	// Using the Config value, create the s3 client
	client := ec2.NewFromConfig(awscfg)

	// validate subnet-ids
	vpcID, subnets := valSubnets(client, k3scfg)

	// create SGs for k3s
	// https://rancher.com/docs/k3s/latest/en/installation/installation-requirements/#networking
	id := createSG(client, k3scfg, vpcID)

	// inputs

	// use one for min and max since we want to create one instance at a time in each subnet
	one := int32(1)
	j := 0

	// loop over instance count and spread instances over the number of subnets provided.
	for i := int32(1); i <= k3scfg.count; i++ {
		runInput := &ec2.RunInstancesInput{
			ImageId:          aws.String(amiID),
			InstanceType:     types.InstanceTypeT2Micro,
			KeyName:          &k3scfg.key,
			MinCount:         &one,
			MaxCount:         &one,
			SecurityGroupIds: id,
			SubnetId:         &subnets[j],
			UserData:         b64(userData),
		}

		// Build the request with its input parameters
		result, err := client.RunInstances(context.TODO(), runInput)
		if err != nil {
			log.Fatalf("failed to create instance, %v", err)
		}

		// tag the instance after creation
		for _, v := range result.Instances {
			log.Printf("Created instnace with ID: %q - PrivateIP: %q\n", *v.InstanceId, *v.PrivateIpAddress)
		}
		tagInstance(client, result.Instances, k3scfg.name)

		// if times iterated through subnets is equal to len of subnets, reset index j
		// so that we can somewhat evenly spread instances to subnets.
		if j+1 == len(subnets) {
			j = 0
		} else {
			j++
		}
	}

}
