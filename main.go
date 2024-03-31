package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	utils "github.com/czarnikhil/RepoDeployer.git/internal"
	"github.com/gin-gonic/gin"
)

type GITURL struct {
	GitURL   string `json:"giturl"`
	Language string `json:"language"`
}

func main() {
	r := gin.Default()
	r.POST("/project", func(c *gin.Context) {
		var gitURL GITURL
		programmingLanguage := "golang"
		repoName, err := utils.GetRepoName(gitURL.GitURL)
		if err != nil {
			fmt.Errorf("Unable to parse git url :%s", gitURL)
		}
		os.Setenv("GIT_REPOSITORY_URL", gitURL.GitURL)
		cmd := exec.Command("sh", "main.sh")
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			log.Printf("Error running main.sh: %v", err)
		}

		session := session.Must(session.NewSessionWithOptions(session.Options{
			Config: aws.Config{
				Region:      aws.String("us-east-1"),
				Credentials: credentials.NewStaticCredentials(os.Getenv("AWS_ACCESS_KEY_ID"), os.Getenv("AWS_SECRET_ACCESS_KEY"), ""),
			},
			SharedConfigState: session.SharedConfigEnable,
		}))
		switch programmingLanguage {
		case "golang":
			if err := utils.BuildProject(repoName, gitURL.GitURL); err != nil {
				log.Printf("Error building Go project: %v", err)
			}
			if err := utils.UploadImage(repoName, session); err != nil {
				log.Printf("Unable to upload image to ECR: %v", err)
			}
		default:
			c.JSON(500, "Only supports golang as of this version")
		}

		url, err := utils.RunTaskDefinition(repoName, repoName, session)
		if err != nil {
			log.Printf("Unable to create task definition: %v", err)
		}
		c.JSON(200, url)
	})
}
