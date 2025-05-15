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
	ctx context.Context, azureDevopsPat *dagger.Secret,
	gitUserEmail, gitUserName, environment, project, repo, appName,
	branch, namespace, aksFolderToCreate, aksFilePath, parentApp string) *dagger.Container {

	// Repo url
	repoURL := fmt.Sprintf("https://dev.azure.com/chamaletsoschrist/%s/_git/%s", project, repo)

	// Create the variables map for yaml parameters substitutions
	variables := map[string]string{
		"parentApp":   parentApp,
		"environment": environment,
		"project":     project,
		"repo":        repo,
		"appName":     appName,
		"branch":      branch,
		"namespace":   namespace,
		"repoURL":     repoURL,
	}

	container := dag.Container().
		// Base container with all the required packages installed
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
		WithExec([]string{"mkdir", "-p", aksFolderToCreate}).
		WithExec([]string{"mkdir", "-p", aksFilePath + "/templates"})

	defaultBranch := getDefaultBranch(ctx, container)
	fmt.Println("defaultBranch :", defaultBranch)

	// Create argocd parent file
	parentFile := fmt.Sprintf("/%s/%s/%s-%s.yaml", repo, aksFolderToCreate, parentApp, environment)
	fmt.Printf("Creating parent application file: %s\n", parentFile)
	container = replaceValuesAndCopyFile(ctx, container, variables, "/AKS/01_parent_file.yaml", parentFile)

	// Generate the ConfigMap and SecretProviderClass yaml files from the json file
	container = container.
		WithWorkdir(aksFilePath + "/templates").
		WithExec([]string{"python3", "/AKS/python_scripts/configmap_generator.py", "-f", "/AKS/settings/qa/api/cbs-transactions-extra-api.json"}).
		WithExec([]string{"python3", "/AKS/python_scripts/generate_secrets_csi.py", "-f", "/AKS/settings/qa/api/cbs-transactions-extra-api.json"})

	container = replaceValuesAndCopyFile(ctx, container, variables, "/AKS/03_Chart.yaml", fmt.Sprintf("/%s/%s/Chart.yaml", repo, aksFilePath))
	container = replaceValuesAndCopyFile(ctx, container, variables, "/AKS/04_values.yaml", fmt.Sprintf("/%s/%s/values.yaml", repo, aksFilePath))
	container = replaceValuesAndCopyFile(ctx, container, variables, "/AKS/05_deployment.yaml", fmt.Sprintf("/%s/%s/deployment.yaml", repo, aksFilePath+"/templates"))
	container = replaceValuesAndCopyFile(ctx, container, variables, "/AKS/06_service.yaml", fmt.Sprintf("/%s/%s/service.yaml", repo, aksFilePath+"/templates"))
	container = replaceValuesAndCopyFile(ctx, container, variables, "/AKS/07_virtualservice.yaml", fmt.Sprintf("/%s/%s/virtualservice.yaml", repo, aksFilePath+"/templates"))

	combinedOutput, _ := container.
		WithWorkdir("..").
		WithExec([]string{"helm", "lint", "."}).
		WithExec([]string{"sh", "-c", "helm lint . > /dev/stdout 2>&1"}).
		Stdout(ctx)

	fmt.Println("Helm Lint Full Output:")
	fmt.Println(combinedOutput)

	// Terminal()
	container = container.WithWorkdir("/" + repo)
	container = commitAndPush(container, branch, defaultBranch, `Adding ConfigMap files for `+parentApp)

	return container
}
