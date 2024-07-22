package main

import (
	"fmt"
	"github.com/go-git/go-git/v5"
)

type APIVersionManager struct {
	// The current version of the API
	currentVersion string

	// The directory where the API versions are stored
	apiDir string

	// The Git repository where the API versions are stored
	repo *git.Repository
}

func main() {
	
	fmt.Println("Hello World!")
}