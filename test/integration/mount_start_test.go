//go:build integration
// +build integration

/*
Copyright 2021 The Kubernetes Authors All rights reserved.

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

package integration

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"testing"
)

const (
	mountGID   = "0"
	mountMSize = "6543"
	mountMode  = "0777"
	mountPort  = "46464"
	mountUID   = "0"
)

// TestMountStart tests using the mount command on start
func TestMountStart(t *testing.T) {
	if NoneDriver() {
		t.Skip("skipping: none driver does not support mount")
	}

	type validateFunc func(context.Context, *testing.T, string)
	profile1 := UniqueProfileName("mount-start-1")
	profile2 := UniqueProfileName("mount-start-2")
	ctx, cancel := context.WithTimeout(context.Background(), Minutes(15))
	defer Cleanup(t, profile1, cancel)
	defer Cleanup(t, profile2, cancel)

	// Serial tests
	t.Run("serial", func(t *testing.T) {
		tests := []struct {
			name      string
			validator validateFunc
			profile   string
		}{
			{"StartWithMountFirst", validateStartWithMount, profile1},
			{"StartWithMountSecond", validateStartWithMount, profile2},
			{"VerifyMountFirst", validateMount, profile1},
			{"VerifyMountSecond", validateMount, profile2},
			{"DeleteFirst", validateDelete, profile1},
			{"VerifyMountPostDelete", validateMount, profile2},
			{"Stop", validateMountStop, profile2},
			{"RestartStopped", validateRestart, profile2},
			{"VerifyMountPostStop", validateMount, profile2},
		}

		for _, test := range tests {
			if ctx.Err() == context.DeadlineExceeded {
				t.Fatalf("Unable to run more tests (deadline exceeded)")
			}

			t.Run(test.name, func(t *testing.T) {
				test.validator(ctx, t, test.profile)
			})
		}
	})
}

// validateStartWithMount starts a cluster with mount enabled
func validateStartWithMount(ctx context.Context, t *testing.T, profile string) {
	defer PostMortemLogs(t, profile)

	args := []string{"start", "-p", profile, "--memory=2048", "--mount", "--mount-gid", mountGID, "--mount-msize", mountMSize, "--mount-mode", mountMode, "--mount-port", mountPort, "--mount-uid", mountUID}
	args = append(args, StartArgs()...)
	rr, err := Run(t, exec.CommandContext(ctx, Target(), args...))
	if err != nil {
		t.Fatalf("failed to start minikube with args: %q : %v", rr.Command(), err)
	}
}

// validateMount checks if the cluster has a folder mounted
func validateMount(ctx context.Context, t *testing.T, profile string) {
	defer PostMortemLogs(t, profile)

	sshArgs := []string{"-p", profile, "ssh", "--"}

	args := sshArgs
	args = append(args, "ls", "/minikube-host")
	rr, err := Run(t, exec.CommandContext(ctx, Target(), args...))
	if err != nil {
		t.Fatalf("mount failed: %q : %v", rr.Command(), err)
	}

	// Docker has it's own mounting method, it doesn't respect the mounting flags
	if DockerDriver() {
		return
	}

	args = sshArgs
	args = append(args, "stat", "--format", "'%a'", "/minikube-host")
	rr, err = Run(t, exec.CommandContext(ctx, Target(), args...))
	if err != nil {
		t.Fatalf("failed to get directory mode: %v", err)
	}

	want := "777"
	if !strings.Contains(rr.Output(), want) {
		t.Errorf("wanted mode to be %q; got: %q", want, rr.Output())
	}

	// We can't get the mount details with Hyper-V
	if HyperVDriver() {
		return
	}

	args = sshArgs
	args = append(args, "mount", "|", "grep", "9p")
	rr, err = Run(t, exec.CommandContext(ctx, Target(), args...))
	if err != nil {
		t.Fatalf("failed to get mount information: %v", err)
	}

	flags := []struct {
		key      string
		expected string
	}{
		{"gid", mountGID},
		{"msize", mountMSize},
		{"port", mountPort},
		{"uid", mountUID},
	}

	for _, flag := range flags {
		want := fmt.Sprintf("%s=%s", flag.key, flag.expected)
		if !strings.Contains(rr.Output(), want) {
			t.Errorf("wanted gid to be: %q; got: %q", want, rr.Output())
		}
	}
}

// validateMountStop stops a cluster
func validateMountStop(ctx context.Context, t *testing.T, profile string) {
	defer PostMortemLogs(t, profile)

	args := []string{"stop", "-p", profile}
	rr, err := Run(t, exec.CommandContext(ctx, Target(), args...))
	if err != nil {
		t.Fatalf("stop failed: %q : %v", rr.Command(), err)
	}
}

// validateRestart restarts a cluster
func validateRestart(ctx context.Context, t *testing.T, profile string) {
	defer PostMortemLogs(t, profile)

	args := []string{"start", "-p", profile}
	rr, err := Run(t, exec.CommandContext(ctx, Target(), args...))
	if err != nil {
		t.Fatalf("restart failed: %q : %v", rr.Command(), err)
	}
}
