package main

import (
	"context"
	"encoding/json"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"io"
	"log"
	"net/http"
)

// createBastionSGRules creates the needed rules on the bastion SG
func createBastionSGRules(client *ec2.Client, id, pubIP string) {

	// ingress rules
	proto := "TCP"

	cidrs := []string{pubIP + "/32"}
	beginPorts := []int32{22}
	endPorts := []int32{22}

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
				},
			},
		}

		_, err := client.AuthorizeSecurityGroupEgress(context.TODO(), sgEgressInput)
		if err != nil {
			log.Fatalf("failed to create egress security group rule, %v", err)
		}
	}
}

// getPublicSubnets returns the ID of a public subnet in the VPC
func getPublicSubnets(client *ec2.Client, k3scfg *cfg, vpcID string) (ids []string) {
	// inputs for describe subnets
	var filterState = "state"
	var filterVPC = "vpc-id"

	subnetsInput := &ec2.DescribeSubnetsInput{
		Filters: []types.Filter{
			{
				Name:   &filterState,
				Values: []string{"available"},
			},
			{
				Name:   &filterVPC,
				Values: []string{vpcID},
			},
		},
	}

	// error if subnet is not found
	result, err := client.DescribeSubnets(context.TODO(), subnetsInput)
	if err != nil {
		log.Fatalf("failed to describe subnet, %v", err)
	}
	for _, v := range result.Subnets {
		if *v.MapPublicIpOnLaunch && *v.AvailableIpAddressCount >= 1 {
			ids = append(ids, *v.SubnetId)
		}
	}

	// bail if no subnets map public ips and there isnt an available address
	if len(ids) < 1 {
		log.Fatalf("unable to determine public subnet to use for bastion, %v", err)
	}

	return ids
}

// getIP returns public IP address as string
func getIP() string {
	r, err := http.Get("http://ip-api.com/json/")
	if err != nil {
		log.Fatalf("failed to lookup IP address, %v", err)
	}
	defer r.Body.Close()

	body, err := io.ReadAll(r.Body)
	if err != nil {
		log.Fatalf("failed to read response body, %v", err)
	}

	ip := struct {
		Query string
	}{}
	err = json.Unmarshal(body, &ip)
	if err != nil {
		log.Fatalf("failed to unmarshal IP address, %v", err)
	}

	return ip.Query
}

// createInstance creates count amount of EC2 instances and attempts to tag them
func createBastion(client *ec2.Client, k3scfg *cfg, vpcID string) {
	// print creating
	log.Printf("Creating bastion node %q for cluster %q.\n", k3scfg.clusterName+"-bastion", k3scfg.clusterName)

	// validate subnet-ids
	idsBastion := getPublicSubnets(client, k3scfg, vpcID)

	// create SGs for bastion
	idSG := createSG(client, k3scfg.clusterName, k3scfg.clusterName+"-bastion", vpcID)

	// get local public IP for SSH in bastion SG rule
	pubIP := getIP()

	// create SG rules for bastion
	createBastionSGRules(client, idSG, pubIP)

	// inputs
	// use one for min and max since we want to create one instance at a time in each subnet
	one := int32(1)

	runInput := &ec2.RunInstancesInput{
		ImageId:          &amiID,
		InstanceType:     types.InstanceTypeT2Micro,
		KeyName:          &k3scfg.key,
		MinCount:         &one,
		MaxCount:         &one,
		SecurityGroupIds: []string{idSG},
		SubnetId:         &idsBastion[0],
	}

	// Build the request with its input parameters
	result, err := client.RunInstances(context.TODO(), runInput)
	if err != nil {
		log.Fatalf("failed to create instance, %v", err)
	}

	// tag the instance after creation
	for _, v := range result.Instances {
		//fmt.Printf("%#v\n", v)
		log.Printf("Created bastion instance with ID: %q - PublicIP: %q\n", *v.InstanceId, *v.PublicDnsName)
	}
	tagInstance(client, result.Instances, k3scfg.clusterName, k3scfg.clusterName+"-bastion")
}
