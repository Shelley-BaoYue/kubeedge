/*
Copyright 2026 The KubeEdge Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"io"
	"regexp"
	"slices"
	"strings"
	"testing"
)

func TestMakeCmdNodeShards(t *testing.T) {
	t.Setenv(focusEnvKey, "")
	t.Setenv(skipEnvKey, `\[Serial\]`)
	t.Setenv(parallelEnvKey, "-p")
	t.Setenv(extraArgsEnvKey, "")
	for _, shard := range []string{"all", "fast", "slow"} {
		t.Run(shard, func(t *testing.T) {
			t.Setenv(nodeShardEnvKey, shard)
			cmd, err := makeCmd(io.Discard)
			if err != nil {
				t.Fatal(err)
			}
			if !slices.Contains(cmd.Args, `--skip=\[Serial\]`) || !slices.Contains(cmd.Args, "-p") {
				t.Fatalf("lost existing skip or parallel flags: %v", cmd.Args)
			}
			wantFocus := defaultFocus
			if shard == "slow" {
				wantFocus = slowNodeFocus
			}
			if !slices.Contains(cmd.Args, "--focus="+wantFocus) {
				t.Fatalf("missing shard focus: %v", cmd.Args)
			}
			if shard == "fast" {
				found := false
				for _, arg := range cmd.Args {
					if strings.HasPrefix(arg, "--skip=") && strings.Contains(arg, slowNodeFocus) {
						found = true
					}
				}
				if !found {
					t.Fatalf("missing slow-test exclusion: %v", cmd.Args)
				}
			}
			binaryArgs := cmd.Args[slices.Index(cmd.Args, "--")+1:]
			if slices.Contains(binaryArgs, "--ginkgo.fail-on-empty=true") != (shard != "all") {
				t.Fatalf("shards must fail on an empty selection: %v", binaryArgs)
			}
		})
	}
}

func TestNodeConformanceFilters(t *testing.T) {
	tests := []struct {
		name string
		slow bool
	}{
		{"[sig-node] Pods should cap back-off at MaxContainerBackOff [Slow] [NodeConformance]", true},
		{"[sig-node] Pods should have their auto-restart back-off timer reset on image update [Slow] [NodeConformance]", true},
		{"[sig-node] Variable Expansion should verify that a failing subpath expansion can be modified during the lifecycle of a container [Slow] [Conformance]", false},
		{"[sig-node] Probing container should *not* be restarted with a GRPC liveness probe [NodeConformance] [Conformance]", false},
		{"[sig-node] Pods should support pod readiness gates [NodeConformance]", false},
		{"[sig-node] Pods should delete a collection of pods [Conformance]", false},
	}
	all := regexp.MustCompile(defaultFocus)
	fastFocus, fastSkip, err := nodeConformanceFilters(defaultFocus, "fast")
	if err != nil {
		t.Fatal(err)
	}
	slowFocus, slowSkip, err := nodeConformanceFilters(defaultFocus, "slow")
	if err != nil {
		t.Fatal(err)
	}
	if slowSkip != "" {
		t.Fatalf("unexpected slow shard skip: %q", slowSkip)
	}
	fast := regexp.MustCompile(fastFocus)
	skip := regexp.MustCompile(fastSkip)
	slow := regexp.MustCompile(slowFocus)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			inFast := fast.MatchString(tt.name) && !skip.MatchString(tt.name)
			inSlow := slow.MatchString(tt.name)
			if inFast == inSlow || (inFast || inSlow) != all.MatchString(tt.name) {
				t.Fatalf("shards must partition the original selection: fast=%t, slow=%t", inFast, inSlow)
			}
			if inSlow != tt.slow {
				t.Fatalf("slow shard membership = %t, want %t", inSlow, tt.slow)
			}
		})
	}
	for _, name := range []string{
		"[sig-node] Pods should run a non-conformance test",
		"[sig-apps] Pods should cap back-off at MaxContainerBackOff [Slow] [NodeConformance]",
	} {
		if (fast.MatchString(name) && !skip.MatchString(name)) || slow.MatchString(name) {
			t.Errorf("shards selected an unrelated test: %s", name)
		}
	}
}

func TestNodeConformanceFiltersDefaultsAndErrors(t *testing.T) {
	for _, focus := range []string{defaultFocus, "custom focus"} {
		gotFocus, skip, err := nodeConformanceFilters(focus, "all")
		if err != nil || gotFocus != focus || skip != "" {
			t.Fatalf("all shard must preserve focus: focus=%q, skip=%q, err=%v", gotFocus, skip, err)
		}
	}
	for _, shard := range []string{"fast", "slow"} {
		if _, _, err := nodeConformanceFilters("custom focus", shard); err == nil {
			t.Errorf("expected an error for custom focus with shard %q", shard)
		}
	}
	if _, _, err := nodeConformanceFilters(defaultFocus, "invalid"); err == nil {
		t.Fatal("expected an error for an invalid shard")
	}
}
