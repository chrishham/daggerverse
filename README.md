# Dagger CLI
dagger call get-container-from-repo \
  --pat=$PAT \
  --gitUserEmail=$gitUserEmail \
  --gitUserName="$gitUserName" \
  --environment=$environment \
  --project=$project \
  --repo=$repo \
  --branch=$branch \
  --namespace=$namespace \
  --aksFolderToCreate=$aksFolderToCreate \
  --parentApp=$parentApp


# Dagger Shell
get-container-from-repo $PAT $gitUserEmail $gitUserName $environment $project $repo $branch $namespace $aksFolderToCreate $parentApp | terminal 