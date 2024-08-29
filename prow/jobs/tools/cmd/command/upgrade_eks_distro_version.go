package command

import (
	"fmt"
	"log"
	"strings"

	"github.com/spf13/cobra"
)

var upgradeEksDistroCMD = &cobra.Command{
	Use:   "upgrade-eks-distro-version",
	Short: "upgrade-eks-distro-version - queries ecr for latest eks-distro version and patches prow images if there's an update",
	RunE:  runUpgradeEksDistro,
}

func init() {
	upgradeEksDistroCMD.PersistentFlags().StringVar(
		&OptBuildConfigPath, "build-config-path", "./build_config.yaml", "path to build_config.yaml, where all the build versions are stored",
	)
	rootCmd.AddCommand(upgradeEksDistroCMD)
}

// runUpgradeEksDistro command queries ECR for the latest eks-distro version
// if the one in build_config.yaml is outdated we use the latest version,
// and patch all prow images in images_config.yaml
func runUpgradeEksDistro(cmd *cobra.Command, args []string) error {

	log.SetPrefix("upgrade-eks-distro-version: ")

	buildConfigData, err := readBuildConfigFile(OptBuildConfigPath)
	if err != nil {
		return err
	}
	currentEksDistroVersion := buildConfigData.EksDistro.CurrentVersion
	eksDistroRepository := buildConfigData.EksDistro.Repository

	log.Printf("Build Config EKS Distro version: %s\n", currentEksDistroVersion)

	ecrEksDistroVersions, err := listRepositoryTags(eksDistroRepository)
	if err != nil {
		return fmt.Errorf("unable to get eks-distro versions from %s: %v", eksDistroRepository, err)
	}
	log.Printf("Successfully listed eks-distro versions from %s", eksDistroRepository)

	latestEcrEksDistroVersion, err := findHighestEcrEksDistroVersion(ecrEksDistroVersions)
	if err != nil {
		return err
	}
	log.Printf("Highest EKS Distro version: %s\n", latestEcrEksDistroVersion)

	needUpgrade := eksDistroVersionIsGreaterThan(latestEcrEksDistroVersion, currentEksDistroVersion)
	if !needUpgrade {
		log.Println("eks-distro version is up-to-date")
		return nil
	}

	log.Printf("Updating eks-distro version to %s\n", latestEcrEksDistroVersion)
	buildConfigData.EksDistro.CurrentVersion = latestEcrEksDistroVersion
	if err = patchBuildVersionFile(buildConfigData, OptBuildConfigPath); err != nil {
		return err
	}
	log.Printf("Successfully updated eks-distro version in build_config")

	commitBranch := fmt.Sprintf(updateEksDistroPRCommitBranch, latestEcrEksDistroVersion)
	prSubject := fmt.Sprintf(updateEksDistroPRSubject, latestEcrEksDistroVersion)
	prDescription := fmt.Sprintf(updateEksDistroPRDescription, currentEksDistroVersion, latestEcrEksDistroVersion)

	log.Println("Commiting changes and creating PR")
	err = commitAndSendPR(OptSourceOwner, OptSourceRepo, commitBranch, updateEksDistroSourceFiles, baseBranch, prSubject, prDescription)
	if err != nil && !strings.Contains(err.Error(), "pull request already exists") {
		return err
	}
	return nil
}
