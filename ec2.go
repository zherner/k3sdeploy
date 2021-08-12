package main

import (
	"context"
	"encoding/base64"
	"fmt"
	"log"
	"strconv"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
)

var (
	k3sInstall = `
#!/usr/bin/env bash
curl -sfL https://get.k3s.io`

	//subnet         = "subnet-017584c03579c5d3e"
	amiID               = "ami-0233c2d874b811deb"
	tagName             = "Name"
	tagK3sdeploycluster = "k3sdeploycluster"
	tagSource           = "source"
	tagSourceValue      = "https://github.com/zherner/k3sdeploy"
	tagK3sdeploy        = "k3sdeploy"
	tagTrueValue        = "true"
	instanceID          = "instance-id"
)

// describeInstance returns instance ids created by this tool and associated with the cluster name
func describeInstance(client *ec2.Client, k3scfg *cfg, name, idIn string) (ids []string, state []int32, ipPri, ipPub []string) {

	// filter inputs must be prepended with "tag:"
	var tagK3sdeploycluster = "tag:" + tagK3sdeploycluster
	var tagKey = "tag:" + tagK3sdeploy
	var tagTagName = "tag:" + tagName
	//var idsIn = []string{}

	filters := []types.Filter{
		{
			Name:   &tagK3sdeploycluster,
			Values: []string{k3scfg.clusterName},
		},
		{
			Name:   &tagKey,
			Values: []string{tagTrueValue},
		},
	}

	if name != "" {
		filters = append(filters, types.Filter{
			Name:   &tagTagName,
			Values: []string{k3scfg.clusterName + name},
		})
	}

	if idIn != "" {
		filters = append(filters, types.Filter{
			Name:   &instanceID,
			Values: []string{idIn},
		})
		//idsIn = []string{idIn}
	}

	describeInput := &ec2.DescribeInstancesInput{
		Filters: filters,
		//InstanceIds: idsIn,
	}

	result, err := client.DescribeInstances(context.TODO(), describeInput)
	if err != nil {
		log.Fatalf("failed to describe instance, %v", err)
	}

	// loop over results to get matching instance id in a running state
	// should be only one result if using idIn (instance-id as input)
	for _, v := range result.Reservations {
		for _, k := range v.Instances {
			// 16 - running, 0 - pending, 48 - terminated
			if *k.State.Code == 48 {
				state = append(state, *k.State.Code)
				// skip the rest for intsances already in terminated state
				continue
			}
			state = append(state, *k.State.Code)
			ids = append(ids, *k.InstanceId)
			if k.PrivateIpAddress == nil {
				ipPri = append(ipPri, "")
			} else {
				ipPri = append(ipPri, *k.PrivateIpAddress)
			}
			if *k.PublicDnsName == "" {
				ipPub = append(ipPub, "")
			} else {
				ipPub = append(ipPub, *k.PublicIpAddress)
			}
			//}
		}
	}

	return ids, state, ipPri, ipPub
}

// b64 base64 encodes a string
func b64(str string) *string {
	enc := base64.StdEncoding.EncodeToString([]byte(str))
	return &enc
}

// tagInstance takes a slice of instanceIds and tags them
func tagInstance(client *ec2.Client, instances []types.Instance, clusterName, name string) {

	for _, v := range instances {
		tagInput := &ec2.CreateTagsInput{
			Resources: []string{*v.InstanceId},
			Tags: []types.Tag{
				{
					Key:   &tagName,
					Value: &name,
				},
				{
					Key:   &tagK3sdeploycluster,
					Value: &clusterName,
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

// createSGRules creates the needed rules on the instance SG
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
func createSG(client *ec2.Client, clusterName, name, vpcID string) string {
	sgName := name + "-sg"

	// inputs for the SG (not the rules)
	sgInput := &ec2.CreateSecurityGroupInput{
		Description: &sgName,
		GroupName:   &sgName,
		TagSpecifications: []types.TagSpecification{
			{
				ResourceType: types.ResourceType("security-group"),
				Tags: []types.Tag{
					{
						Key:   &tagName,
						Value: &sgName,
					},
					{
						Key:   &tagK3sdeploycluster,
						Value: &clusterName,
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

	// TODO: stop returning one group ID as slice for RunInstancesInput
	return *result.GroupId
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
func createInstances(awscfg aws.Config, k3scfg *cfg) {
	// for debuggin ssh
	//_ = sshExtractToken(awscfg, k3scfg, "i-05d25082d445d76df", "i-018e690a621877f4d")
	//return

	// print creating
	log.Printf("Deploying internal cluster %q with %d instances.\n", k3scfg.clusterName, k3scfg.count)

	// Using the Config value, create the s3 client
	client := ec2.NewFromConfig(awscfg)

	// validate subnet-ids
	vpcID, subnets := valSubnets(client, k3scfg)

	// create bastion
	idBastion, ipBastion := createBastion(client, k3scfg, vpcID)

	// create SGs for k3s
	// https://rancher.com/docs/k3s/latest/en/installation/installation-requirements/#networking
	idSG := createSG(client, k3scfg.clusterName, k3scfg.clusterName, vpcID)

	// create SG rules for instances
	createSGRules(client, idSG)

	log.Printf("Created Security Group ingress and egress rules on for Security Group with ID: %q\n", idSG)
	// inputs

	// use one for min and max since we want to create one instance at a time in each subnet
	one := int32(1)
	j := 0
	userData := ""
	ipClusterMain := ""
	idClusterMain := ""
	k3sClusterToken := ""

	// loop over instance count and spread instances over the number of subnets provided.
	for i := int32(1); i <= k3scfg.count; i++ {

		// used in tagInstance to add to name a count of instances -01 -02 -03 etc
		nameAppend := "-worker-0" + strconv.Itoa(int(i)-1)

		// tag the first node as -main instead
		if i == 1 {
			nameAppend = "-main"
			userData = k3sInstall + " | sh -"
		} else {
			userData = k3sInstall + " | K3S_URL=https://" + ipClusterMain + ":6443" + " K3S_TOKEN=" + k3sClusterToken + " sh -"
		}

		runInput := &ec2.RunInstancesInput{
			ImageId:          &amiID,
			InstanceType:     types.InstanceTypeT2Micro,
			KeyName:          &k3scfg.key,
			MinCount:         &one,
			MaxCount:         &one,
			SecurityGroupIds: []string{idSG},
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
			if i == 1 {
				ipClusterMain = *v.PrivateIpAddress
				idClusterMain = *v.InstanceId
			}
		}

		tagInstance(client, result.Instances, k3scfg.clusterName, k3scfg.clusterName+nameAppend)

		// extract token after cluster main is created
		if i == 1 {
			k3sClusterToken = sshExtractToken(awscfg, k3scfg, idBastion, idClusterMain)
		}

		// get first (main) instance id
		// if times iterated through subnets is equal to len of subnets, reset index j
		// so that we can somewhat evenly spread instances to subnets.
		if j+1 == len(subnets) {
			j = 0
		} else {
			j++
		}
	}

	fmt.Printf("\n ssh -NT -L 6443:%s:6443 ec2-user@%s\n", ipClusterMain, ipBastion)
}
