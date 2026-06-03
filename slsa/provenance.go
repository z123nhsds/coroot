package slsa

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"
)

const (
	PredicateTypeSLSAProvenance = "https://slsa.dev/provenance/v1"
	BuildTypeGitHubActions      = "https://slsa.dev/github-actions-buildtype/v1"
)

type ProvenanceStatement struct {
	PredicateType string        `json:"predicateType"`
	Subject       []Resource    `json:"subject"`
	Predicate     Provenance    `json:"predicate"`
}

type Resource struct {
	URI   string       `json:"uri"`
	Digest DigestSet   `json:"digest"`
}

type DigestSet map[string]string

type Provenance struct {
	BuildDefinition BuildDefinition `json:"buildDefinition"`
	RunDetails      RunDetails      `json:"runDetails"`
}

type BuildDefinition struct {
	BuildType            string          `json:"buildType"`
	ExternalParameters   ExternalParams  `json:"externalParameters"`
	InternalParameters   InternalParams  `json:"internalParameters"`
	ResolvedDependencies []ResolvedDep   `json:"resolvedDependencies"`
}

type ExternalParams struct {
	Repository string `json:"repository"`
	Ref        string `json:"ref"`
	Sha        string `json:"sha"`
	Workflow   WorkflowParams `json:"workflow"`
}

type WorkflowParams struct {
	Path      string `json:"path"`
	Reference string `json:"reference"`
}

type InternalParams struct {
	GitHubActor  string `json:"github_actor,omitempty"`
	RunnerOS     string `json:"runner_os,omitempty"`
	RunnerArch   string `json:"runner_arch,omitempty"`
	RunnerEnv    string `json:"runner_environment,omitempty"`
}

type ResolvedDep struct {
	URI    string    `json:"uri"`
	Digest DigestSet `json:"digest,omitempty"`
}

type RunDetails struct {
	Builder   Builder   `json:"builder"`
	BuildMeta BuildMeta `json:"metadata"`
}

type Builder struct {
	ID string `json:"id"`
}

type BuildMeta struct {
	InvocationID string    `json:"invocationID,omitempty"`
	StartedOn    time.Time `json:"startedOn,omitempty"`
	FinishedOn   time.Time `json:"finishedOn,omitempty"`
}

func NewProvenanceStatement(repository, ref, sha string) *ProvenanceStatement {
	return &ProvenanceStatement{
		PredicateType: PredicateTypeSLSAProvenance,
		Subject:       nil,
		Predicate: Provenance{
			BuildDefinition: BuildDefinition{
				BuildType: BuildTypeGitHubActions,
				ExternalParameters: ExternalParams{
					Repository: repository,
					Ref:        ref,
					Sha:        sha,
					Workflow: WorkflowParams{
						Path:      ".github/workflows/release.yml",
						Reference: ref,
					},
				},
				InternalParameters: InternalParams{},
				ResolvedDependencies: []ResolvedDep{
					{
						URI:    fmt.Sprintf("git+%s", repository),
						Digest: DigestSet{"sha1": sha},
					},
				},
			},
			RunDetails: RunDetails{
				Builder: Builder{
					ID: "https://github.com/slsa-framework/slsa-github-generator/.github/workflows/generator_container_slsa3.yml",
				},
			},
		},
	}
}

func (s *ProvenanceStatement) AddSubject(uri, sha256hex string) {
	s.Subject = append(s.Subject, Resource{
		URI:   uri,
		Digest: DigestSet{"sha256": sha256hex},
	})
}

func (s *ProvenanceStatement) SetBuilderID(id string) {
	s.Predicate.RunDetails.Builder.ID = id
}

func (s *ProvenanceStatement) SetInvocationID(id string) {
	s.Predicate.RunDetails.BuildMeta.InvocationID = id
}

func (s *ProvenanceStatement) SetBuildTimes(started, finished time.Time) {
	s.Predicate.RunDetails.BuildMeta.StartedOn = started
	s.Predicate.RunDetails.BuildMeta.FinishedOn = finished
}

func (s *ProvenanceStatement) SetInternalParams(actor, os, arch, env string) {
	s.Predicate.BuildDefinition.InternalParameters = InternalParams{
		GitHubActor: actor,
		RunnerOS:    os,
		RunnerArch:  arch,
		RunnerEnv:   env,
	}
}

func (s *ProvenanceStatement) ToJSON() ([]byte, error) {
	return json.MarshalIndent(s, "", "  ")
}

func SHA256Hex(data []byte) string {
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}
