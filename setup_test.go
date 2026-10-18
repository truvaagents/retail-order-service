package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestSetupHelpWithoutUsername(t *testing.T) {
	script, err := filepath.Abs("setup.sh")
	if err != nil {
		t.Fatal(err)
	}
	for _, argument := range []string{"", "help", "-h", "--help"} {
		t.Run("argument="+argument, func(t *testing.T) {
			command := exec.Command("bash", "-c", `
whoami() { printf 'unexpected username lookup\n' >&2; return 1; }
export -f whoami
if [ -n "$2" ]; then bash "$1" "$2"; else bash "$1"; fi
`, "bash", script, argument)
			output, err := command.CombinedOutput()
			if err != nil || !strings.Contains(string(output), "Usage: ./setup.sh") || strings.Contains(string(output), "unexpected username lookup") {
				t.Fatalf("help requires username: %q, %v", output, err)
			}
		})
	}
}

func TestSetupClusterSelection(t *testing.T) {
	script, err := os.ReadFile("setup.sh")
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, verb, environmentCluster, configuredCluster, username, wantCluster string
	}{
		{name: "default username", verb: "status", username: "fixture-user", wantCluster: "truvag3-demo-fixture-user"},
		{name: "environment override without username", verb: "status", environmentCluster: "selected-cluster", wantCluster: "selected-cluster"},
		{name: "configured status without username", verb: "status", configuredCluster: "configured-cluster", wantCluster: "configured-cluster"},
		{name: "configured logs without username", verb: "logs", configuredCluster: "configured-cluster", wantCluster: "configured-cluster"},
		{name: "missing username", verb: "status"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			directory := t.TempDir()
			scriptPath := filepath.Join(directory, "setup.sh")
			if err := os.WriteFile(scriptPath, script, 0o600); err != nil {
				t.Fatal(err)
			}
			if tc.configuredCluster != "" {
				if err := os.WriteFile(filepath.Join(directory, ".env"), []byte("CLUSTER_NAME="+tc.configuredCluster+"\n"), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			for name, body := range map[string]string{
				"whoami": `printf 'whoami\n' >> "$SETUP_CALLS"
[ -n "$SETUP_USERNAME" ] || exit 1
printf '%s\n' "$SETUP_USERNAME"
`,
				"kind": `printf 'kind %s\n' "$*" >> "$SETUP_CALLS"
[ "$*" = 'get clusters' ] || exit 1
printf '%s\n' "$SETUP_CLUSTER"
`,
				"kubectl": `printf 'kubectl %s\n' "$*" >> "$SETUP_CALLS"`,
			} {
				if err := os.WriteFile(filepath.Join(directory, name), []byte("#!/bin/sh\n"+body), 0o700); err != nil {
					t.Fatal(err)
				}
			}
			callsPath := filepath.Join(directory, "calls")
			command := exec.Command("bash", scriptPath, tc.verb)
			for _, variable := range os.Environ() {
				key, _, _ := strings.Cut(variable, "=")
				switch key {
				case "PATH", "CLUSTER_NAME", "KIND_CONTEXT", "SETUP_CALLS", "SETUP_USERNAME", "SETUP_CLUSTER":
					continue
				}
				command.Env = append(command.Env, variable)
			}
			command.Env = append(command.Env, "PATH="+directory+string(os.PathListSeparator)+os.Getenv("PATH"), "SETUP_CALLS="+callsPath, "SETUP_USERNAME="+tc.username, "SETUP_CLUSTER="+tc.wantCluster)
			if tc.environmentCluster != "" {
				command.Env = append(command.Env, "CLUSTER_NAME="+tc.environmentCluster)
			}
			output, runErr := command.CombinedOutput()
			calls, err := os.ReadFile(callsPath)
			if tc.wantCluster == "" {
				if runErr == nil || !strings.Contains(string(output), "set CLUSTER_NAME explicitly") || err != nil || string(calls) != "whoami\n" {
					t.Fatalf("missing username should fail before accessing cluster: output=%q, err=%v, calls=%q, readErr=%v", output, runErr, calls, err)
				}
				return
			}
			if runErr != nil || err != nil {
				t.Fatalf("cluster selection: output=%q, err=%v, calls=%q, readErr=%v", output, runErr, calls, err)
			}
			want := ""
			if tc.environmentCluster == "" && tc.configuredCluster == "" {
				want = "whoami\n"
			}
			context := "--context kind-" + tc.wantCluster
			want += "kind get clusters\nkubectl " + context + " get namespace truvag3-examples\n"
			if tc.verb == "logs" {
				want += "kubectl " + context + " logs -n truvag3-examples deployment/retail-order-service --tail=100\n"
			} else {
				want += "kubectl " + context + " get deployment,pod,service,ingress -n truvag3-examples -l app.kubernetes.io/part-of=retail-order-service\n"
			}
			if string(calls) != want {
				t.Fatalf("cluster commands: got %q; want %q", calls, want)
			}
		})
	}
}
