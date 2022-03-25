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
package controller

import (
	"bytes"
	"context"
	"fmt"
	"path"
	"time"

	"github.com/go-openapi/jsonpointer"
	k8sutil "github.com/k0sproject/k0s/pkg/kubernetes"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	v1informers "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"

	"github.com/k0sproject/k0s/internal/pkg/stringmap"
	"github.com/k0sproject/k0s/internal/pkg/templatewriter"
	"github.com/k0sproject/k0s/internal/pkg/users"
	"github.com/k0sproject/k0s/pkg/apis/k0s.k0sproject.io/v1beta1"
	"github.com/k0sproject/k0s/pkg/assets"
	"github.com/k0sproject/k0s/pkg/component"
	"github.com/k0sproject/k0s/pkg/constant"
	"github.com/k0sproject/k0s/pkg/supervisor"
)

var _ component.Component = &Autopilot{}
var _ component.ReconcilerComponent = &Autopilot{}

type Autopilot struct {
	K0sVars      constant.CfgVars
	supervisor   *supervisor.Supervisor
	client       kubernetes.Interface
	uid          int
	saver        manifestsSaver
	nodeInformer cache.SharedIndexInformer
	previousArgs stringmap.StringMap
	log          *logrus.Entry
}

func NewAutopilot(saver manifestsSaver, K0sVars constant.CfgVars, clientFactory k8sutil.ClientFactoryInterface) (*Autopilot, error) {
	client, err := clientFactory.GetClient()
	if err != nil {
		return nil, err
	}
	return &Autopilot{
		saver:        saver,
		K0sVars:      K0sVars,
		client:       client,
		nodeInformer: v1informers.NewNodeInformer(client, 1*time.Minute, nil),
		log:          logrus.WithField("component", "autopilot"),
	}, nil
}

// Init does currently nothing
func (a *Autopilot) Init(ctx context.Context) error {
	go a.nodeInformer.Run(ctx.Done())
	a.nodeInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			node, ok := obj.(*corev1.Node)
			if !ok {
				return
			}
			a.ensureNodeAnnotations(node)
		},
		UpdateFunc: func(_, obj interface{}) {
			node, ok := obj.(*corev1.Node)
			if !ok {
				return
			}
			a.ensureNodeAnnotations(node)
		},
	})

	var err error
	a.uid, err = users.GetUID(constant.ApiserverUser)
	if err != nil {
		a.log.Warning(fmt.Errorf("running autopilot as root: %w", err))
	}

	return assets.Stage(a.K0sVars.BinDir, "autopilot", constant.BinDirMode)
}

// Run reconciles the k0s default PSP rules
func (a *Autopilot) Run(_ context.Context) error {
	return nil
}

// Stop does currently nothing
func (a *Autopilot) Stop() error {
	if a.supervisor != nil {
		return a.supervisor.Stop()
	}
	return nil
}

func (a *Autopilot) Healthy() error { return nil }

// Reconcile detects changes in configuration and applies them to the component
func (a *Autopilot) Reconcile(_ context.Context, clusterConfig *v1beta1.ClusterConfig) error {
	if a.supervisor != nil {
		a.log.Info("reconcile has nothing to do")
		err := a.supervisor.Stop()
		a.supervisor = nil
		if err != nil {
			return err
		}
	}

	output := bytes.NewBuffer([]byte{})
	tw := templatewriter.TemplateWriter{
		Name:     "autopilot",
		Template: autopilotTemplate,
		Data:     struct{}{},
	}

	err := tw.WriteToBuffer(output)
	if err != nil {
		return fmt.Errorf("error writing autopilot template: %w", err)
	}

	err = a.saver.Save("autopilot.yaml", output.Bytes())
	if err != nil {
		return fmt.Errorf("error writing autopilot yaml: %w", err)
	}

	args := stringmap.StringMap{
		"mode":       "controller",
		"kubeconfig": path.Join(a.K0sVars.CertRootDir, "admin.conf"),
		"data-dir":   a.K0sVars.DataDir,
	}
	a.supervisor = &supervisor.Supervisor{
		Name:    "autopilot",
		BinPath: assets.BinPath("autopilot", a.K0sVars.BinDir),
		RunDir:  a.K0sVars.RunDir,
		DataDir: a.K0sVars.DataDir,
		Args:    args.ToDashedArgs(),
		UID:     a.uid,
	}
	a.previousArgs = args
	return a.supervisor.Supervise()
}

