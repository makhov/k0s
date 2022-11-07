/*
Copyright 2022 k0s authors

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

package nllb3x3

import (
	"fmt"
	"testing"
	"time"

	"github.com/k0sproject/k0s/inttest/common"
	testifysuite "github.com/stretchr/testify/suite"
)

type suite struct {
	common.FootlooseSuite
}

const config = `
spec:
  network:
    nodeLocalLoadBalancer:
	  enabled: true
	  type: EnvoyProxy
`

// SetupTest prepares the controller and filesystem, getting it into a consistent
// state which we can run tests against.
func (s *suite) SetupTest() {
	var joinToken string

	for idx := 0; idx < s.FootlooseSuite.ControllerCount; idx++ {
		s.Require().NoError(s.WaitForSSH(s.ControllerNode(idx), 2*time.Minute, 1*time.Second))
		s.PutFile(s.ControllerNode(idx), "/tmp/k0s.yaml", config)

		// Note that the token is intentionally empty for the first controller
		s.Require().NoError(s.InitController(idx, "--config=/tmp/k0s.yaml", "--disable-components=metrics-server", joinToken))
		s.Require().NoError(s.WaitJoinAPI(s.ControllerNode(idx)))

		// With the primary controller running, create the join token for subsequent controllers.
		if idx == 0 {
			token, err := s.GetJoinToken("controller")
			s.Require().NoError(err)
			joinToken = token
		}
	}

	// Final sanity -- ensure all nodes see each other according to etcd
	for idx := 0; idx < s.FootlooseSuite.ControllerCount; idx++ {
		s.Require().Len(s.GetMembers(idx), s.FootlooseSuite.ControllerCount)
	}

	// Create a worker join token
	workerJoinToken, err := s.GetJoinToken("worker")
	s.Require().NoError(err)

	// Start the workers using the join token
	s.Require().NoError(s.RunWorkersWithToken(workerJoinToken))

	client, err := s.KubeClient(s.ControllerNode(0))
	s.Require().NoError(err)

	const kubeSystem = "kube-system"
	for idx := 0; idx < s.FootlooseSuite.WorkerCount; idx++ {
		s.Require().NoError(s.WaitForNodeReady(s.WorkerNode(idx), client))

		nodeName := s.WorkerNode(idx)
		nllbPodName := fmt.Sprintf("nllb-%s", nodeName)
		s.Require().NoError(s.WaitForNodeReady(nodeName, client))
		s.T().Logf("Waiting for pod %s/%s to become ready", kubeSystem, nllbPodName)
		s.Require().NoError(
			common.WaitForPod(s.Context(), client, nllbPodName, kubeSystem),
			"Pod %s/%s is not ready", kubeSystem, nllbPodName,
		)
	}
}

func (s *suite) TestK0sGetsUp() {
	require := s.Require()
	const kubeSystem = "kube-system"

	kc, err := s.KubeClient(s.ControllerNode(0))
	require.NoError(err)

	require.NoError(common.WaitForKubeRouterReady(s.Context(), kc), "kube-router did not start")
	require.NoError(common.WaitForLease(s.Context(), kc, "kube-scheduler", kubeSystem))
	require.NoError(common.WaitForLease(s.Context(), kc, "kube-controller-manager", kubeSystem))

	// Test that we get logs, it's a signal that konnectivity tunnels work.
	s.T().Log("Waiting to get logs from pods")
	require.NoError(common.WaitForPodLogs(s.Context(), kc, kubeSystem))

	sshW0, err := s.SSH(s.WorkerNode(0))
	s.Require().NoError(err, "failed to SSH into worker 0")
	defer sshW0.Disconnect()

	out, err := sshW0.ExecWithOutput(s.Context(), `pidof -s kubelet`)
	s.Require().NoError(err)
	s.T().Log("pid ", out)

	_, err = sshW0.ExecWithOutput(s.Context(), `kill $(pidof -s kubelet)`)
	s.Require().NoError(err)

	out, err = sshW0.ExecWithOutput(s.Context(), `pidof -s kubelet`)
	s.Require().NoError(err)
	s.T().Log("pid ", out)
}

func TestNodeLocalLoadBalancerSuite(t *testing.T) {
	s := suite{
		common.FootlooseSuite{
			ControllerCount: 3,
			WorkerCount:     3,
		},
	}
	testifysuite.Run(t, &s)
}
