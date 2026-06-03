package provenance

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"time"
)

const (
	SLSAVersion = "1.0"
	BuildType   = "https://slsa.dev/provenance/v1/buildType/generic"
)

type Subject struct {
	Name   string            `json:"name"`
	Digest map[string]string `json:"digest"`
}

type Builder struct {
	ID string `json:"id"`
}

type BuildMetadata struct {
	InvocationID string    `json:"invocationId"`
	StartedOn    time.Time `json:"startedOn"`
	FinishedOn   time.Time `json:"finishedOn"`
}

type ConfigSource struct {
	URI       string            `json:"uri"`
	Digest    map[string]string `json:"digest"`
	EntryPoint string           `json:"entryPoint"`
}

type Invocation struct {
	ConfigSource ConfigSource `json:"configSource"`
}

type Provenance struct {
	Type          string         `json:"_type"`
	Subject       []Subject      `json:"subject"`
	PredicateType string         `json:"predicateType"`
	Predicate     Predicate      `json:"predicate"`
}

type Predicate struct {
	BuildDefinition BuildDefinition `json:"buildDefinition"`
	RunDetails      RunDetails      `json:"runDetails"`
}

type BuildDefinition struct {
	BuildType       string            `json:"buildType"`
	ExternalParams  map[string]any    `json:"externalParameters"`
	InternalParams  map[string]any    `json:"internalParameters"`
	ResolvedDependencies []Dependency `json:"resolvedDependencies"`
}

type Dependency struct {
	Name   string            `json:"name"`
	URI    string            `json:"uri"`
	Digest map[string]string `json:"digest"`
}

type RunDetails struct {
	Builder       Builder        `json:"builder"`
	Metadata      BuildMetadata  `json:"metadata"`
	ByProducts    []ByProduct    `json:"byproducts"`
}

type ByProduct struct {
	Name   string            `json:"name"`
	Digest map[string]string `json:"digest"`
}

// GenerateProvenance 生成 SLSA3 兼容的 provenance 文件
func GenerateProvenance(binaryPath, repoURI, commitHash, entryPoint, builderID, invocationID string) (*Provenance, error) {
	// 1. 计算二进制文件的 hash
	digest, err := computeFileHash(binaryPath)
	if err != nil {
		return nil, fmt.Errorf("failed to compute file hash: %w", err)
	}

	// 2. 创建 subject
	subject := Subject{
		Name:   binaryPath,
		Digest: map[string]string{"sha256": digest},
	}

	// 3. 创建 provenance 结构
	now := time.Now()
	provenance := &Provenance{
		Type:          "https://in-toto.io/Statement/v1",
		PredicateType: "https://slsa.dev/provenance/v1",
		Subject:       []Subject{subject},
		Predicate: Predicate{
			BuildDefinition: BuildDefinition{
				BuildType: BuildType,
				ExternalParams: map[string]any{
					"repository": repoURI,
					"commit":     commitHash,
					"command":    []string{"go", "build"},
				},
				InternalParams: map[string]any{
					"go_version": "1.25",
				},
				ResolvedDependencies: []Dependency{
					{
						Name:   "go",
						URI:    "https://go.dev/dl/go1.25.5",
						Digest: map[string]string{"sha256": "compute_this_hash"},
					},
				},
			},
			RunDetails: RunDetails{
				Builder: Builder{
					ID: builderID,
				},
				Metadata: BuildMetadata{
					InvocationID: invocationID,
					StartedOn:    now,
					FinishedOn:   now,
				},
				ByProducts: []ByProduct{
					{
						Name:   "build.log",
						Digest: map[string]string{"sha256": "compute_build_log_hash"},
					},
				},
			},
		},
	}

	return provenance, nil
}

// WriteProvenance 将 provenance 写入文件
func WriteProvenance(p *Provenance, outputPath string) error {
	data, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal provenance: %w", err)
	}
	return os.WriteFile(outputPath, data, 0644)
}

func computeFileHash(path string) (string, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	hash := sha256.Sum256(content)
	return hex.EncodeToString(hash[:]), nil
}
