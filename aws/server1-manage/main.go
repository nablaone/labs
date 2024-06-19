package main

import (
	_ "embed"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"
)

//go:embed assets/keyId.txt
var keyId string

//go:embed assets/secretKey.txt
var secretKey string

//go:embed assets/instanceId.txt
var instanceId string

func status(svc *ec2.EC2) (string, error) {

	var instances []*string

	instances = append(instances, &instanceId)

	descInstance := &ec2.DescribeInstancesInput{
		InstanceIds: instances,
	}

	result, err := svc.DescribeInstances(descInstance)
	if err != nil {
		return "", err
	} else {
		return *result.Reservations[0].Instances[0].State.Name, nil
	}
}

func start(svc *ec2.EC2) error {

	input := &ec2.StartInstancesInput{
		InstanceIds: []*string{
			&instanceId,
		},
	}
	result, err := svc.StartInstances(input)
	if err != nil {
		log.Println("error while starting instance", err)
		return err
	} else {
		log.Println("Instance started", result.StartingInstances)
	}
	return nil
}

func stop(svc *ec2.EC2) {
	input := &ec2.StopInstancesInput{
		InstanceIds: []*string{
			&instanceId,
		},
	}
	result, err := svc.StopInstances(input)
	if err != nil {
		log.Println("error while stopping instance", err)
	} else {
		fmt.Println("Instance stopped", result.StoppingInstances)
	}

}

func main() {

	instanceId = strings.TrimSpace(instanceId)

	creds := credentials.NewStaticCredentials(strings.TrimSpace(keyId),
		strings.TrimSpace(secretKey),
		"")
	cfg := &aws.Config{
		Region:      aws.String("eu-central-1"),
		Credentials: creds,
	}

	sess, err := session.NewSession(cfg)
	if err != nil {
		panic(err)
	}

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, os.Kill)

	// Create new EC2 client
	svc := ec2.New(sess)

	s, err := status(svc)

	if err != nil {
		log.Println("Error while getting server status: ", err)
		os.Exit(1)
	}

	if s == "running" {
		log.Println("Server is running.")
	} else {

		err = start(svc)
		if err != nil {
			log.Println("Error while starting server:", err)
			os.Exit(2)
		}
	}

LOOP:
	for {

		s, err := status(svc)
		if err != nil {
			log.Println("Error while getting server status:", err)
			os.Exit(1)
		}

		if s == "pending" {
			log.Println("Server is starting ......")
		}

		if s == "running" {
			log.Println("Server is running.")
		}

		select {
		case <-sig:
			log.Println("Terminated")
			break LOOP
		case <-time.After(10 * time.Second):

		}
	}

	stop(svc)

}
