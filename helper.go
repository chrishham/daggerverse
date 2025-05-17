package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/chrishham/daggerverse/internal/dagger"
)

// Build and publish the multi-platform Azure working image
func (m *Daggerverse) CreateAndPublishWorkingImage(
	ctx context.Context,
	dockerHubToken *dagger.Secret,
) (string, error) {
	// platforms to build for and push in a multi-platform image
	var platforms = []dagger.Platform{
		"linux/amd64", // a.k.a. x86_64
		"linux/arm64", // a.k.a. aarch64
		// "linux/s390x", // a.k.a. IBM S/390
	}

	// container registry for the multi-platform image
	const imageRepo = "docker.io/chrishham/ubuntu-24-04-azure:latest"

	platformVariants := make([]*dagger.Container, 0, len(platforms))
	for _, platform := range platforms {
		fmt.Println("Platform: ", platform)

		ctr := dag.Container(dagger.ContainerOpts{Platform: platform}).
			From("ubuntu:24.04").
			// Install required packages
			WithExec([]string{"apt-get", "update"}).
			WithExec([]string{"apt-get", "install", "-y", "git", "curl", "jq", "wget", "unzip", "python3", "python3-yaml"}).
			// Install Azure CLI, Kubectl, Kubelogin and Helm
			WithExec([]string{"bash", "-c", "curl -sL https://gist.githubusercontent.com/chrishham/283bd8928a42eb5818ede2f8ee4ef6fe/raw/47227f94016fd80b471d94565ce3a8900123541b/gistfile1.txt | bash"})

		platformVariants = append(platformVariants, ctr)
	}

	// publish to registry
	imageDigest, err := dag.Container().
		WithRegistryAuth("docker.io", "chrishham", dockerHubToken).
		Publish(ctx, imageRepo, dagger.ContainerPublishOpts{
			PlatformVariants: platformVariants,
		})

	if err != nil {
		return "", err
	}

	// return build directory
	return imageDigest, nil
}

// Create the working image required for the pipeline to run
func (m *Daggerverse) TestImage(ctx context.Context, dockerHubToken string) string {
	address, err := dag.Container().
		From("ttl.sh/chrishham-3874354@sha256:053f43fb76c77de11b403cb051c8937985a2878f80fdad1acaeb12f1a5766af0").
		// Install required packages
		WithRegistryAuth("docker.io", "chrishham", dag.SetSecret("password", dockerHubToken)).
		Publish(ctx, "docker.io/chrishham/ubuntu-24-04-azure-arm:latest")
		// Publish(ctx, fmt.Sprintf("ttl.sh/chrishham-%.0f", math.Floor(rand.Float64()*10000000)))
	if err != nil {
		panic(err)
	}
	return address
}

// ************************
// Helper functions
// ************************
func commitAndPush(ctx context.Context, container *dagger.Container, branch, defaultBranch, commitMsg string) *dagger.Container {
	container = container.WithExec([]string{"git", "checkout", branch}).
		WithExec([]string{"git", "pull", "origin", branch}).
		WithExec([]string{"git", "add", "-A", "."}).
		WithExec([]string{"bash", "-c", `
if [[ -n "$(git status -s)" ]]; then
  git commit -m "` + commitMsg + `"
  git push origin ` + defaultBranch + `
else
  echo "No changes to commit"
fi`})
	printStdout(ctx, container)

	return container
}

func replaceValuesAndCopyFile(ctx context.Context, container *dagger.Container, variables map[string]string, srcFile, destFile string) *dagger.Container {
	fileContent, _ := container.File(srcFile).Contents(ctx)
	fmt.Println("Copying ", srcFile)
	for key, value := range variables {
		fileContent = strings.ReplaceAll(fileContent, "$("+key+")", value)
	}

	return container.
		WithNewFile(destFile, fileContent)
}

func getDefaultBranch(ctx context.Context, container *dagger.Container) string {
	defaultBranch, _ := container.
		WithExec([]string{
			"bash", "-c",
			"git remote show origin | grep 'HEAD branch' | cut -d' ' -f5",
		}).Stdout(ctx)

	return defaultBranch
}

// LoadJSONAsEnvMap reads a JSON file from a container path and returns a map[string]string
func LoadJSONAsEnvMap(ctx context.Context, container *dagger.Container, jsonPath string) (map[string]string, error) {
	// Read contents of the JSON file inside the container
	contents, err := container.File(jsonPath).Contents(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to read file %s: %w", jsonPath, err)
	}

	// Parse it as generic map
	var raw map[string]interface{}
	if err := json.Unmarshal([]byte(contents), &raw); err != nil {
		return nil, fmt.Errorf("invalid JSON: %w", err)
	}

	// Normalize values to strings
	values := make(map[string]string)
	for k, v := range raw {
		switch val := v.(type) {
		case string:
			values[k] = val
		default:
			jsonVal, err := json.Marshal(val)
			if err != nil {
				return nil, fmt.Errorf("could not marshal value for key %s: %w", k, err)
			}
			values[k] = string(jsonVal)
		}
	}

	return values, nil
}

func copyHelmFiles(ctx context.Context, container *dagger.Container, variables map[string]string) (*dagger.Container, error) {
	repo := variables["repo"]
	aksPath := variables["aksFilePath"]

	helmBase := "/AKS/aks_manifests/helm"
	files, err := getFilesUnderDir(ctx, container, helmBase)
	if err != nil {
		return nil, err
	}

	for _, relPath := range files {
		src := helmBase + "/" + relPath
		dest := fmt.Sprintf("/%s/%s/%s", repo, aksPath, relPath)
		container = replaceValuesAndCopyFile(ctx, container, variables, src, dest)
	}

	return container, nil
}

// helper to recursively collect all file paths under a dir in a container
func getFilesUnderDir(ctx context.Context, container *dagger.Container, dir string) ([]string, error) {
	fileList, err := container.Directory(dir).Glob(ctx, "**/*")
	if err != nil {
		return nil, fmt.Errorf("failed to list files under %s: %w", dir, err)
	}

	var files []string
	for _, path := range fileList {
		if strings.HasSuffix(path, "/") {
			continue // skip directories
		}

		fullPath := dir + "/" + path
		_, err := container.File(fullPath).Contents(ctx)
		if err == nil {
			files = append(files, path)
		}
	}
	return files, nil
}
func printStdout(ctx context.Context, container *dagger.Container) {
	stdout, _ := container.Stdout(ctx)
	fmt.Println(stdout)
}


