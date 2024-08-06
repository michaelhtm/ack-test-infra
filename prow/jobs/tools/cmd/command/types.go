package command

type ImagesConfig struct {
	ImageRepo string `yaml:"image_repo"`
	Images    map[string]string `yaml:"images"`
}

type Version struct {
	GoVersion string `yaml:"go_version"`
}