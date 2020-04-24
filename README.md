# estafette-extension-gke-yaml
This extension allows to deploy plain kubernetes manifests substituting envvars to GKE clusters

```yaml
deploy:
  image: extensions/gke-yaml:stable
  credentials: gke-tooling
  namespace: mynamespace
  manifests:
  - kubernetes.yaml
  placeholders:
    APP_NAME: ${ESTAFETTE_LABEL_APP}
    VERSION: ${ESTAFETTE_BUILD_VERSION}
  deployments:
  - mydeployment
  statefulsets:
  - mystatefulset
  deamonsets:
  - mydeamonset
  dryrun: true
```