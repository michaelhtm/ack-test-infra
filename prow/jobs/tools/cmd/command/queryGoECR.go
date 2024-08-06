package command

import (
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/aquasecurity/go-version/pkg/semver"
	"github.com/aws-controllers-k8s/test-infra/command/ecrpublic"
	"gopkg.in/yaml.v3"
)

var (
	optBuildConfigPath string
	optGoEcrRepository string
)

var queryCmd = &cobra.Command{
	Use:   "upgrade-go-version",
	Short: "upgrade-go-version - queries for latest image version and patches prow image versions",
	RunE:  startQuery,
}

func init() {
	queryCmd.PersistentFlags().StringVar(
		&optBuildConfigPath, "buildConfigPath", "../../../buildConfig.yaml", "path to buildConfig.yaml, where all the build versions are stored",
	)
	queryCmd.PersistentFlags().StringVar(
		&optGoEcrRepository, "golang-ecr-public", "", "path to buildConfig.yaml, where all the build versions are stored",
	)

	rootCmd.AddCommand(queryCmd)
}

// compareVersions() returns true if v1 is greater than v2
func compareVersions(v1, v2 string) bool {
	sem1, err := semver.Parse(v1)
	checkError(err)
	sem2, err := semver.Parse(v2)
	checkError(err)
	return sem1.GreaterThan(sem2)
}

func startQuery(cmd *cobra.Command, args []string) error {

	client := ecrpublic.New()
	tags, err := client.ListRepositoryTags(optGoEcrRepository)
	if err != nil {
		return fmt.Errorf("cannot list repositories in %s. %s", optGoEcrRepository, err)
	}

	versions := make([]semver.Version, 0, len(tags))
	regex, _ := regexp.Compile(`[a-z]`)

	for _, tag := range tags {
		if regex.MatchString(tag) {
			continue
		}
		temp := strings.Split(tag, ".")
		if len(temp) == 3 {
			v, err := semver.Parse(tag)
			if err != nil {
				return fmt.Errorf("error: unable to parse version %s. %s", tag, err)
			}
			versions = append(versions, v)
		}
	}

	sort.Sort(semver.Collection(versions))
	ecrGoVersion := versions[len(versions)-1].String()
	fileData, err := os.ReadFile(optBuildConfigPath)

	if err != nil {
		return fmt.Errorf("unable to read file %s. %s", optBuildConfigPath, err)
	}

	var configGoVersion *Version
	if err := yaml.Unmarshal(fileData, &configGoVersion); err != nil {
		return fmt.Errorf("unable to unmarshal yaml file: %s. %s", fileData, err)
	}

	if compareVersions(ecrGoVersion, configGoVersion.GoVersion) {
		configGoVersion.GoVersion = ecrGoVersion
		file, err := os.Create(optBuildConfigPath)
		if err != nil {
			return fmt.Errorf("file %s creation failed. %s", optBuildConfigPath, err)
		}
		defer file.Close()

		err = yaml.NewEncoder(file).Encode(configGoVersion)
		if err != nil {
			return fmt.Errorf("%s encoding failed. %s", configGoVersion, err)
		}

		imagesConfigData, err := os.ReadFile(optImagesConfigPath)
		if err != nil {
			return fmt.Errorf("unable to read %s. %s", optImagesConfigPath, err)
		}

		var imagesConfig *ImagesConfig
		if err = yaml.Unmarshal(imagesConfigData, &imagesConfig); err != nil {
			return fmt.Errorf("unable to unmarshal imagesConfigData, %s, %s", imagesConfigData, err)
		}

		for _, image := range imagesConfig.Images {
			temp := strings.Split(image, "-")
			version, err := semver.Parse(temp[len(temp)-1])
			checkError(err)
			newVersion := version.IncPatch()
			temp[len(temp)-1] = newVersion.String()
			fmt.Println(strings.Join(temp, "-"))
		}

		makePR()
	}

	return nil
}
