package main

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/chrishham/daggerverse/internal/dagger"
)

type Daggerverse struct{}

// Returns a container with the input repo cloned
func (m *Daggerverse) GetContainerFromRepo(
	ctx context.Context,
	pat,
	gitUserEmail,
	gitUserName,
	environment,
	project,
	repo,
	branch,
	namespace,
	aksFolderToCreate,
	parentApp string,
) *dagger.Container {
	repoUrl := fmt.Sprintf("https://%s@dev.azure.com/chamaletsoschrist/%s/_git/%s", pat, project, repo)
	aksYamlRepoUrl := fmt.Sprintf("https://%s@dev.azure.com/chamaletsoschrist/DevOps_Private/_git/AKS", pat)

	repoContainer := dag.Container().
		From("alpine:latest").
		WithExec([]string{"apk", "add", "git"}).
		WithExec([]string{"git", "config", "--global", "user.email", gitUserEmail}).
		WithExec([]string{"git", "config", "--global", "user.name", gitUserName}).
		WithEnvVariable("CACHEBUSTER", time.Now().String()).
		WithExec([]string{"git", "clone", aksYamlRepoUrl}).
		WithExec([]string{"git", "clone", repoUrl}).
		WithWorkdir(repo)

	defaultBranch, _ := repoContainer.
		WithExec([]string{
			"sh", "-c",
			"git remote show origin | grep 'HEAD branch' | cut -d' ' -f5",
		}).Stdout(ctx)
	fmt.Println("defaultBranch :", defaultBranch)
	// Read the original file content
	fileContent, _ := repoContainer.WithExec([]string{"cat", "/AKS/01_parent_file.yaml"}).Stdout(ctx)

	// Replace all variables directly using the function parameters

	variables := map[string]string{
		"parentApp":   parentApp,
		"environment": environment,
		"project":     project,
		"repo":        repo,
		"branch":      branch,
		"namespace":   namespace,
		"repoUrl":     repoUrl,
		// Add any other variables you need to replace
	}

	for key, value := range variables {
		fileContent = strings.ReplaceAll(fileContent, "$("+key+")", value)
	}

	parentFile := fmt.Sprintf("/%s/%s/%s-%s.yaml", repo, aksFolderToCreate, parentApp, environment)
	repoContainer = repoContainer.
		WithExec([]string{"mkdir", "-p", aksFolderToCreate}).
		WithNewFile("/tmp/modified_file.yaml", fileContent).
		WithExec([]string{"cp", "/tmp/modified_file.yaml", parentFile})
		// WithExec([]string{"cp", "/AKS/01_parent_file.yaml", parentFile})

	repoContainer = CommitAndPush(repoContainer, branch, defaultBranch, `Adding application file for `+parentApp)

	return repoContainer
}

func CommitAndPush(container *dagger.Container, branch, defaultBranch, commitMsg string) *dagger.Container {
	return container.WithExec([]string{"git", "checkout", branch}).
		WithExec([]string{"git", "pull", "origin", branch}).
		WithExec([]string{"git", "add", "-A", "."}).
		WithExec([]string{"sh", "-c", `
if [[ -n "$(git status -s)" ]]; then
  git commit -m "` + commitMsg + `"
  git push origin ` + defaultBranch + `
else
  echo "No changes to commit"
fi`})
}

// substituteVars replaces all $(varName) in input with corresponding values from vars map.
func substituteVars(input string, vars map[string]string) string {
	// Regex to match $(varName)
	re := regexp.MustCompile(`\$\(([^)]+)\)`)

	// Replace all matches with corresponding values from map
	return re.ReplaceAllStringFunc(input, func(match string) string {
		key := re.FindStringSubmatch(match)[1]
		if val, ok := vars[key]; ok {
			return val
		}
		return match // leave untouched if not found
	})
}
