/*
Copyright 2021 The Kubernetes Authors.

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
package build

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"strings"

	"k8s.io/klog"

	"sigs.k8s.io/kubetest2/pkg/build"
	"sigs.k8s.io/kubetest2/pkg/exec"
)

type gkeBuildAction string

const (
	compile      gkeBuildAction = "compile"
	pack         gkeBuildAction = "package"
	stage        gkeBuildAction = "push-gcs"
	printVersion gkeBuildAction = "print-version"
)

const (
	// GKEMakeStrategy builds and stages using the gke_make build
	GKEMakeStrategy build.BuildAndStageStrategy = "gke_make"
)

type GKEMake struct {
	RepoRoot      string
	BuildScript   string
	VersionSuffix string
	StageLocation string
}

func gkeBuildActions(actions []gkeBuildAction) string {
	stringActions := make([]string, len(actions))
	for i, action := range actions {
		stringActions[i] = string(action)
	}
	return strings.Join(stringActions, ",")
}

func arg(key, value string) string {
	return fmt.Sprintf("%s=%s", key, value)
}

func (gmb *GKEMake) runWithActions(stdout, stderr io.Writer, actions []gkeBuildAction, extraArgs ...string) error {

	args := []string{arg("GKE_BUILD_ACTIONS", gkeBuildActions(actions))}
	args = append(args, extraArgs...)
	cmd := exec.Command(gmb.BuildScript, args...)
	cmd.SetDir(gmb.RepoRoot)
	cmd.SetStdout(stdout)
	cmd.SetStderr(stderr)
	return cmd.Run()
}

func (gmb *GKEMake) Build() (string, error) {
	klog.V(2).Infof("starting gke build ...")

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	if err := gmb.runWithActions(stdout, stderr, []gkeBuildAction{printVersion}, arg("VERSION_SUFFIX", gmb.VersionSuffix)); err != nil {
		klog.Errorf("failed to get version: %s\n%v", stderr.String(), err)
		return "", err
	}

	version := strings.TrimSpace(stdout.String())
	if version == "" {
		klog.Error(stderr.String())
		return "", fmt.Errorf("failed to get version: got empty version")
	}

	// Skip validation for faster builds
	// TODO: add support for a separate validate mode
	if err := gmb.runWithActions(os.Stdout, os.Stderr, []gkeBuildAction{compile, pack}, arg("VERSION", version), arg("TARGET_PLATFORMS", "linux/amd64,windows/amd64")); err != nil {
		return "", err
	}

	return version, nil
}

var _ build.Builder = &GKEMake{}

func (gmb *GKEMake) Stage(version string) error {
	klog.V(2).Infof("staging gke builds ...")
	if !strings.HasPrefix(version, "v") {
		version = "v" + version
	}
	args := []string{arg("VERSION", version)}
	if gmb.StageLocation != "" {
		args = append(args, arg("GCS_BUCKET", gmb.StageLocation))
	}

	if err := gmb.runWithActions(os.Stdout, os.Stderr, []gkeBuildAction{stage}, args...); err != nil {
		return err
	}
	return nil
}

var _ build.Stager = &GKEMake{}
