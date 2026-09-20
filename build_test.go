package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strings"
	"testing"
)

func TestBuildIdentity(t *testing.T) {
	info := BuildInfo{SchemaVersion: 1, Repository: "truvaagents/retail-order-service", SourceSHA256: strings.Repeat("a", 64), GitRevision: strings.Repeat("b", 40), FrameworkVersion: "v0.5.0"}
	sum := sha256.Sum256([]byte(info.SourceSHA256 + "\n" + info.FrameworkVersion + "\n"))
	info.Revision = "build-" + hex.EncodeToString(sum[:])
	data, _ := json.Marshal(info)
	if got, err := parseBuildInfo(data); err != nil || got != info {
		t.Fatalf("valid working-tree provenance rejected: %v", err)
	}
	for _, invalid := range [][]byte{[]byte("{}"), append(data, []byte("{}")...), []byte(strings.Replace(string(data), info.Revision, "build-"+strings.Repeat("0", 64), 1)), []byte(strings.Replace(string(data), info.Repository, "unapproved/target", 1))} {
		if _, err := parseBuildInfo(invalid); err == nil {
			t.Fatal("invalid provenance accepted")
		}
	}
}
