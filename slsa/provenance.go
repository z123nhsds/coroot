package slsa

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/sha512"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"time"

	"github.com/coroot/coroot/utils"
)

const (
	PredicateTypeProvenance = "https://slsa.dev/provenance/v1"
	PredicateTypeSPDX       = "https://spdx.dev/Document"
	MediaTypeDSSE           = "application/vnd.dsse.envelope+json"
	MediaTypeInToto         = "application/vnd.in-toto+json"

	SLSABuildL3 = "https://slsa.dev/spec/v1.0/levels#build-l3"
)

type HashAlgorithm string

const (
	HashSHA256 HashAlgorithm = "sha256"
	HashSHA512 HashAlgorithm = "sha512"
)

type Subject struct {
	Name   string            `json:"name"`
	Digest map[string]string `json:"digest"`
}

type ResourceDescriptor struct {
	URI             string            `json:"uri"`
	Digest          map[string]string `json:"digest"`
	DownloadLocation string           `json:"downloadLocation,omitempty"`
	MediaType       string            `json:"mediaType,omitempty"`
	Annotations     map[string]string `json:"annotations,omitempty"`
}

type Builder struct {
	ID             string             `json:"id"`
	Version        map[string]string  `json:"version,omitempty"`
	BuilderDependencies []ResourceDescriptor `json:"builderDependencies,omitempty"`
}

type BuildMetadata struct {
	InvocationID     string            `json:"invocationId,omitempty"`
	StartedOn        string            `json:"startedOn,omitempty"`
	FinishedOn       string            `json:"finishedOn,omitempty"`
	Reproducible     bool              `json:"reproducible,omitempty"`
	Completeness     Completeness      `json:"completeness,omitempty"`
	Annotations      map[string]string `json:"annotations,omitempty"`
}

type Completeness struct {
	Parameters  bool `json:"parameters"`
	Environment bool `json:"environment"`
	Materials   bool `json:"materials"`
}

type ProvenanceRecipe struct {
	Type              string              `json:"type"`
	DefinedInMaterial *int                `json:"definedInMaterial,omitempty"`
	EntryPoint        string              `json:"entryPoint,omitempty"`
	Arguments         json.RawMessage     `json:"arguments,omitempty"`
	Environment       json.RawMessage     `json:"environment,omitempty"`
}

type Provenance struct {
	BuildDefinition `json:"buildDefinition"`
	RunDetails      `json:"runDetails"`
}

type BuildDefinition struct {
	BuildType            string              `json:"buildType"`
	ExternalParameters   json.RawMessage     `json:"externalParameters"`
	InternalParameters   json.RawMessage     `json:"internalParameters,omitempty"`
	ResolvedDependencies []ResourceDescriptor `json:"resolvedDependencies,omitempty"`
}

type RunDetails struct {
	Builder   Builder          `json:"builder"`
	Metadata  *BuildMetadata   `json:"metadata,omitempty"`
	Byproducts []ResourceDescriptor `json:"byproducts,omitempty"`
}

type InTotoStatement struct {
	Type          string     `json:"_type"`
	Subject       []Subject  `json:"subject"`
	PredicateType string     `json:"predicateType"`
	Predicate     json.RawMessage `json:"predicate"`
}

type DSSEEnvelope struct {
	PayloadType string `json:"payloadType"`
	Payload     string `json:"payload"`
	Signatures  []DSSESignature `json:"signatures"`
}

type DSSESignature struct {
	KeyID string `json:"keyid,omitempty"`
	Sig   string `json:"sig"`
}

type ProvenanceGenerator struct {
	BuilderID      string
	BuildType      string
	PrivateKey     *rsa.PrivateKey
	KeyID          string
	Completeness   Completeness
}

