package main

import (
	"bytes"
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"regexp"
)

// setup.sh replaces the placeholder only inside the isolated build snapshot.
// No runtime environment variable may override compiled source identity.
//
//go:embed build-info.json
var compiledBuild []byte

type BuildInfo struct {
	SchemaVersion       int    `json:"schema_version"`
	Repository          string `json:"repository"`
	Revision            string `json:"revision"`
	SourceSHA256        string `json:"source_sha256"`
	GitRevision         string `json:"git_revision"`
	SourceMatchesCommit bool   `json:"source_matches_commit"`
	FrameworkVersion    string `json:"framework_version"`
}

var sha256Pattern = regexp.MustCompile(`^[a-f0-9]{64}$`)
var gitRevisionPattern = regexp.MustCompile(`^[a-f0-9]{40}$`)
var frameworkVersionPattern = regexp.MustCompile(`^v[0-9]+\.[0-9]+\.[0-9]+$`)

func loadBuildInfo() (BuildInfo, error) {
	return parseBuildInfo(compiledBuild)
}

func parseBuildInfo(data []byte) (BuildInfo, error) {
	var info BuildInfo
	d := json.NewDecoder(bytes.NewReader(data))
	d.DisallowUnknownFields()
	if len(data) > 2048 || d.Decode(&info) != nil || info.SchemaVersion != 1 || info.Repository != "truvaagents/retail-order-service" || !sha256Pattern.MatchString(info.SourceSHA256) || !gitRevisionPattern.MatchString(info.GitRevision) || !frameworkVersionPattern.MatchString(info.FrameworkVersion) {
		return info, errors.New("invalid build identity; build through setup.sh")
	}
	var extra any
	if !errors.Is(d.Decode(&extra), io.EOF) {
		return info, errors.New("invalid build identity envelope")
	}
	digest := sha256.Sum256([]byte(info.SourceSHA256 + "\n" + info.FrameworkVersion + "\n"))
	if info.Revision != "build-"+hex.EncodeToString(digest[:]) {
		return info, errors.New("build identity digest mismatch")
	}
	return info, nil
}

func (s *service) buildInfo(w http.ResponseWriter, r *http.Request) {
	// Downward API values describe this serving pod, not Kubernetes' desired
	// state, historical deployments, image attestation, or arbitrary pod data.
	WriteJSON(w, http.StatusOK, struct {
		BuildInfo
		PodName   string `json:"pod_name,omitempty"`
		Namespace string `json:"namespace,omitempty"`
		PodUID    string `json:"pod_uid,omitempty"`
	}{s.build, os.Getenv("ORDER_POD_NAME"), os.Getenv("ORDER_POD_NAMESPACE"), os.Getenv("ORDER_POD_UID")})
}