func (a *Autopilot) ensureNodeAnnotations(node *corev1.Node) {

	if _, ok := node.Annotations[constant.AutopilotAnnotationVersion]; !ok {
		_, err := a.addNodeAnnotation(node.Name, constant.AutopilotAnnotationVersion, "v2")
		if err != nil {
			a.log.Errorf("error adding node annotation %s: %v", constant.AutopilotAnnotationVersion, err)
		}
	}

	if _, ok := node.Annotations[constant.AutopilotAnnotationData]; !ok {
		_, err := a.addNodeAnnotation(node.Name, constant.AutopilotAnnotationData, "{}")
		if err != nil {
			a.log.Errorf("error adding node annotation %s: %v", constant.AutopilotAnnotationData, err)
		}
	}
}

func (a *Autopilot) addNodeAnnotation(node string, key string, value string) (*corev1.Node, error) {
	keyPath := fmt.Sprintf("/metadata/annotations/%s", jsonpointer.Escape(key))
	patch := fmt.Sprintf(`[{"op":"add", "path":"%s", "value":"%s" }]`, keyPath, value)
	return a.client.CoreV1().Nodes().Patch(context.TODO(), node, types.JSONPatchType, []byte(patch), metav1.PatchOptions{})
}

const autopilotTemplate = `
apiVersion: v1
kind: ServiceAccount
metadata:
  name: autopilot
  namespace: kube-system
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: autopilot-role-workers
rules:
  - apiGroups: ["autopilot.k0sproject.io"]
    resources: ["plans"]
    verbs: ["list", "watch"]

  - apiGroups: ["autopilot.k0sproject.io"]
    resources: ["plans/status"]
    verbs: ["update"]

  - apiGroups: [""]
    resources: ["nodes"]
    verbs: ["list", "patch", "update", "watch"]

  - apiGroups: [""]
    resources: ["pods"]
    verbs: ["list"]

  - apiGroups: [""]
    resources: ["pods/eviction"]
    verbs: ["create"]

  - apiGroups: ["apps"]
    resources: ["daemonsets"]
    verbs: ["get"]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: autopilot
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: autopilot-role-workers
subjects:
- kind: ServiceAccount
  name: autopilot
  namespace: kube-system
---
apiVersion: apps/v1
kind: DaemonSet
metadata:
  name: autopilot
  namespace: kube-system
spec:
  selector:
    matchLabels:
      name: autopilot
  template:
    metadata:
      labels:
        name: autopilot
    spec:
      hostPID: true
      serviceAccountName: autopilot
      tolerations:
      - operator: "Exists"
      containers:
      - name: autopilot
        image: quay.io/k0sproject/autopilot:edge
        args:
          - "--mode=worker"
        securityContext:
          privileged: true
          runAsUser: 0
        env:
        - name: AUTOPILOT_HOSTNAME
          valueFrom:
            fieldRef:
              fieldPath: spec.nodeName
        volumeMounts:
        - name: autopilot-run
          mountPath: /run/k0s
          readOnly: true
        - name: autopilot-usrlocalbin
          mountPath: /usr/local/bin
          readOnly: false
        livenessProbe:
          httpGet:
            path: /healthz
            port: 10802
          initialDelaySeconds: 3
          periodSeconds: 10
        readinessProbe:
          httpGet:
            path: /readyz
            port: 10802
          initialDelaySeconds: 3
          periodSeconds: 10
      volumes:
      - name: autopilot-run
        hostPath:
          path: /run/k0s
      - name: autopilot-usrlocalbin
        hostPath:
          path: /usr/local/bin
`
