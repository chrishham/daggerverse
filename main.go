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
	ctx context.Context, azureDevopsPat *dagger.Secret, gitUserEmail, gitUserName, jsonPath string) *dagger.Container {
	fmt.Println("\n📁 [SETUP] Preparing environment")
	container := dag.Container().
		// Base container with all the required packages installed
		From("chrishham/ubuntu-24-04-azure:latest").
		WithSecretVariable("AZURE_DEVOPS_PAT", azureDevopsPat).
		// Configure git
		WithExec([]string{"git", "config", "--global", "user.email", gitUserEmail}).
		WithExec([]string{"git", "config", "--global", "user.name", gitUserName}).
		// Disable cache
		WithEnvVariable("CACHEBUSTER", time.Now().String()).
		// Clone AKS repo into the container
		WithExec([]string{"bash", "-c", "git clone https://$AZURE_DEVOPS_PAT@dev.azure.com/chamaletsoschrist/DevOps_Private/_git/AKS"})

	jsonFullPath := "/AKS/settings/" + jsonPath
	// Assign the variables of the jsonPath to a map[string]string
	variables, err := LoadJSONAsEnvMap(ctx, container, jsonFullPath)
	if err != nil {
		panic(err)
	}

	// Clone application's repo and create required folders
	container = container.
		WithExec([]string{"bash", "-c", "git clone https://$AZURE_DEVOPS_PAT@dev.azure.com/chamaletsoschrist/" + variables["project"] + "/_git/" + variables["repo"]}).
		WithWorkdir(variables["repo"]).
		WithExec([]string{"mkdir", "-p", variables["aksFolderToCreate"]}).
		WithExec([]string{"mkdir", "-p", variables["aksFilePath"] + "/templates"})

	defaultBranch := getDefaultBranch(ctx, container)
	// fmt.Println("defaultBranch :", defaultBranch)

	fmt.Println("\n🔧 Step 1: Generating ArgoCD parent file")
	parentFile := fmt.Sprintf("/%s/%s/%s-%s.yaml", variables["repo"], variables["aksFolderToCreate"], variables["parentApp"], variables["environment"])
	fmt.Printf("Creating parent application file: %s\n", parentFile)
	container = replaceValuesAndCopyFile(ctx, container, variables, "/AKS/aks_manifests/argocd/parent_file.yaml", parentFile)

	fmt.Println("\n🔧 Step 2: Generating ConfigMap and SecretProviderClass")
	
	container = container.
		WithWorkdir(variables["aksFilePath"] + "/templates").
		WithExec([]string{"python3", "/AKS/python_scripts/configmap_generator.py", "-f", jsonFullPath})
	printStdout(ctx, container)

	container = container.
		WithExec([]string{"python3", "/AKS/python_scripts/generate_secrets_csi.py", "-f", jsonFullPath})
	printStdout(ctx, container)

	fmt.Println("\n🔧 Step 3: Generating Helm manifests")
	container, err = copyHelmFiles(ctx, container, variables)
	if err != nil {
		panic(err)
	}

	fmt.Println("\n🔧 Step 4: Linting Helm manifests")
	helmLintOutput, _ := container.
		WithWorkdir("..").
		WithExec([]string{"helm", "lint", "."}).
		WithExec([]string{"sh", "-c", "helm lint . > /dev/stdout 2>&1"}).
		Stdout(ctx)

	fmt.Println(helmLintOutput)

	fmt.Println("\n🚀 [DEPLOY] Push manifests to repo")
	container = container.WithWorkdir("/" + variables["repo"])
	container = commitAndPush(ctx, container, variables["branch"], defaultBranch, `Adding ConfigMap files for `+variables["parentApp"])

	return container
}
