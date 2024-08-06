package command

import (
	"os"

	"github.com/spf13/cobra"
)

const (
	appName      = "ack-build-tools"
	appShortDesc = "prow-patcher - patch prow images, build, and release"
	appLongDesc  = `prow-patcher
	
	A tool to patch prow jobs when there is a change to test infra, or when there's a new go version pushed to ECR`
)

var (
	optImagesConfigPath string
)

var rootCmd = &cobra.Command{
	Use:          appName,
	Short:        appShortDesc,
	Long:         appLongDesc,
	SilenceUsage: true,
}

func init() {
	rootCmd.PersistentFlags().StringVar(
		&optImagesConfigPath, "images-config-path", "../../../buildConfig.yaml", "path to buildConfig.yaml, where all the build versions are stored",
	)
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
