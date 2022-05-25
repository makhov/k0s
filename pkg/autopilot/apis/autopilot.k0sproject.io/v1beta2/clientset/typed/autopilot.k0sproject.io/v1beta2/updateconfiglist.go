/*
Copyright 2021 Mirantis
*/
// Code generated by client-gen. DO NOT EDIT.

package v1beta2

import (
	"context"

	v1beta2 "github.com/k0sproject/k0s/pkg/autopilot/apis/autopilot.k0sproject.io/v1beta2"
	scheme "github.com/k0sproject/k0s/pkg/autopilot/apis/autopilot.k0sproject.io/v1beta2/clientset/scheme"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	rest "k8s.io/client-go/rest"
)

// UpdateConfigListsGetter has a method to return a UpdateConfigListInterface.
// A group's client should implement this interface.
type UpdateConfigListsGetter interface {
	UpdateConfigLists() UpdateConfigListInterface
}

// UpdateConfigListInterface has methods to work with UpdateConfigList resources.
type UpdateConfigListInterface interface {
	Create(ctx context.Context, updateConfigList *v1beta2.UpdateConfigList, opts v1.CreateOptions) (*v1beta2.UpdateConfigList, error)
	UpdateConfigListExpansion
}

// updateConfigLists implements UpdateConfigListInterface
type updateConfigLists struct {
	client rest.Interface
}

// newUpdateConfigLists returns a UpdateConfigLists
func newUpdateConfigLists(c *AutopilotV1beta2Client) *updateConfigLists {
	return &updateConfigLists{
		client: c.RESTClient(),
	}
}

// Create takes the representation of a updateConfigList and creates it.  Returns the server's representation of the updateConfigList, and an error, if there is any.
func (c *updateConfigLists) Create(ctx context.Context, updateConfigList *v1beta2.UpdateConfigList, opts v1.CreateOptions) (result *v1beta2.UpdateConfigList, err error) {
	result = &v1beta2.UpdateConfigList{}
	err = c.client.Post().
		Resource("updateconfiglists").
		VersionedParams(&opts, scheme.ParameterCodec).
		Body(updateConfigList).
		Do(ctx).
		Into(result)
	return
}
