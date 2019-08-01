// +build integration

/*
Copyright 2016 The Kubernetes Authors All rights reserved.

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
	"fmt"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/docker/machine/libmachine/state"
	"k8s.io/minikube/pkg/minikube/constants"
	"k8s.io/minikube/test/integration/util"
)

func TestStartStop(t *testing.T) {
	p := profileName(t) // gets profile name used for minikube and kube context
	if toParallel() {
		t.Parallel()
	}

	t.Run("group", func(t *testing.T) {
		if toParallel() {
			t.Parallel()
		}
		tests := []struct {
			name string
			args []string
		}{
			{"oldest", []string{ // nocache_oldest
				"--cache-images=false",
				fmt.Sprintf("--kubernetes-version=%s", constants.OldestKubernetesVersion),
				// default is the network created by libvirt, if we change the name minikube won't boot
				// because the given network doesn't exist
				"--kvm-network=default",
				"--kvm-qemu-uri=qemu:///system",
			}},
			{"cni", []string{ // feature_gates_newest_cni
				"--feature-gates",
				"ServerSideApply=true",
				"--network-plugin=cni",
				"--extra-config=kubelet.network-plugin=cni",
				"--extra-config=kubeadm.pod-network-cidr=192.168.111.111/16",
				fmt.Sprintf("--kubernetes-version=%s", constants.NewestKubernetesVersion),
			}},
			{"containerd", []string{ // containerd_and_non_default_apiserver_port
				"--container-runtime=containerd",
				"--docker-opt containerd=/var/run/containerd/containerd.sock",
				"--apiserver-port=8444",
			}},
			{"crio", []string{ // crio_ignore_preflights
				"--container-runtime=crio",
				"--extra-config",
				"kubeadm.ignore-preflight-errors=SystemVerification",
			}},
		}

		for _, tc := range tests {
			n := tc.name // because similar to https://golang.org/doc/faq#closures_and_goroutines
			t.Run(tc.name, func(t *testing.T) {
				if toParallel() {
					t.Parallel()
				}

				pn := p + n // TestStartStopoldest
				mk := NewMinikubeRunner(t, pn, "--wait=false")
				// TODO : redundant first clause ? never happens?
				if !strings.Contains(pn, "docker") && isTestNoneDriver() {
					t.Skipf("skipping %s - incompatible with none driver", t.Name())
				}

				mk.RunCommand("config set WantReportErrorPrompt false", true)
				stdout, stderr, err := mk.Start(tc.args...)
				if err != nil {
					t.Fatalf("failed to start minikube (for profile %s) failed : %v\nstdout: %s\nstderr: %s", pn, err, stdout, stderr)
				}

				mk.CheckStatus(state.Running.String())

				ip, stderr := mk.RunCommand("ip", true)
				ip = strings.TrimRight(ip, "\n")
				if net.ParseIP(ip) == nil {
					t.Fatalf("IP command returned an invalid address: %s \n %s", ip, stderr)
				}

				stop := func() error {
					stdout, stderr, err = mk.RunCommandRetriable("stop")
					return mk.CheckStatusNoFail(state.Stopped.String())
				}

				err = util.RetryX(stop, 10*time.Second, 2*time.Minute)
				mk.CheckStatus(state.Stopped.String())

				// TODO medyagh:
				// https://github.com/kubernetes/minikube/issues/4854

				stdout, stderr, err = mk.Start(tc.args...)
				if err != nil {
					t.Fatalf("failed to start minikube (for profile %s) failed : %v\nstdout: %s\nstderr: %s", t.Name(), err, stdout, stderr)
				}

				mk.CheckStatus(state.Running.String())

				mk.RunCommand("delete", true)
				mk.CheckStatus(state.None.String())
			})
		}
	})
}
