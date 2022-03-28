# k3sdeploy

Quickly and easily deploy an internal development K3s cluster on AWS EC2.

# What is k3sdeploy

The `k3sdeploy` tool creates a kubernetes ([k3s](https://k3s.io/)) cluster on EC2 instances along with a bastion host for accessing the k3s
cluster on private subnets.

# Requirements
- AWS access keys configured locally with EC2 access to create, list, delete, and tag EC2 instances, describe EC2 instances and subnets.
- The specified EC2 private key locally stored.
- One or more existing subnets without auto assigned IPv4 address enabled.
- One or more existing subnets with auto assign IPv4 address enabled.
- [Kubectl](https://kubernetes.io/docs/tasks/tools/) installed.

# How it works
The latest Amazon2 linux AMI is determined and used to create the given number of instances spread over the given number of subnets using the provisioning key specified. A bastion instance is created with an IPv4 address that is used to SSH proxy for configuration of K3s cluster and for SSH tunneling to the cluster main node for `kubectl` commands.

# How to use k3sdeploy
First either download binary or build from source:

- Download the binary from the [release page](https://github.com/zherner/k3sdeploy/releases).

or

- Clone the repo: `git clone https://github.com/zherner/k3sdeploy.git`
- In the repo dir: `make install`
- cd $GOPATH/bin

or

- Clone the repo: `git clone https://github.com/zherner/k3sdeploy.git`
- In the repo dir: `make build`

Then:
- Create cluster: `k3sdeploy -c 3 -n my-k3s-cluster-name -k /path/to/ec2/private/key.pem -s subnet-12345,subnet-45567`
- SSH tunnel via the command given by the tool: `ssh -NT -L 6443:<cluster-main-private-ip>:6443 ec2-user@<bastion-public-ip>`
- Use the cluster: see section below [How to use cluster](#how-to-use-cluster).

# How to use cluster
- Ensure SSH tunnel command is running.
- Either export or specify the kubeconfig file to use: `KUBECONFIG=./k3s_kubeconfig kubectl get ns`

# Cleanup
- Destroy the cluster nodes and related security groups: `cd $GOPATH/bin && k3sdeploy -d my-k3s-cluster-name`
- You will be prompted TWICE before deleting related resources.
