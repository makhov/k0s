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

package workerconfig

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/k0sproject/k0s/internal/testutil"
	"github.com/k0sproject/k0s/pkg/apis/k0s.k0sproject.io/v1beta1"
	"github.com/k0sproject/k0s/pkg/constant"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	u "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/sirupsen/logrus"
	"sigs.k8s.io/yaml"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/client-go/kubernetes/typed/core/v1/fake"
	k8stesting "k8s.io/client-go/testing"
)

type (
	obj = map[string]any
	arr = []any
)

func TestReconciler_Lifecycle(t *testing.T) {
	cleaner := newMockCleaner()
	clients := testutil.NewFakeClientFactory()
	underTest, err := NewReconciler(
		constant.GetConfig(t.TempDir()),
		&v1beta1.ClusterSpec{
			API: &v1beta1.APISpec{},
			Network: &v1beta1.Network{
				ClusterDomain: "test.local",
				ServiceCIDR:   "99.99.99.0/24",
			},
		},
		clients, true,
	)
	require.NoError(t, err)
	underTest.cleaner = cleaner

	log := logrus.New()
	log.SetLevel(logrus.DebugLevel)
	underTest.log = log

	t.Run("fails_to_start_without_init", func(t *testing.T) {
		err := underTest.Start(context.TODO())
		require.Error(t, err)
		require.Equal(t, "cannot start: workerconfig.reconcilerCreated", err.Error())
	})

	t.Run("init", func(t *testing.T) {
		assert.NoError(t, underTest.Init(context.TODO()))
		assert.Equal(t, "init", cleaner.state.Load())
	})

	t.Run("another_init_fails", func(t *testing.T) {
		err := underTest.Init(context.TODO())
		require.Error(t, err)
		assert.Equal(t, "cannot initialize: workerconfig.reconcilerInitialized", err.Error())
		assert.Equal(t, "init", cleaner.state.Load())
	})

	mockApplier := installMockApplier(t, underTest)
	mockKubernetesEndpoints(t, clients)

	stopTest := func() func(t *testing.T) {
		var once sync.Once
		return func(t *testing.T) {
			once.Do(func() {
				assert.NoError(t, underTest.Stop())
				assert.Equal(t, "stop", cleaner.state.Load())
			})
		}
	}()

	t.Run("starts", func(runT *testing.T) {
		require.NoError(runT, underTest.Start(context.TODO()))
		t.Cleanup(func() { stopTest(t) })
		assert.Equal(t, "init", cleaner.state.Load())
	})

	t.Run("another_start_fails", func(t *testing.T) {
		err := underTest.Start(context.TODO())
		require.Error(t, err)
		assert.Equal(t, "cannot start: workerconfig.reconcilerStarted", err.Error())
		assert.Equal(t, "init", cleaner.state.Load())
	})

	t.Run("reconciles", func(t *testing.T) {
		applied := mockApplier.expectApply(t, nil)
		require.NoError(t, underTest.Reconcile(context.TODO(), &v1beta1.ClusterConfig{
			Spec: &v1beta1.ClusterSpec{
				Network: &v1beta1.Network{
					ClusterDomain: "reconcile.local",
				},
				Images: &v1beta1.ClusterImages{
					DefaultPullPolicy: string(corev1.PullNever),
				},
			},
		}))
		assert.NotEmpty(t, applied(), "Expected some resources to be applied")
		cleaner.awaitState(t, "reconciled")
	})

	t.Run("stops", func(t *testing.T) {
		stopTest(t)
	})

	t.Run("stop_may_be_called_again", func(t *testing.T) {
		require.NoError(t, underTest.Stop())
		cleaner.assertState(t, "stop")
	})

	t.Run("reinit_fails", func(t *testing.T) {
		err := underTest.Init(context.TODO())
		require.Error(t, err)
		assert.Equal(t, "cannot initialize: workerconfig.reconcilerStopped", err.Error())
		cleaner.assertState(t, "stop")
	})

	t.Run("restart_fails", func(t *testing.T) {
		err := underTest.Start(context.TODO())
		require.Error(t, err)
		assert.Equal(t, "cannot start: workerconfig.reconcilerStopped", err.Error())
		cleaner.assertState(t, "stop")
	})
}

