package main

// Params is used to parameterize the deployment, set from custom properties in the manifest
type Params struct {
	Manifests []string `json:"manifests,omitempty" yaml:"manifests,omitempty"`

	Namespace string `json:"namespace,omitempty" yaml:"namespace,omitempty"`

	Deployments  []string `json:"deployments,omitempty" yaml:"deployments,omitempty"`
	Statefulsets []string `json:"statefulsets,omitempty" yaml:"statefulsets,omitempty"`
	Daemonsets   []string `json:"daemonsets,omitempty" yaml:"daemonsets,omitempty"`
	Jobs         []string `json:"jobs,omitempty" yaml:"jobs,omitempty"`

	Placeholders map[string]string `json:"placeholders,omitempty" yaml:"placeholders,omitempty"`

	AwaitZeroReplicas bool `json:"awaitZeroReplicas,omitempty" yaml:"awaitZeroReplicas,omitempty"`

	DryRun bool `json:"dryrun,omitempty" yaml:"dryrun,omitempty"`

	JobTimeoutSeconds int `json:"jobtimeoutseconds,omitempty" yaml:"jobtimeoutseconds,omitempty"`
}

// SetDefaults fills in empty fields with convention-based defaults
func (p *Params) SetDefaults() {
	if len(p.Manifests) == 0 {
		p.Manifests = []string{"kubernetes.yaml"}
	}
}
