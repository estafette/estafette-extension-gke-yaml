package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"runtime"

	"github.com/alecthomas/kingpin"
	foundation "github.com/estafette/estafette-foundation"
	"github.com/rs/zerolog/log"
	"gopkg.in/yaml.v2"
)

var (
	appgroup  string
	app       string
	version   string
	branch    string
	revision  string
	buildDate string
	goVersion = runtime.Version()
)

var (
	// flags
	paramsJSON      = kingpin.Flag("params", "Extension parameters, created from custom properties.").Envar("ESTAFETTE_EXTENSION_CUSTOM_PROPERTIES").Required().String()
	paramsYAML      = kingpin.Flag("params-yaml", "Extension parameters, created from custom properties.").Envar("ESTAFETTE_EXTENSION_CUSTOM_PROPERTIES_YAML").Required().String()
	credentialsJSON = kingpin.Flag("credentials", "GKE credentials configured at service level, passed in to this trusted extension.").Envar("ESTAFETTE_CREDENTIALS_KUBERNETES_ENGINE").Required().String()

	// optional flags
	// gitSource     = kingpin.Flag("git-source", "Repository source.").Envar("ESTAFETTE_GIT_SOURCE").String()
	// gitOwner      = kingpin.Flag("git-owner", "Repository owner.").Envar("ESTAFETTE_GIT_OWNER").String()
	// gitName       = kingpin.Flag("git-name", "Repository name, used as application name if not passed explicitly and app label not being set.").Envar("ESTAFETTE_GIT_NAME").String()
	// gitBranch     = kingpin.Flag("git-branch", "Repository commit branch.").Envar("ESTAFETTE_GIT_BRANCH").String()
	// gitRevision   = kingpin.Flag("git-revision", "Repository commit revisition.").Envar("ESTAFETTE_GIT_REVISION").String()
	// appLabel      = kingpin.Flag("app-name", "App label, used as application name if not passed explicitly.").Envar("ESTAFETTE_LABEL_APP").String()
	// buildVersion  = kingpin.Flag("build-version", "Version number, used if not passed explicitly.").Envar("ESTAFETTE_BUILD_VERSION").String()
	releaseName = kingpin.Flag("release-name", "Name of the release section, which is used by convention to resolve the credentials.").Envar("ESTAFETTE_RELEASE_NAME").String()
	// releaseAction = kingpin.Flag("release-action", "Name of the release action, to control the type of release.").Envar("ESTAFETTE_RELEASE_ACTION").String()
	// releaseID     = kingpin.Flag("release-id", "ID of the release, to use as a label.").Envar("ESTAFETTE_RELEASE_ID").String()
	// triggeredBy   = kingpin.Flag("triggered-by", "The user id of the person triggering the release.").Envar("ESTAFETTE_TRIGGER_MANUAL_USER_ID").String()

	assistTroubleshootingOnError = false
	paramsForTroubleshooting     = Params{}
)

