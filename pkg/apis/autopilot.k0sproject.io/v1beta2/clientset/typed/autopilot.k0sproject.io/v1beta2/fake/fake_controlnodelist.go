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

// Code generated by client-gen. DO NOT EDIT.

package fake

import (
	"context"

	v1beta2 "github.com/k0sproject/k0s/pkg/apis/autopilot.k0sproject.io/v1beta2"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	schema "k8s.io/apimachinery/pkg/runtime/schema"
	testing "k8s.io/client-go/testing"
)

// FakeControlNodeLists implements ControlNodeListInterface
type FakeControlNodeLists struct {
	Fake *FakeAutopilotV1beta2
}

var controlnodelistsResource = schema.GroupVersionResource{Group: "autopilot.k0sproject.io", Version: "v1beta2", Resource: "controlnodelists"}

var controlnodelistsKind = schema.GroupVersionKind{Group: "autopilot.k0sproject.io", Version: "v1beta2", Kind: "ControlNodeList"}

// Create takes the representation of a controlNodeList and creates it.  Returns the server's representation of the controlNodeList, and an error, if there is any.
func (c *FakeControlNodeLists) Create(ctx context.Context, controlNodeList *v1beta2.ControlNodeList, opts v1.CreateOptions) (result *v1beta2.ControlNodeList, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootCreateAction(controlnodelistsResource, controlNodeList), &v1beta2.ControlNodeList{})
	if obj == nil {
		return nil, err
	}
	return obj.(*v1beta2.ControlNodeList), err
}
