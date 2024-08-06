package command

import (
	"fmt"
	"os/exec"

	"context"
	"log"
	"os"
	"sort"
	"strings"

	"github.com/aquasecurity/go-version/pkg/semver"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ecrpublic"
	"github.com/aws/aws-sdk-go-v2/service/ecrpublic/types"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)


var (
	optRepositoryName string
)

var buildProwCmd = &cobra.Command{
	Use: "build-prow-images",
	Short: "build-prow-images - builds prow images in image_config.yaml and pushes them to ack-infra public ecr",
	RunE: buildProwImages,
}

func init() {
	buildProwCmd.PersistentFlags().StringVar(
		&optRepositoryName, "repository-name", "prow", "account number where images will be pushed",
	)

	rootCmd.AddCommand(buildProwCmd)
}


func kanikoExecutor(version, dockerfile, destination string) {
	// BuildImage("my-app", "my-app-0.0.9")
	app := "/kaniko/executor"
	args := []string {
		"--dockerfile",
	 	dockerfile,
		"--destination",
		destination+version,
		"--context",
		"git://github.com/michaelhtm/aws-docker.git",
		"--cleanup",
		"true",
	}

	cmd := exec.Command(app, args...)
	stdout, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Println(string(stdout))
		fmt.Println(err.Error())
		return
	}
}

func checkError(err error) {
	if err != nil {
		fmt.Println(err.Error())
		log.Fatal(err)
	}
}

func retrieveImageTags() (imageTags [][]string) {
	cfg, err := config.LoadDefaultConfig(context.TODO(), config.WithRegion("us-east-1"))
	if err != nil {
		log.Fatalf("failed to load config, %v", err)
		return
	}

	svc := ecrpublic.NewFromConfig(cfg)

	// Describe those images
	describeImagesInput := &ecrpublic.DescribeImagesInput{
		RepositoryName: aws.String("prow"),
		// MaxResults:     aws.Int32(1000),
		RegistryId: aws.String("399481058530"),
	}

	var token string = "temporaryToken"

	describeImagesOutput := &ecrpublic.DescribeImagesOutput{
		NextToken: &token, // initialize to Empty String for now
	}

	imageDetails := make([]types.ImageDetail, 0, 120)

	for describeImagesOutput.NextToken != nil {
		describeImagesOutput, err = svc.DescribeImages(context.TODO(), describeImagesInput)
		if err != nil {
			log.Fatalf("failed to describe images, %v", err)
			return
		}

		imageDetails = append(imageDetails, describeImagesOutput.ImageDetails...)

		describeImagesInput.NextToken = describeImagesOutput.NextToken
	}

	// sort them from oldest to newest
	sort.Slice(imageDetails, func(i, j int) bool {
		return imageDetails[i].ImagePushedAt.Unix() < imageDetails[j].ImagePushedAt.Unix()
	})

	// version_length := len(describeImagesOutput.ImageDetails)

	// fmt.Println(describeImagesOutput.ImageDetails[version_length-1].ImageTags)

	imageTags = make([][]string, 0, 120)

	for _, imageTag := range imageDetails {
		imageTags = append(imageTags, imageTag.ImageTags)
	}

	return imageTags

}

func compareImageTags(desiredImageVersion string, imageVersions [][]string) (highestVersion string, needUpgrade bool) {

	desiredImageVersionTemp := strings.Split(desiredImageVersion, "-")
	imageName := strings.Join(desiredImageVersionTemp[:len(desiredImageVersionTemp)-1], "-")

	desiredVersion, err := semver.Parse(desiredImageVersionTemp[len(desiredImageVersionTemp)-1])
	checkError(err)

	images := make([]string, 0, 100)

	for _, imageTags := range imageVersions {

		for _, imageTag := range imageTags {
			// we don't want the version with v on it
			if strings.Contains(imageTag, "v") {
				continue
			}
			if strings.Contains(imageTag, imageName) {
				imageVersion := strings.Split(imageTag, "-")[len(strings.Split(imageTag, "-"))-1]
				checkError(err)
				images = append(images, imageVersion)
			}
		}
	}

	versions := make([]semver.Version, 0, 100)
	// fmt.Println(images)
	for _, image := range images {
		version, err := semver.Parse(image)
		checkError(err)
		versions = append(versions, version)
	}

	sort.Sort(semver.Collection(versions))

	highestVersionSem := versions[len(versions)-1]

	needUpgrade = desiredVersion.GreaterThan(highestVersionSem)

	if needUpgrade {
		kanikoExecutor(desiredImageVersion, "Dockerfile."+imageName, "399481058530.dkr.ecr.us-west-2.amazonaws.com/prow:")
	}
	
	return highestVersionSem.String(), needUpgrade
}

func buildProwImages(cmd *cobra.Command, args []string) error {

	// configure stuff
	imageTags := retrieveImageTags()

	imagesConfigData, err := os.ReadFile("./images_config.yaml") 
	if err != nil {
		log.Fatalf("failed to read file, %v", err)
		return err
	}

	var imagesConfig *ImagesConfig
	if err :=  yaml.Unmarshal(imagesConfigData, &imagesConfig); err != nil {
		return fmt.Errorf("unable to unmarshal %s. %s", imagesConfigData, err)
	}

	desiredImageVersions := make([]string, 0, len(imagesConfig.Images))

	for _, image := range imagesConfig.Images {
		desiredImageVersions = append(desiredImageVersions, image)
	}

	for _, desiredImageVersion := range desiredImageVersions {
		highestVersion, needUpgrade := compareImageTags(desiredImageVersion, imageTags)
		fmt.Println(highestVersion, needUpgrade)
	}

	return nil
}