func TestReconciler_ResourceGeneration(t *testing.T) {
	cleaner := newMockCleaner()
	clients := testutil.NewFakeClientFactory()
	underTest, err := NewReconciler(
		constant.GetConfig(t.TempDir()),
		&v1beta1.ClusterSpec{
			API: &v1beta1.APISpec{},
			Network: &v1beta1.Network{
				ClusterDomain: "test.local",
				ServiceCIDR:   "99.99.99.0/24",
			},
		},
		clients, true,
	)
	require.NoError(t, err)
	underTest.cleaner = cleaner

	log := logrus.New()
	log.SetLevel(logrus.DebugLevel)
	underTest.log = log

	require.NoError(t, underTest.Init(context.TODO()))

	mockKubernetesEndpoints(t, clients)
	mockApplier := installMockApplier(t, underTest)

	require.NoError(t, underTest.Start(context.TODO()))
	t.Cleanup(func() {
		assert.NoError(t, underTest.Stop())
	})

	applied := mockApplier.expectApply(t, nil)

	require.NoError(t, underTest.Reconcile(context.TODO(), &v1beta1.ClusterConfig{
		Spec: &v1beta1.ClusterSpec{
			WorkerProfiles: v1beta1.WorkerProfiles{{
				Name:   "profile_XXX",
				Config: []byte(`{"authentication": {"anonymous": {"enabled": true}}}`),
			}, {
				Name:   "profile_YYY",
				Config: []byte(`{"authentication": {"webhook": {"cacheTTL": "15s"}}}`),
			}},
			Network: &v1beta1.Network{
				ClusterDomain: "test.local",
				ServiceCIDR:   "10.254.254.0/24",
			},
			Images: &v1beta1.ClusterImages{
				DefaultPullPolicy: string(corev1.PullNever),
			},
		},
	}))

	configMaps := map[string]func(t *testing.T, expected obj){
		"worker-config-default-1.25": func(t *testing.T, expected obj) {
			require.NoError(t, u.SetNestedField(expected, true, "cgroupsPerQOS"))
		},

		"worker-config-default-windows-1.25": func(t *testing.T, expected obj) {
			require.NoError(t, u.SetNestedField(expected, false, "cgroupsPerQOS"))
		},

		"worker-config-profile_XXX-1.25": func(t *testing.T, expected obj) {
			require.NoError(t, u.SetNestedField(expected, true, "authentication", "anonymous", "enabled"))
		},

		"worker-config-profile_YYY-1.25": func(t *testing.T, expected obj) {
			require.NoError(t, u.SetNestedField(expected, "15s", "authentication", "webhook", "cacheTTL"))
		},
	}

	appliedResources := applied()
	assert.Len(t, appliedResources, len(configMaps)+4)

	for name, mod := range configMaps {
		t.Run(name, func(t *testing.T) {
			kubelet := requireKubelet(t, appliedResources, name)
			expected := makeKubeletConfig(t, func(expected obj) { mod(t, expected) })
			assert.JSONEq(t, expected, kubelet)
		})
	}

	const rbacName = "system:bootstrappers:worker-config"

	t.Run("Role", func(t *testing.T) {
		role := find(t, "Expected to find a Role named "+rbacName,
			appliedResources, func(resource *u.Unstructured) bool {
				return resource.GetKind() == "Role" && resource.GetName() == rbacName
			},
		)

		rules, ok, err := u.NestedSlice(role.Object, "rules")
		require.NoError(t, err)
		require.True(t, ok, "No rules field")
		require.Len(t, rules, 1, "Expected a single rule")

		rule, ok := rules[0].(obj)
		require.True(t, ok, "Invalid rule")

		resourceNames, ok, err := u.NestedStringSlice(rule, "resourceNames")
		require.NoError(t, err)
		require.True(t, ok, "No resourceNames field")

		assert.Len(t, resourceNames, len(configMaps))
		for expected := range configMaps {
			assert.Contains(t, resourceNames, expected)
		}
	})

	t.Run("RoleBinding", func(t *testing.T) {
		binding := find(t, "Expected to find a RoleBinding named "+rbacName,
			appliedResources, func(resource *u.Unstructured) bool {
				return resource.GetKind() == "RoleBinding" && resource.GetName() == rbacName
			},
		)

		roleRef, ok, err := u.NestedMap(binding.Object, "roleRef")
		if assert.NoError(t, err) && assert.True(t, ok, "No roleRef field") {
			expected := obj{
				"apiGroup": "rbac.authorization.k8s.io",
				"kind":     "Role",
				"name":     rbacName,
			}

			assert.Equal(t, expected, roleRef)
		}

		subjects, ok, err := u.NestedSlice(binding.Object, "subjects")
		if assert.NoError(t, err) && assert.True(t, ok, "No subjects field") {
			expected := arr{obj{
				"apiGroup": "rbac.authorization.k8s.io",
				"kind":     "Group",
				"name":     "system:bootstrappers",
			}, obj{
				"apiGroup": "rbac.authorization.k8s.io",
				"kind":     "Group",
				"name":     "system:nodes",
			}}

			assert.Equal(t, expected, subjects)
		}
	})
}

