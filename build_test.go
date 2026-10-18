package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
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
	for _, invalid := range [][]byte{[]byte("{}"), append(data, []byte("{}")...), []byte(strings.Replace(string(data), info.Revision, "build-"+strings.Repeat("0", 64), 1)), []byte(strings.Replace(string(data), info.Repository, "invalid-repository", 1))} {
		if _, err := parseBuildInfo(invalid); err == nil {
			t.Fatal("invalid provenance accepted")
		}
	}
}

func TestBuildRepositoryIdentity(t *testing.T) {
	for _, repository := range []string{"truvaagents/retail-order-service", "example-team/retail-order-service", "a/.github", strings.Repeat("a", 39) + "/" + strings.Repeat("b", 100)} {
		info := BuildInfo{SchemaVersion: 1, Repository: repository, SourceSHA256: strings.Repeat("a", 64), GitRevision: strings.Repeat("b", 40), FrameworkVersion: "v0.5.0"}
		sum := sha256.Sum256([]byte(info.SourceSHA256 + "\n" + info.FrameworkVersion + "\n"))
		info.Revision = "build-" + hex.EncodeToString(sum[:])
		data, err := json.Marshal(info)
		if err != nil {
			t.Fatal(err)
		}
		// Runtime environment cannot replace the compiled repository identity.
		t.Setenv("ORDER_BUILD_REPOSITORY", "different-owner/different-repository")
		if got, err := parseBuildInfo(data); err != nil || got.Repository != repository {
			t.Fatalf("repository %q rejected or rewritten: %v", repository, err)
		}
	}
	for _, repository := range []string{"", "owner", "owner/repo/extra", "https://github.com/owner/repo", "owner/repo\n", "-owner/repo", "owner-/repo", "owner--name/repo", "owner/.", "owner/..", strings.Repeat("a", 40) + "/repo", "owner/" + strings.Repeat("b", 101)} {
		if validBuildRepository(repository) {
			t.Errorf("invalid repository %q accepted", repository)
		}
	}
}

func TestSetupBuildRepository(t *testing.T) {
	script, err := filepath.Abs("setup.sh")
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, origin, override, want string
		explicit                     bool
	}{
		{name: "https", origin: "https://github.com/example-team/retail-order-service.git", want: "example-team/retail-order-service"},
		{name: "https without suffix", origin: "https://github.com/example-team/retail-order-service", want: "example-team/retail-order-service"},
		{name: "ssh", origin: "git@github.com:example-team/retail-order-service.git", want: "example-team/retail-order-service"},
		{name: "ssh URL", origin: "ssh://git@github.com/example-team/retail-order-service.git", want: "example-team/retail-order-service"},
		{name: "explicit override", origin: "https://example.invalid/mirror", explicit: true, override: "example-team/retail-order-service", want: "example-team/retail-order-service"},
		{name: "offline explicit identity", explicit: true, override: "example-team/retail-order-service", want: "example-team/retail-order-service"},
		{name: "empty explicit identity", explicit: true},
		{name: "invalid explicit identity", explicit: true, override: "https://github.com/example-team/retail-order-service"},
		{name: "missing origin"},
		{name: "unsupported origin", origin: "https://example.invalid/mirror"},
		{name: "credential in origin", origin: "https://do-not-print@github.com/example-team/retail-order-service.git"},
		{name: "invalid owner", origin: "https://github.com/example--team/retail-order-service.git"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			directory := t.TempDir()
			if output, err := exec.Command("git", "init", "--quiet", directory).CombinedOutput(); err != nil {
				t.Fatalf("initialize fixture: %s: %v", output, err)
			}
			if tc.origin != "" {
				if output, err := exec.Command("git", "-C", directory, "remote", "add", "origin", tc.origin).CombinedOutput(); err != nil {
					t.Fatalf("configure fixture: %s: %v", output, err)
				}
			}
			command := exec.Command("bash", "-c", `whoami() { printf "unexpected username lookup\n" >&2; return 1; }; source "$1"; SCRIPT_DIR="$2"; resolve_build_repository`, "bash", script, directory)
			for _, variable := range os.Environ() {
				if !strings.HasPrefix(variable, "ORDER_BUILD_REPOSITORY=") {
					command.Env = append(command.Env, variable)
				}
			}
			if tc.explicit {
				command.Env = append(command.Env, "ORDER_BUILD_REPOSITORY="+tc.override)
			}
			output, err := command.CombinedOutput()
			if tc.want == "" {
				if err == nil || len(output) == 0 || strings.Contains(string(output), "do-not-print") {
					t.Fatalf("invalid configuration was accepted or unsafe diagnostic returned: %q, %v", output, err)
				}
				return
			}
			if err != nil || strings.TrimSpace(string(output)) != tc.want {
				t.Fatalf("repository: got %q, %v; want %q", output, err, tc.want)
			}
		})
	}
}