func main() {

	// parse command line parameters
	kingpin.Parse()

	// init log format from envvar ESTAFETTE_LOG_FORMAT
	foundation.InitLoggingFromEnv(appgroup, app, version, branch, revision, buildDate)

	// create context to cancel commands on sigterm
	ctx := foundation.InitCancellationContext(context.Background())

	log.Info().Msg("Unmarshalling credentials parameter...")
	var credentialsParam CredentialsParam
	err := json.Unmarshal([]byte(*paramsJSON), &credentialsParam)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed unmarshalling credential parameter")
	}

	log.Info().Msg("Setting default for credential parameter...")
	credentialsParam.SetDefaults(*releaseName)

	log.Info().Msg("Validating required credential parameter...")
	valid, errors := credentialsParam.ValidateRequiredProperties()
	if !valid {
		log.Fatal().Msgf("Not all valid fields are set: %v", errors)
	}

	log.Info().Msg("Unmarshalling injected credentials...")
	var credentials []GKECredentials
	err = json.Unmarshal([]byte(*credentialsJSON), &credentials)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed unmarshalling injected credentials")
	}

	log.Info().Msgf("Checking if credential %v exists...", credentialsParam.Credentials)
	credential := GetCredentialsByName(credentials, credentialsParam.Credentials)
	if credential == nil {
		log.Fatal().Msgf("Credential with name %v does not exist.", credentialsParam.Credentials)
	}

	var params Params
	if credential.AdditionalProperties.Defaults != nil {
		log.Info().Msgf("Using defaults from credential %v...", credentialsParam.Credentials)
		// todo log just the specified defaults, not the entire parms object
		// defaultsAsYAML, err := yaml.Marshal(credential.AdditionalProperties.Defaults)
		// if err == nil {
		// 	log.Printf(string(defaultsAsYAML))
		// }
		params = *credential.AdditionalProperties.Defaults
	}

	log.Info().Msg("Unmarshalling parameters / custom properties...")
	err = yaml.Unmarshal([]byte(*paramsYAML), &params)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed unmarshalling parameters")
	}

	log.Info().Msg("Setting defaults for parameters that are not set in the manifest...")
	params.SetDefaults()

	log.Info().Msg("Retrieving service account email from credentials...")
	var keyFileMap map[string]interface{}
	err = json.Unmarshal([]byte(credential.AdditionalProperties.ServiceAccountKeyfile), &keyFileMap)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed unmarshalling service account keyfile")
	}
	var saClientEmail string
	if saClientEmailIntfc, ok := keyFileMap["client_email"]; !ok {
		log.Fatal().Msg("Field client_email missing from service account keyfile")
	} else {
		if t, aok := saClientEmailIntfc.(string); !aok {
			log.Fatal().Msg("Field client_email not of type string")
		} else {
			saClientEmail = t
		}
	}

	log.Info().Msgf("Storing gke credential %v on disk...", credentialsParam.Credentials)
	err = ioutil.WriteFile("/key-file.json", []byte(credential.AdditionalProperties.ServiceAccountKeyfile), 0600)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed writing service account keyfile")
	}

	log.Info().Msg("Authenticating to google cloud")
	foundation.RunCommandWithArgs(ctx, "gcloud", []string{"auth", "activate-service-account", saClientEmail, "--key-file", "/key-file.json"})

	log.Info().Msgf("Setting gcloud account to %v", saClientEmail)
	foundation.RunCommandWithArgs(ctx, "gcloud", []string{"config", "set", "account", saClientEmail})

	log.Info().Msg("Setting gcloud project")
	foundation.RunCommandWithArgs(ctx, "gcloud", []string{"config", "set", "project", credential.AdditionalProperties.Project})

	log.Info().Msgf("Getting gke credentials for cluster %v", credential.AdditionalProperties.Cluster)
	clustersGetCredentialsArsgs := []string{"container", "clusters", "get-credentials", credential.AdditionalProperties.Cluster}
	if credential.AdditionalProperties.Zone != "" {
		clustersGetCredentialsArsgs = append(clustersGetCredentialsArsgs, "--zone", credential.AdditionalProperties.Zone)
	} else if credential.AdditionalProperties.Region != "" {
		clustersGetCredentialsArsgs = append(clustersGetCredentialsArsgs, "--region", credential.AdditionalProperties.Region)
	} else {
		log.Fatal().Msg("Credentials have no zone or region; at least one of them has to be defined")
	}
	foundation.RunCommandWithArgs(ctx, "gcloud", clustersGetCredentialsArsgs)

	// create 'rendered' directory
	renderedDir, err := ioutil.TempDir("", "rendered-*")
	if err != nil {
		log.Fatal().Err(err).Msg("Failed creating a temporary directory for rendered manifests")
	}

	defer os.RemoveAll(renderedDir)

	// check if manifests exists
	for _, m := range params.Manifests {
		if _, err := os.Stat(m); os.IsNotExist(err) {
			log.Fatal().Msgf("Manifest %v does not exist", m)
		}

		// read file
		manifestContent, err := ioutil.ReadFile(m)
		if err != nil {
			log.Fatal().Err(err).Msgf("Can't read manifest %v", m)
		}

		// 'render' with placeholders
		renderedManifestContent := os.Expand(string(manifestContent), func(placeholderName string) string {

			if placeholderValue, ok := params.Placeholders[placeholderName]; ok {
				return placeholderValue
			}

			return fmt.Sprintf("${%v}", placeholderName)
		})

		// store rendered manifest
		err = ioutil.WriteFile(filepath.Join(renderedDir, m), []byte(renderedManifestContent), 0666)
		if err != nil {
			log.Fatal().Err(err).Msgf("Failed writing manifest to '%v'", filepath.Join(renderedDir, m))
		}
	}

	// dry-run manifests
	for _, m := range params.Manifests {
		kubectlApplyArgs := []string{"apply", "-f", filepath.Join(renderedDir, m), "-n", params.Namespace}

		// always perform a dryrun to ensure we're not ending up in a semi broken state where half of the templates is successfully applied and others not
		log.Info().Msgf("Performing a dryrun to test the validity of manifest '%v'...", m)
		foundation.RunCommandWithArgs(ctx, "kubectl", append(kubectlApplyArgs, "--dry-run"))
	}

	if !params.DryRun {

		// apply manifests
		for _, m := range params.Manifests {
			kubectlApplyArgs := []string{"apply", "-f", filepath.Join(renderedDir, m), "-n", params.Namespace}

			// always perform a dryrun to ensure we're not ending up in a semi broken state where half of the templates is successfully applied and others not
			log.Info().Msgf("Performing a dryrun to test the validity of manifest '%v'...", m)
			foundation.RunCommandWithArgs(ctx, "kubectl", append(kubectlApplyArgs, "--dry-run"))
		}

		for _, deploy := range params.Deployments {
			log.Info().Msgf("Waiting for deployment '%v' to finish...", deploy)
			err = foundation.RunCommandWithArgsExtended(ctx, "kubectl", []string{"rollout", "status", "deployment", deploy, "-n", params.Namespace})
		}

		for _, sts := range params.Statefulsets {
			log.Info().Msgf("Waiting for statefulset '%v' to finish...", sts)
			err = foundation.RunCommandWithArgsExtended(ctx, "kubectl", []string{"rollout", "status", "statefulset", sts, "-n", params.Namespace})
		}

		for _, ds := range params.Daemonsets {
			log.Info().Msgf("Waiting for daemonsets '%v' to finish...", ds)
			err = foundation.RunCommandWithArgsExtended(ctx, "kubectl", []string{"rollout", "status", "daemonsets", ds, "-n", params.Namespace})
		}
	}
}