func TestReconciler_ReconcilesOnChangesOnly(t *testing.T) {
	cluster := v1beta1.DefaultClusterConfig(nil)
	cleaner := newMockCleaner()
	clients := testutil.NewFakeClientFactory()
	underTest, err := NewReconciler(
		constant.GetConfig(t.TempDir()),
		&v1beta1.ClusterSpec{
			API: &v1beta1.APISpec{},
			Network: &v1beta1.Network{
				ClusterDomain: "test.local",
				ServiceCIDR:   "99.99.99.0/24",
			},
		},
		clients, true,
	)
	require.NoError(t, err)
	underTest.cleaner = cleaner

	log := logrus.New()
	log.SetLevel(logrus.DebugLevel)
	underTest.log = log

	require.NoError(t, underTest.Init(context.TODO()))

	mockKubernetesEndpoints(t, clients)
	mockApplier := installMockApplier(t, underTest)

	require.NoError(t, underTest.Start(context.TODO()))
	t.Cleanup(func() {
		assert.NoError(t, underTest.Stop())
	})

	expectApply := func(t *testing.T) {
		t.Helper()
		applied := mockApplier.expectApply(t, nil)
		assert.NoError(t, underTest.Reconcile(context.TODO(), cluster))
		appliedResources := applied()
		assert.NotEmpty(t, applied, "Expected some resources to be applied")

		for _, r := range appliedResources {
			pp, found, err := u.NestedString(r.Object, "data", "defaultImagePullPolicy")
			assert.NoError(t, err)
			if found {
				t.Logf("%s/%s: %s", r.GetKind(), r.GetName(), pp)
			}
		}

		cleaner.awaitState(t, "reconciled")
	}

	expectCached := func(t *testing.T) {
		t.Helper()
		assert.NoError(t, underTest.Reconcile(context.TODO(), cluster))
	}

	expectApplyButFail := func(t *testing.T) {
		t.Helper()
		applied := mockApplier.expectApply(t, assert.AnError)
		assert.ErrorIs(t, underTest.Reconcile(context.TODO(), cluster), assert.AnError)
		assert.NotEmpty(t, applied(), "Expected some resources to be applied")
	}

	// Set some value that affects worker configs.
	cluster.Spec.WorkerProfiles = v1beta1.WorkerProfiles{{Name: "foo", Config: json.RawMessage(`{"nodeLeaseDurationSeconds": 1}`)}}
	t.Run("first_time_apply", expectApply)
	t.Run("second_time_cached", expectCached)

	// Change that value, so that configs need to be reapplied.
	cluster.Spec.WorkerProfiles = v1beta1.WorkerProfiles{{Name: "foo", Config: json.RawMessage(`{"nodeLeaseDurationSeconds": 2}`)}}
	t.Run("third_time_apply_fails", expectApplyButFail)

	// After an error, expect a reapplication in any case.
	t.Run("fourth_time_apply", expectApply)

	// Even if the last successfully applied config is restored, expect it to be applied after a failure.
	cluster.Spec.WorkerProfiles = v1beta1.WorkerProfiles{{Name: "foo", Config: json.RawMessage(`{"nodeLeaseDurationSeconds": 1}`)}}
	t.Run("fifth_time_apply", expectApply)
	t.Run("sixth_time_cached", expectCached)
}

