package utils

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"net/url"
	"os/exec"
	"path"

	"path/filepath"
	"strings"

	"github.com/aws/aws-sdk-go/aws"

	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ecrpublic"
	"github.com/aws/aws-sdk-go/service/ecs"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/registry"
	"github.com/docker/docker/client"
)

func BuildProject(repoName string, gitRepoURL string) error {
	cmd := exec.Command("docker", "build", "--build-arg", fmt.Sprintf("GIT_REPOSITORY_URL=%s", gitRepoURL), "-t", repoName, ".")
	cmd.Dir = filepath.Join(".", "docker-files", "golang")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("build command execution error: %v\n%s", err, stderr.String())
	}
	return nil
}

func UploadImage(dockerImage string, sess *session.Session) error {

	ecr := ecrpublic.New(sess)

	input := &ecrpublic.GetAuthorizationTokenInput{}
	output, err := ecr.GetAuthorizationTokenWithContext(context.Background(), input)
	if err != nil {
		log.Printf("Error getting authorization token from ECR Public: %v", err)
		return err
	}

	authData := output.AuthorizationData
	if authData == nil {
		log.Println("No authorization data found")
		return nil
	}

	token := authData.AuthorizationToken
	decodedToken, err := base64.StdEncoding.DecodeString(*token)
	if err != nil {
		return err
	}

	cli, err := client.NewClientWithOpts(client.FromEnv)
	if err != nil {
		log.Printf("Error creating Docker client: %v", err)
		return err
	}

	authConfig := registry.AuthConfig{
		Username:      "AWS",
		Password:      strings.Split(string(decodedToken), ":")[1],
		ServerAddress: "public.ecr.aws/z7u8q2p8",
	}

	authConfigBytes, err := json.Marshal(authConfig)
	if err != nil {
		return err
	}

	authConfigEncoded := base64.URLEncoding.EncodeToString(authConfigBytes)
	if _, err := cli.RegistryLogin(context.Background(), authConfig); err != nil {
		log.Printf("Error logging in to Docker registry: %v", err)
		return err
	}

	log.Println("Docker login successful")

	taggedImage := fmt.Sprintf("public.ecr.aws/z7u8q2p8/nikhil-build-server:%s", dockerImage)
	if err := cli.ImageTag(context.Background(), dockerImage+":latest", taggedImage); err != nil {
		log.Printf("Error tagging Docker image: %v", err)
		return err
	}

	log.Println("Tagging successful")

	pushOpts := types.ImagePushOptions{
		RegistryAuth: authConfigEncoded,
		All:          true,
	}
	pushResp, err := cli.ImagePush(context.Background(), taggedImage, pushOpts)
	if err != nil {
		log.Printf("Error pushing Docker image to ECR Public: %v", err)
		return err
	}
	defer pushResp.Close()

	log.Println("Pushing successful")

	return nil
}

func GetRepoName(gitURL string) (string, error) {
	repoName, err := url.Parse(gitURL)
	if err != nil {
		return "", err
	}
	return strings.ToLower(path.Base(repoName.Path)), nil
}

func RunTaskDefinition(repoName, taskDefinition string, sess *session.Session) (string, error) {

	ecsClient := ecs.New(sess)
	awsvpcConfiguration := &ecs.AwsVpcConfiguration{
		AssignPublicIp: aws.String("ENABLED"),
		Subnets: []*string{
			aws.String("subnet-00eea2a7cb8690fa5"),
			aws.String("subnet-0e4c5a95821911326"),
			aws.String("subnet-0d820b0342b9a5d1c"),
			aws.String("subnet-03116cd1a25440217"),
		},
		SecurityGroups: []*string{
			aws.String("sg-01c3f060df732b415"),
		},
	}

	input := &ecs.RunTaskInput{
		Cluster:        aws.String("build-server"),
		TaskDefinition: aws.String(taskDefinition),
		LaunchType:     aws.String("FARGATE"),
		Count:          aws.Int64(1),
		NetworkConfiguration: &ecs.NetworkConfiguration{
			AwsvpcConfiguration: awsvpcConfiguration,
		},
	}

	result, err := ecsClient.RunTask(input)
	if err != nil {
		return "", fmt.Errorf("failed to run task: %v", err)
	}

	taskID := result.Tasks[0].TaskArn
	describeTasksInput := &ecs.DescribeTasksInput{
		Cluster: aws.String("build-server"),
		Tasks:   []*string{taskID},
	}
	taskDetails, err := ecsClient.DescribeTasks(describeTasksInput)
	if err != nil {
		return "", fmt.Errorf("failed to describe task: %v", err)
	}

	var publicIP string
	var containerPort int32
	for _, container := range taskDetails.Tasks[0].Containers {
		if *container.Name == "your-container-name" { // Replace with your container name
			for _, network := range container.NetworkInterfaces {
				publicIP = *network.PrivateIpv4Address
				break // Assuming only one network interface is relevant
			}
			for _, portMapping := range container.NetworkBindings {
				containerPort = int32(*portMapping.HostPort)
				break // Assuming only one port mapping is relevant
			}
			break
		}
	}

	if publicIP == "" || containerPort == 0 {
		return "", fmt.Errorf("failed to find public IP or container port")
	}

	return fmt.Sprintf("http://%s:%d", publicIP, containerPort), nil
}
