package main

import (
	"context"
	"fmt"
	"time"

	"github.com/chrishham/daggerverse/internal/dagger"
)

type Daggerverse struct{}

// Creates and configures Helm charts with CSI driver integration for AKS deployments.
func (m *Daggerverse) CreateHelmManifestsCSI(
	ctx context.Context, azureDevopsPat *dagger.Secret, gitUserEmail, gitUserName, environment, project,
	repo, branch, namespace, aksFolderToCreate, parentApp string) *dagger.Container {

	// Setup repo url for cloning
	repoURL := fmt.Sprintf("https://dev.azure.com/chamaletsoschrist/%s/_git/%s", project, repo)

	// Create the variables map for yaml parameters substitutions
	variables := map[string]string{
		"parentApp":   parentApp,
		"environment": environment,
		"project":     project,
		"repo":        repo,
		"branch":      branch,
		"namespace":   namespace,
		"repoURL":     repoURL,
	}

	// Working container with all the required packages installed
	container := dag.Container().
		From("chrishham/ubuntu-24-04-azure:latest").
		WithSecretVariable("AZURE_DEVOPS_PAT", azureDevopsPat).
		// Configure git
		WithExec([]string{"git", "config", "--global", "user.email", gitUserEmail}).
		WithExec([]string{"git", "config", "--global", "user.name", gitUserName}).
		// Disable cache
		WithEnvVariable("CACHEBUSTER", time.Now().String()).
		// Clone repos into the container
		WithExec([]string{"bash", "-c", "git clone https://$AZURE_DEVOPS_PAT@dev.azure.com/chamaletsoschrist/DevOps_Private/_git/AKS"}).
		WithExec([]string{"bash", "-c", "git clone https://$AZURE_DEVOPS_PAT@dev.azure.com/chamaletsoschrist/" + project + "/_git/" + repo}).
		// Cd to repo and create required folders
		WithWorkdir(repo).
		WithExec([]string{"mkdir", "-p", aksFolderToCreate + "/templates"})

	defaultBranch := getDefaultBranch(ctx, container)
	fmt.Println("defaultBranch :", defaultBranch)

	// Create argocd parent file
	parentFile := fmt.Sprintf("/%s/%s/%s-%s.yaml", repo, aksFolderToCreate, parentApp, environment)
	fmt.Printf("Creating parent application file: %s", parentFile)
	container = replaceValuesAndCopyFile(ctx, container, variables, "/AKS/01_parent_file.yaml", parentFile)
	container = commitAndPush(container, branch, defaultBranch, `Adding application file for `+parentApp)

	// Generate the config maps from the json file
	container = container.
		WithWorkdir(aksFolderToCreate + "/templates").
		WithExec([]string{"python3", "/AKS/python_scripts/configmap_generator.py", "-f", "/AKS/settings/qa/api/cbs-transactions-extra-api.json"})

	container = commitAndPush(container, branch, defaultBranch, `Adding ConfigMap files for `+parentApp)

	return container
}