func TestReconciler_Cleaner_CleansUpManifestsOnInit(t *testing.T) {
	k0sVars := constant.GetConfig(t.TempDir())
	folder := filepath.Join(k0sVars.ManifestsDir, "kubelet")
	file := filepath.Join(folder, "kubelet-config.yaml")
	unrelatedFile := filepath.Join(folder, "unrelated")
	require.NoError(t, os.MkdirAll(folder, 0755))
	require.NoError(t, os.WriteFile(file, []byte("foo"), 0644))
	require.NoError(t, os.WriteFile(unrelatedFile, []byte("foo"), 0644))

	t.Run("leaves_unrelated_files_alone", func(t *testing.T) {
		underTest, err := NewReconciler(
			k0sVars,
			&v1beta1.ClusterSpec{
				API: &v1beta1.APISpec{},
				Network: &v1beta1.Network{
					ClusterDomain: "test.local",
					ServiceCIDR:   "99.99.99.0/24",
				},
			},
			testutil.NewFakeClientFactory(), true,
		)
		require.NoError(t, err)

		assert.NoError(t, underTest.Init(context.TODO()))

		assert.NoFileExists(t, file, "Expected the deprecated file to be deleted.")
		assert.FileExists(t, unrelatedFile, "Expected the unrelated file to be untouched.")
	})

	require.NoError(t, os.Remove(unrelatedFile))

	t.Run("prunes_empty_folder", func(t *testing.T) {
		underTest, err := NewReconciler(
			k0sVars,
			&v1beta1.ClusterSpec{
				API: &v1beta1.APISpec{},
				Network: &v1beta1.Network{
					ClusterDomain: "test.local",
					ServiceCIDR:   "99.99.99.0/24",
				},
			},
			testutil.NewFakeClientFactory(), true,
		)
		require.NoError(t, err)

		assert.NoError(t, underTest.Init(context.TODO()))

		assert.NoDirExists(t, folder, "Expected the empty deprecated folder to be deleted.")
		assert.DirExists(t, k0sVars.ManifestsDir, "Expected the manifests folder to be untouched.")
	})
}

func TestReconciler_ReconcileLoopClosesDoneChannels(t *testing.T) {
	underTest := Reconciler{
		log:     logrus.New(),
		cleaner: newMockCleaner(),
	}
	underTest.cleaner.init()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Prepare update channel for three updates
	updates, firstDone, thirdDone := make(chan updateFunc, 3), make(chan error, 1), make(chan error, 1)

	// Put in the first update: It will cancel the context when done and then
	// send the other updates.
	updates <- func(*snapshot) chan<- error { return firstDone }
	go func() {
		defer close(updates) // No more updates after the ones from below.

		<-firstDone
		cancel()

		// Put in the second update with a nil done channel that should be ignored.
		updates <- func(*snapshot) chan<- error { return nil }

		// Put in the third update that should receive the context's error.
		updates <- func(*snapshot) chan<- error { return thirdDone }
	}()

	underTest.runReconcileLoop(ctx, updates, nil)

	select {
	// The first channel should have been consumed by the goroutine that closes
	// the context and the update channel. The reconcile method should return
	// only after the update channel has been closed.
	case _, ok := <-firstDone:
		assert.False(t, ok, "Unexpected element in first done channel")
	default:
		assert.Fail(t, "First done channel not closed")
	}

	assert.Len(t, thirdDone, 1, "Third done channel should contain an element")
	assert.ErrorIs(t, <-thirdDone, ctx.Err(), "Third done channel should contain the context's error")
	select {
	case _, ok := <-thirdDone:
		assert.False(t, ok, "Unexpected element in third done channel")
	default:
		assert.Fail(t, "Third done channel not closed")
	}
}

