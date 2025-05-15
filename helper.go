package main

import (
	"context"
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
			WithExec([]string{"apt-get", "install", "-y", "git", "curl", "jq" ,"wget", "unzip", "python3", "python3-yaml"}).
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
func commitAndPush(container *dagger.Container, branch, defaultBranch, commitMsg string) *dagger.Container {
	return container.WithExec([]string{"git", "checkout", branch}).
		WithExec([]string{"git", "pull", "origin", branch}).
		WithExec([]string{"git", "add", "-A", "."}).
		WithExec([]string{"bash", "-c", `
if [[ -n "$(git status -s)" ]]; then
  git commit -m "` + commitMsg + `"
  git push origin ` + defaultBranch + `
else
  echo "No changes to commit"
fi`})
}

func replaceValuesAndCopyFile(ctx context.Context, container *dagger.Container, variables map[string]string, srcFile, destFile string) *dagger.Container {
	fileContent, _ := container.WithExec([]string{"cat", srcFile}).Stdout(ctx)

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

// func (m *Daggerverse) Example(name string) *dagger.K3S {
// 	return dag.
// 		K3S(name)
// }

// starts a k3s server and deploys a helm chart
func (m *Daggerverse) K3S(ctx context.Context) (string, error) {
	k3s := dag.K3S("test")
	kServer := k3s.Server()

	kServer, err := kServer.Start(ctx)
	if err != nil {
		return "", err
	}

	ep, err := kServer.Endpoint(ctx, dagger.ServiceEndpointOpts{Port: 80, Scheme: "http"})
	if err != nil {
		return "", err
	}

	return dag.Container().From("alpine/helm").
		WithExec([]string{"apk", "add", "kubectl"}).
		WithEnvVariable("KUBECONFIG", "/.kube/config").
		WithFile("/.kube/config", k3s.Config(true)).
		WithExec([]string{"helm", "upgrade", "--install", "--force", "--wait", "--debug", "nginx", "oci://registry-1.docker.io/bitnamicharts/nginx"}).
		WithExec([]string{"sh", "-c", "while true; do curl -sS " + ep + " && exit 0 || sleep 1; done"}).Stdout(ctx)

}
