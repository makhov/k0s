/*
Copyright 2021 Mirantis
*/
// Code generated by client-gen. DO NOT EDIT.

package fake

import (
	"context"

	v1beta2 "github.com/k0sproject/k0s/pkg/autopilot/apis/autopilot.k0sproject.io/v1beta2"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	labels "k8s.io/apimachinery/pkg/labels"
	schema "k8s.io/apimachinery/pkg/runtime/schema"
	watch "k8s.io/apimachinery/pkg/watch"
	testing "k8s.io/client-go/testing"
)

// FakePlans implements PlanInterface
type FakePlans struct {
	Fake *FakeAutopilotV1beta2
}

var plansResource = schema.GroupVersionResource{Group: "autopilot.k0sproject.io", Version: "v1beta2", Resource: "plans"}

var plansKind = schema.GroupVersionKind{Group: "autopilot.k0sproject.io", Version: "v1beta2", Kind: "Plan"}

// Get takes name of the plan, and returns the corresponding plan object, and an error if there is any.
func (c *FakePlans) Get(ctx context.Context, name string, options v1.GetOptions) (result *v1beta2.Plan, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootGetAction(plansResource, name), &v1beta2.Plan{})
	if obj == nil {
		return nil, err
	}
	return obj.(*v1beta2.Plan), err
}

// List takes label and field selectors, and returns the list of Plans that match those selectors.
func (c *FakePlans) List(ctx context.Context, opts v1.ListOptions) (result *v1beta2.PlanList, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootListAction(plansResource, plansKind, opts), &v1beta2.PlanList{})
	if obj == nil {
		return nil, err
	}

	label, _, _ := testing.ExtractFromListOptions(opts)
	if label == nil {
		label = labels.Everything()
	}
	list := &v1beta2.PlanList{ListMeta: obj.(*v1beta2.PlanList).ListMeta}
	for _, item := range obj.(*v1beta2.PlanList).Items {
		if label.Matches(labels.Set(item.Labels)) {
			list.Items = append(list.Items, item)
		}
	}
	return list, err
}

// Watch returns a watch.Interface that watches the requested plans.
func (c *FakePlans) Watch(ctx context.Context, opts v1.ListOptions) (watch.Interface, error) {
	return c.Fake.
		InvokesWatch(testing.NewRootWatchAction(plansResource, opts))
}

// Create takes the representation of a plan and creates it.  Returns the server's representation of the plan, and an error, if there is any.
func (c *FakePlans) Create(ctx context.Context, plan *v1beta2.Plan, opts v1.CreateOptions) (result *v1beta2.Plan, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootCreateAction(plansResource, plan), &v1beta2.Plan{})
	if obj == nil {
		return nil, err
	}
	return obj.(*v1beta2.Plan), err
}

// Update takes the representation of a plan and updates it. Returns the server's representation of the plan, and an error, if there is any.
func (c *FakePlans) Update(ctx context.Context, plan *v1beta2.Plan, opts v1.UpdateOptions) (result *v1beta2.Plan, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootUpdateAction(plansResource, plan), &v1beta2.Plan{})
	if obj == nil {
		return nil, err
	}
	return obj.(*v1beta2.Plan), err
}

// Delete takes name of the plan and deletes it. Returns an error if one occurs.
func (c *FakePlans) Delete(ctx context.Context, name string, opts v1.DeleteOptions) error {
	_, err := c.Fake.
		Invokes(testing.NewRootDeleteActionWithOptions(plansResource, name, opts), &v1beta2.Plan{})
	return err
}