func requireKubelet(t *testing.T, resources []*u.Unstructured, name string) string {
	configMap := find(t, "No ConfigMap found with name "+name,
		resources, func(resource *u.Unstructured) bool {
			return resource.GetKind() == "ConfigMap" && resource.GetName() == name
		},
	)
	kubeletConfigYAML, ok, err := u.NestedString(configMap.Object, "data", "kubeletConfiguration")
	require.NoError(t, err)
	require.True(t, ok, "No data.kubeletConfiguration field")
	kubeletConfigJSON, err := yaml.YAMLToJSONStrict([]byte(kubeletConfigYAML))
	require.NoError(t, err)
	return string(kubeletConfigJSON)
}

func find[T any](t *testing.T, failureMessage string, items []T, filter func(T) bool) (item T) {
	for _, item := range items {
		if filter(item) {
			return item
		}
	}

	require.Fail(t, failureMessage)
	return item
}

func makeKubeletConfig(t *testing.T, mods ...func(obj)) string {
	kubelet := u.Unstructured{
		Object: obj{
			"apiVersion": "kubelet.config.k8s.io/v1beta1",
			"authentication": obj{
				"anonymous": obj{},
				"webhook": obj{
					"cacheTTL": "0s",
				},
				"x509": obj{},
			},
			"authorization": obj{
				"webhook": obj{
					"cacheAuthorizedTTL":   "0s",
					"cacheUnauthorizedTTL": "0s",
				},
			},
			"clusterDNS": arr{
				"99.99.99.10",
			},
			"clusterDomain":                    "test.local",
			"cpuManagerReconcilePeriod":        "0s",
			"eventRecordQPS":                   0,
			"evictionPressureTransitionPeriod": "0s",
			"failSwapOn":                       false,
			"fileCheckFrequency":               "0s",
			"httpCheckFrequency":               "0s",
			"imageMinimumGCAge":                "0s",
			"kind":                             "KubeletConfiguration",
			"logging": obj{
				"flushFrequency": 0,
				"options": obj{
					"json": obj{
						"infoBufferSize": "0",
					},
				},
				"verbosity": 0,
			},
			"memorySwap":                      obj{},
			"nodeStatusReportFrequency":       "0s",
			"nodeStatusUpdateFrequency":       "0s",
			"rotateCertificates":              true,
			"runtimeRequestTimeout":           "0s",
			"serverTLSBootstrap":              true,
			"shutdownGracePeriod":             "0s",
			"shutdownGracePeriodCriticalPods": "0s",
			"streamingConnectionIdleTimeout":  "0s",
			"syncFrequency":                   "0s",
			"tlsCipherSuites": arr{
				"TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256",
				"TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384",
				"TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305",
				"TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256",
				"TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384",
				"TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305",
				"TLS_RSA_WITH_AES_128_GCM_SHA256",
				"TLS_RSA_WITH_AES_256_GCM_SHA384",
			},
			"volumeStatsAggPeriod": "0s",
		},
	}

	for _, mod := range mods {
		mod(kubelet.Object)
	}

	json, err := kubelet.MarshalJSON()
	require.NoError(t, err)
	return string(json)
}

type mockApplier struct {
	ptr atomic.Pointer[[]mockApplierCall]
}

type mockApplierCall = func(resources) error

func (m *mockApplier) expectApply(t *testing.T, retval error) func() resources {
	ch := make(chan resources, 1)

	recv := func() resources {
		select {
		case lastCalled, ok := <-ch:
			if !ok {
				require.Fail(t, "Channel closed unexpectedly")
			}
			return lastCalled

		case <-time.After(10 * time.Second):
			require.Fail(t, "Timed out while waiting for call to apply()")
			return nil // function diverges above
		}
	}

	send := func(r resources) error {
		defer close(ch)
		if r == nil { // called during test cleanup
			return nil
		}

		ch <- r
		return retval
	}

	for {
		calls := m.ptr.Load()
		len := len(*calls)
		newCalls := make([]mockApplierCall, len+1)
		copy(newCalls, *calls)
		newCalls[len] = send
		if m.ptr.CompareAndSwap(calls, &newCalls) {
			break
		}
	}

	return recv
}

