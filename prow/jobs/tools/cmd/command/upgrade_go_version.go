// Copyright Amazon.com Inc. or its affiliates. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License"). You may
// not use this file except in compliance with the License. A copy of the
// License is located at
//
//     http://aws.amazon.com/apache2.0/
//
// or in the "license" file accompanying this file. This file is distributed
// on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either
// express or implied. See the License for the specific language governing
// permissions and limitations under the License.

package command

import (
	"fmt"
	"log"

	"github.com/spf13/cobra"
)

// TODO: need to add more flags to handle making a pull request
var (
	OptBuildConfigPath string
)

var upgradeGoVersionCMD = &cobra.Command{
	Use:   "upgrade-go-version",
	Short: "upgrade-go-version - queries for latest image version and patches prow image versions",
	RunE:  runUpgradeGoVersion,
}

func init() {
	upgradeGoVersionCMD.PersistentFlags().StringVar(
		&OptBuildConfigPath, "build-config-path", "build_config.yaml", "path to build_config.yaml, where all the build versions are stored",
	)
	rootCmd.AddCommand(upgradeGoVersionCMD)
}

// runUpgradeGoVersion command compares the GO version in ECR public to the one in build_config
// If the version in ECR is greater, it increases the patch version of prow images in
// images_config.yaml, updates build_config.yaml with latest version
// and pushes a PR with the updates.
func runUpgradeGoVersion(cmd *cobra.Command, args []string) error {
	log.SetPrefix("upgrade-go-version: ")

	goBuildVersion, err := readBuildConfigFile(OptBuildConfigPath)
	if err != nil {
		return err
	}
	currentGoVersion := goBuildVersion.Go.CurrentVersion
	goRepository := goBuildVersion.Go.Repository
	log.Printf("Successfully extracted build versions from %s\n", OptBuildConfigPath)
	log.Printf("Current version in build config is %s\n", currentGoVersion)

	ecrGoVersions, err := listRepositoryTags(goRepository)
	if err != nil {
		return fmt.Errorf("unable to get go versions from ecr public: %v", err)
	}

	log.Printf("Successfully listed go versions from %s\n", goRepository)

	latestEcrGoVersion, err := findHighestTagVersion(ecrGoVersions)
	if err != nil {
		return fmt.Errorf("%v", err)
	}
	log.Printf("Current highest Go version in %s is %s\n", goRepository, latestEcrGoVersion)

	needUpgrade, err := isGreaterVersion(latestEcrGoVersion, currentGoVersion)
	if err != nil {
		return err
	}

	if !needUpgrade {
		log.Printf("Go version in %s is up-to-date\n", OptBuildConfigPath)
		return nil
	}

	log.Printf("Changing Go build version to %s in %s\n", latestEcrGoVersion, OptBuildConfigPath)
	goBuildVersion.Go.CurrentVersion = latestEcrGoVersion
	if err = patchBuildVersionFile(goBuildVersion, OptBuildConfigPath); err != nil {
		return err
	}
	log.Println("Successfully updated Go version!")

	log.Printf("Patching prow image versions in %s\n", OptImagesConfigPath)
	imagesConfig, err := readCurrentImagesConfig(OptImagesConfigPath)
	if err != nil {
		return err
	}
	if err = increasePatchImageConfig(imagesConfig); err != nil {
		return err
	}
	if err = patchImageConfigVersionFile(imagesConfig, OptImagesConfigPath); err != nil {
		return err
	}
	log.Println("Successfully patched prow image versions!")

	commitBranch := fmt.Sprintf(updateGoPRCommitBranch, latestEcrGoVersion)
	prSubject := fmt.Sprintf(updateGoPRSubject, latestEcrGoVersion)
	prDescription := fmt.Sprintf(updateGoPRDescription, currentGoVersion, latestEcrGoVersion)

	log.Println("Committing and creating PR with changes")
	if err = commitAndSendPR(OptSourceOwner, OptSourceRepo, commitBranch, updateGoSourceFiles, baseBranch, prSubject, prDescription); err != nil {
		return err
	}
	log.Println("Successfully created PR")

	return nil
}