func NewProvenanceGenerator(builderID, buildType string, keyPEM []byte) (*ProvenanceGenerator, error) {
	gen := &ProvenanceGenerator{
		BuilderID: builderID,
		BuildType: buildType,
		Completeness: Completeness{
			Parameters:  true,
			Environment: true,
			Materials:   true,
		},
	}

	if len(keyPEM) > 0 {
		block, _ := pem.Decode(keyPEM)
		if block == nil {
			return nil, errors.New("failed to decode PEM block")
		}
		key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
		if err != nil {
			key, err = x509.ParsePKCS1PrivateKey(block.Bytes)
			if err != nil {
				return nil, fmt.Errorf("failed to parse private key: %w", err)
			}
		}
		rsaKey, ok := key.(*rsa.PrivateKey)
		if !ok {
			return nil, errors.New("private key is not RSA")
		}
		gen.PrivateKey = rsaKey

		pubDER, err := x509.MarshalPKIXPublicKey(&rsaKey.PublicKey)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal public key: %w", err)
		}
		h := sha256.Sum256(pubDER)
		gen.KeyID = "sha256:" + base64.RawURLEncoding.EncodeToString(h[:])
	}

	return gen, nil
}

func (g *ProvenanceGenerator) ComputeDigest(content []byte, algo HashAlgorithm) string {
	switch algo {
	case HashSHA512:
		h := sha512.Sum512(content)
		return fmt.Sprintf("%x", h[:])
	default:
		h := sha256.Sum256(content)
		return fmt.Sprintf("%x", h[:])
	}
}

func (g *ProvenanceGenerator) GenerateStatement(
	subjects []Subject,
	materials []ResourceDescriptor,
	recipe ProvenanceRecipe,
	buildMetadata *BuildMetadata,
) (*InTotoStatement, error) {
	buildDef := BuildDefinition{
		BuildType:            g.BuildType,
		ExternalParameters:   mustMarshal(recipe.Arguments),
		InternalParameters:   mustMarshal(map[string]any{
			"slsa_level":    3,
			"builder_id":    g.BuilderID,
			"completeness":  g.Completeness,
		}),
		ResolvedDependencies: materials,
	}

	runDetails := RunDetails{
		Builder: Builder{
			ID:      g.BuilderID,
			Version: map[string]string{"slsa": "v1.0"},
		},
		Metadata: buildMetadata,
	}

	prov := Provenance{
		BuildDefinition: buildDef,
		RunDetails:      runDetails,
	}

	provBytes, err := json.Marshal(prov)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal provenance: %w", err)
	}

	stmt := &InTotoStatement{
		Type:          MediaTypeInToto,
		Subject:       subjects,
		PredicateType: PredicateTypeProvenance,
		Predicate:     provBytes,
	}

	return stmt, nil
}

func (g *ProvenanceGenerator) SignStatement(stmt *InTotoStatement) (*DSSEEnvelope, error) {
	payload, err := json.Marshal(stmt)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal statement: %w", err)
	}

	env := &DSSEEnvelope{
		PayloadType: MediaTypeInToto,
		Payload:     base64.StdEncoding.EncodeToString(payload),
	}

	if g.PrivateKey != nil {
		paestr := []byte("DSSEv1")
		paestr = append(paestr, byte(' '))
		paestr = append(paestr, []byte(env.PayloadType)...)
		paestr = append(paestr, byte(' '))
		paestr = append(paestr, []byte(env.Payload)...)

		hash := sha256.Sum256(paestr)
		sig, err := rsa.SignPKCS1v15(rand.Reader, g.PrivateKey, crypto.SHA256, hash[:])
		if err != nil {
			return nil, fmt.Errorf("failed to sign: %w", err)
		}

		env.Signatures = []DSSESignature{{
			KeyID: g.KeyID,
			Sig:   base64.StdEncoding.EncodeToString(sig),
		}}
	}

	return env, nil
}

func (g *ProvenanceGenerator) VerifyEnvelope(envelope *DSSEEnvelope, publicKey *rsa.PublicKey) error {
	if len(envelope.Signatures) == 0 {
		return errors.New("no signatures in envelope")
	}

	paestr := []byte("DSSEv1")
	paestr = append(paestr, byte(' '))
	paestr = append(paestr, []byte(envelope.PayloadType)...)
	paestr = append(paestr, byte(' '))
	paestr = append(paestr, []byte(envelope.Payload)...)

	hash := sha256.Sum256(paestr)

	for _, sig := range envelope.Signatures {
		sigBytes, err := base64.StdEncoding.DecodeString(sig.Sig)
		if err != nil {
			return fmt.Errorf("failed to decode signature: %w", err)
		}
		if err := rsa.VerifyPKCS1v15(publicKey, crypto.SHA256, hash[:], sigBytes); err != nil {
			return fmt.Errorf("signature verification failed: %w", err)
		}
	}

	return nil
}

