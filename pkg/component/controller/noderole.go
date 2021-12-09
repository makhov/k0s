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
package controller

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/k0sproject/k0s/pkg/constant"
	k8sutil "github.com/k0sproject/k0s/pkg/kubernetes"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
)

// NodeRole implements the component interface to manage node role labels for worker nodes
type NodeRole struct {
	kubeClientFactory k8sutil.ClientFactoryInterface

	log     *logrus.Entry
	k0sVars constant.CfgVars
}

// NewNodeRole creates new NodeRole reconciler
func NewNodeRole(k0sVars constant.CfgVars, clientFactory k8sutil.ClientFactoryInterface) (*NodeRole, error) {
	log := logrus.WithFields(logrus.Fields{"component": "noderole"})
	return &NodeRole{
		kubeClientFactory: clientFactory,
		log:               log,
		k0sVars:           k0sVars,
	}, nil
}

// Init no-op
func (n *NodeRole) Init() error {
	return nil
}

// Run adds labels
func (n *NodeRole) Run(ctx context.Context) error {
	client, err := n.kubeClientFactory.GetClient()
	if err != nil {
		return err
	}
	go func() {
		timer := time.NewTicker(1 * time.Minute)
		defer timer.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-timer.C:
				nodes, err := client.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
				if err != nil {
					n.log.Errorf("failed to get node list: %v", err)
					continue
				}
			NODES:
				for _, node := range nodes.Items {
					var labelToAdd string
					for label, value := range node.Labels {
						if strings.HasPrefix(label, constant.NodeRoleLabelNamespace) {
							continue NODES
						}

						if label == constant.K0SNodeRoleLabel {
							labelToAdd = fmt.Sprintf("%s/%s=true", constant.NodeRoleLabelNamespace, value)
						}
					}

					_, err = n.addNodeLabel(ctx, client, node.Name, labelToAdd)
					if err != nil {
						n.log.Errorf("failed to set label '%s' to node %s: %v", labelToAdd, node.Name, err)
						continue
					}
				}
			}
		}
	}()

	return nil
}

func (n *NodeRole) addNodeLabel(ctx context.Context, client kubernetes.Interface, node, label string) (*corev1.Node, error) {
	patch := fmt.Sprintf(`[{"op":"add", "path":"/metadata/labels", "value":"%s" }]`, label)
	return client.CoreV1().Nodes().Patch(ctx, node, types.JSONPatchType, []byte(patch), metav1.PatchOptions{})
}

// Stop no-op
func (n *NodeRole) Stop() error {
	return nil
}

// Health-check interface
func (n *NodeRole) Healthy() error { return nil }
