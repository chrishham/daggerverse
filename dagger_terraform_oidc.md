# Terraform: Cloud-Agnostic Federated Identity Setup (e.g. GitHub → Kubernetes)

# 1. OIDC trust setup - example for Azure (can adapt to AWS/GCP)
resource "azurerm_user_assigned_identity" "github" {
  name                = "github-oidc-identity"
  location            = azurerm_resource_group.rg.location
  resource_group_name = azurerm_resource_group.rg.name
}

resource "azurerm_federated_identity_credential" "github" {
  name                = "github-oidc-cred"
  resource_group_name = azurerm_resource_group.rg.name
  parent_id           = azurerm_user_assigned_identity.github.id
  audience            = ["api://AzureADTokenExchange"]
  issuer              = "https://token.actions.githubusercontent.com"
  subject             = "repo:<OWNER>/<REPO>:ref:refs/heads/main"
}

# 2. Output kubeconfig
output "kubeconfig" {
  value     = azurerm_kubernetes_cluster.aks.kube_config_raw
  sensitive = true
}

# 3. IAM binding for AKS (attach identity to cluster)
resource "azurerm_role_assignment" "aks_uai" {
  scope                = azurerm_kubernetes_cluster.aks.id
  role_definition_name = "Azure Kubernetes Service RBAC Admin"
  principal_id         = azurerm_user_assigned_identity.github.principal_id
}

# Dagger example (in Go)
// dagger/main.go
package main

import (
	"context"
	"log"

	"dagger.io/dagger"
)

func main() {
	ctx := context.Background()
	client, err := dagger.Connect(ctx, dagger.WithLogOutput(log.Writer()))
	if err != nil {
		log.Fatal(err)
	}
	defer client.Close()

	// Use kubeconfig from env or secret manager
	kubeconfig := client.Host().EnvVariable("KUBECONFIG")

	// Run kubectl apply in a container
	_, err = client.Container().From("bitnami/kubectl").
		WithMountedFile("/root/.kube/config", kubeconfig).
		WithExec([]string{"kubectl", "apply", "-f", "deployment.yaml"}).
		Stdout(ctx)

	if err != nil {
		log.Fatal(err)
	}
}

# In your CI (GitHub Actions)
# .github/workflows/deploy.yml
jobs:
  deploy:
    permissions:
      id-token: write
      contents: read
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      - name: Setup Dagger
        uses: dagger/dagger-for-github@v5
      - name: Deploy to AKS
        run: dagger run go run main.go