func (g *ProvenanceGenerator) DecodePayload(envelope *DSSEEnvelope) (*InTotoStatement, error) {
	payloadBytes, err := base64.StdEncoding.DecodeString(envelope.Payload)
	if err != nil {
		return nil, fmt.Errorf("failed to decode payload: %w", err)
	}

	var stmt InTotoStatement
	if err := json.Unmarshal(payloadBytes, &stmt); err != nil {
		return nil, fmt.Errorf("failed to unmarshal statement: %w", err)
	}

	return &stmt, nil
}

func NewSubject(name string, content []byte, algo HashAlgorithm) Subject {
	digest := map[string]string{}
	if algo == "" {
		algo = HashSHA256
	}
	h := sha256Hash(content)
	if algo == HashSHA512 {
		h = sha512Hash(content)
	}
	digest[string(algo)] = h
	return Subject{
		Name:   name,
		Digest: digest,
	}
}

func NewResourceDescriptor(uri string, content []byte, mediaType string) ResourceDescriptor {
	digest := map[string]string{
		string(HashSHA256): sha256Hash(content),
	}
	return ResourceDescriptor{
		URI:       uri,
		Digest:    digest,
		MediaType: mediaType,
	}
}

func GenerateBuildProvenance(
	artifactName string,
	artifactContent []byte,
	sourceRepo string,
	commitSHA string,
	builderID string,
	entryPoint string,
	envVars map[string]string,
	keyPEM []byte,
) ([]byte, error) {
	gen, err := NewProvenanceGenerator(builderID, "https://coroot.com/build-types/v1", keyPEM)
	if err != nil {
		return nil, fmt.Errorf("failed to create generator: %w", err)
	}

	now := time.Now().UTC().Format(time.RFC3339)
	invocationID := utils.NanoId(16)

	subjects := []Subject{
		NewSubject(artifactName, artifactContent, HashSHA256),
	}

	materials := []ResourceDescriptor{
		{
			URI:    sourceRepo,
			Digest: map[string]string{string(HashSHA256): commitSHA},
		},
	}

	envJSON, err := json.Marshal(envVars)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal env vars: %w", err)
	}
	argsJSON, err := json.Marshal(map[string]string{
		"entry_point": entryPoint,
		"source":      sourceRepo,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to marshal args: %w", err)
	}

	recipe := ProvenanceRecipe{
		Type:       gen.BuildType,
		EntryPoint: entryPoint,
		Arguments:  argsJSON,
		Environment: envJSON,
	}

	metadata := &BuildMetadata{
		InvocationID: invocationID,
		StartedOn:    now,
		FinishedOn:   now,
		Reproducible: true,
		Completeness: gen.Completeness,
		Annotations: map[string]string{
			"slsa_level": SLSABuildL3,
		},
	}

	stmt, err := gen.GenerateStatement(subjects, materials, recipe, metadata)
	if err != nil {
		return nil, fmt.Errorf("failed to generate statement: %w", err)
	}

	env, err := gen.SignStatement(stmt)
	if err != nil {
		return nil, fmt.Errorf("failed to sign statement: %w", err)
	}

	return json.Marshal(env)
}

func sha256Hash(content []byte) string {
	h := sha256.Sum256(content)
	return fmt.Sprintf("%x", h[:])
}

func sha512Hash(content []byte) string {
	h := sha512.Sum512(content)
	return fmt.Sprintf("%x", h[:])
}

func mustMarshal(v any) json.RawMessage {
	data, err := json.Marshal(v)
	if err != nil {
		return json.RawMessage("{}")
	}
	return data
}