/*
Copyright 2021 k0s authors

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
package manifests

import (
	"fmt"
	"strings"
	"testing"

	"github.com/instrumenta/kubeval/kubeval"
	"github.com/stretchr/testify/suite"

	"github.com/k0sproject/k0s/inttest/common"
)

type ManifestsSuite struct {
	common.FootlooseSuite
}

func (s *ManifestsSuite) TestK0sGetsUp() {
	s.NoError(s.InitController(0, "--enable-worker"))

	kc, err := s.KubeClient(s.ControllerNode(0))
	s.NoError(err)

	err = s.WaitForNodeReady(s.ControllerNode(0), kc)
	s.NoError(err)

	ssh, err := s.SSH(s.ControllerNode(0))
	s.NoError(err)
	defer ssh.Disconnect()

	out, err := ssh.ExecWithOutput("find /var/lib/k0s/manifests -type f")
	s.NoError(err)

	files := strings.Split(out, "\n")

	cfg := kubeval.NewDefaultConfig()
	cfg.Strict = true

	for _, file := range files {
		filename := strings.TrimPrefix(file, "/var/lib/k0s/manifests/")

		s.T().Run(filename, func(t *testing.T) {
			out, err := ssh.ExecWithOutput(fmt.Sprintf("cat %s", file))
			s.NoError(err)

			cfg.FileName = filename

			results, err := kubeval.Validate([]byte(out), cfg)
			s.NoError(err, filename)

			for _, result := range results {
				for _, err := range result.Errors {
					s.T().Error(filename, result.Kind, result.ResourceName, err)
				}
			}
		})
	}
}

func TestManifestsSuite(t *testing.T) {
	s := ManifestsSuite{
		common.FootlooseSuite{
			ControllerCount: 1,
		},
	}
	suite.Run(t, &s)
}