func installMockApplier(t *testing.T, underTest *Reconciler) *mockApplier {
	mockApplier := mockApplier{}
	mockApplier.ptr.Store(new([]mockApplierCall))

	underTest.mu.Lock()
	defer underTest.mu.Unlock()

	underTestState := underTest.state
	initialized, ok := underTestState.(reconcilerInitialized)
	require.True(t, ok, "unexpected state: %T", underTestState)
	require.NotNil(t, initialized.apply)
	t.Cleanup(func() {
		for _, call := range *mockApplier.ptr.Swap(nil) {
			assert.NoError(t, call(nil))
		}
	})

	initialized.apply = func(ctx context.Context, r resources) error {
		if r == nil {
			panic("cannot call apply() with nil resources")
		}

		for {
			expected := mockApplier.ptr.Load()
			if len(*expected) < 1 {
				panic("unexpected call to apply")
			}

			newExpected := (*expected)[1:]
			if mockApplier.ptr.CompareAndSwap(expected, &newExpected) {
				return (*expected)[0](r)
			}
		}
	}

	underTest.state = initialized
	return &mockApplier
}

func mockKubernetesEndpoints(t *testing.T, clients testutil.FakeClientFactory) {
	client, err := clients.GetClient()
	require.NoError(t, err)

	ep := corev1.Endpoints{
		ObjectMeta: metav1.ObjectMeta{ResourceVersion: t.Name()},
		Subsets: []corev1.EndpointSubset{{
			Addresses: []corev1.EndpointAddress{
				{IP: "127.10.10.1"},
			},
			Ports: []corev1.EndpointPort{
				{Name: "https", Port: 6443, Protocol: corev1.ProtocolTCP},
			},
		}},
	}

	epList := corev1.EndpointsList{
		ListMeta: metav1.ListMeta{ResourceVersion: t.Name()},
		Items:    []corev1.Endpoints{ep},
	}

	_, err = client.CoreV1().Endpoints("default").Create(context.TODO(), ep.DeepCopy(), metav1.CreateOptions{})
	require.NoError(t, err)

	clients.Client.CoreV1().(*fake.FakeCoreV1).PrependReactor("list", "endpoints", func(k8stesting.Action) (bool, runtime.Object, error) {
		return true, epList.DeepCopy(), nil
	})
}

type mockCleaner struct {
	mu    sync.Cond
	state atomic.Value
}

func newMockCleaner() *mockCleaner {
	return &mockCleaner{
		mu: sync.Cond{L: new(sync.Mutex)},
	}
}

func (m *mockCleaner) assertState(t *testing.T, state string) {
	t.Helper()
	assert.Equal(t, state, m.state.Load(), "Unexpected cleaner state")
}

func (m *mockCleaner) awaitState(t *testing.T, state string) {
	t.Helper()

	var timeout bool
	timeouter := time.AfterFunc(10*time.Second, func() {
		m.mu.L.Lock()
		defer m.mu.L.Unlock()
		timeout = true
		m.mu.Broadcast()
	})
	defer timeouter.Stop()

	m.mu.L.Lock()
	defer m.mu.L.Unlock()

	for {
		if timeout {
			require.Fail(t, "Timeout while awaiting cleaner state", state)
		}
		if m.state.Load() == state {
			return
		}
		m.mu.Wait()
	}
}

func (m *mockCleaner) init() {
	if !m.state.CompareAndSwap(nil, "init") {
		panic(fmt.Sprintf("unexpected call to init(): %v", m.state.Load()))
	}
	m.mu.Broadcast()
}

func (m *mockCleaner) reconciled(ctx context.Context) {
	state := m.state.Load()
	if (state != "init" && state != "reconciled") || !m.state.CompareAndSwap(state, "reconciled") {
		panic(fmt.Sprintf("unexpected call to reconciled(): %v", state))
	}

	m.mu.Broadcast()
}

func (m *mockCleaner) stop() {
	state := m.state.Load()
	if state == "stop" || !m.state.CompareAndSwap(state, "stop") {
		panic(fmt.Sprintf("unexpected call to stop(): %v", state))
	}
	m.mu.Broadcast()
}
