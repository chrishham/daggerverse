# Build and publish the base Azure image to dockerhub
dagger call create-and-publish-working-image --docker-hub-token=env://DOCKERHUB_TOKEN

# Dagger CLI
```sh
dagger call create-helm-manifests-csi \
  --azureDevopsPat=env://AZURE_DEVOPS_PAT \
  --gitUserEmail=$gitUserEmail \
  --gitUserName="$gitUserName" \
  --environment=$environment \
  --project=$project \
  --repo=$repo \
  --appName=$appName \
  --branch=$branch \
  --namespace=$namespace \
  --aksFolderToCreate=$aksFolderToCreate \
  --aksFilePath=$aksFilePath \
  --parentApp=$parentApp
```

# Dagger Shell
```sh
create-helm-manifests-csi $PAT $gitUserEmail "$gitUserName" $environment $project $repo $branch $namespace $aksFolderToCreate $parentApp | terminal 
```

# TODO

* extract all variables from jsonPath
* handle errors
* pretty format dagger functions output with sections and fmt.println statements
* spin up a k3s cluster and install the helm chart 
helm upgrade --install my-release . --namespace my-namespace --create-namespace


```sh
az login --service-principal -u $AZURE_CLIENT_ID -p $AZURE_CLIENT_SECRET --tenant $AZURE_TENANT_ID
```

az account show --query id -o tsv

az ad sp create-for-rbac --name "my-sp" --role Contributor --scopes /subscriptions/ (output of previous command)

# Get AKS credentials
az aks get-credentials --resource-group $(resourceGroupName) --name $(clusterName) --overwrite-existing

# Try to create the federated credential and capture both stdout and stderr
if OUTPUT=$(az identity federated-credential create \
  --name "$CRED_NAME" \
  --identity-name "$IDENTITY_NAME" \
  --resource-group "$(rg_user_assignment)" \
  --issuer "$ISSUER_URL" \
  --subject "$SUBJECT" 2>&1); then
  
  echo "Successfully created federated credential"

  az aks get-credentials --resource-group $(resourceGroupName) --name $(clusterName) --overwrite-existing
