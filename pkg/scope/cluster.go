package scope

import (
	"context"

	"github.com/pkg/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"

	infrav1 "github.com/charlie-haley/cluster-api-provider-kind/api/v1alpha1"
	"github.com/go-logr/logr"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/util/patch"
)

// ClusterScopeParams defines the input parameters used to create a new Scope.
type ClusterScopeParams struct {
	Log         logr.Logger
	Client      client.Client
	Cluster     *clusterv1.Cluster
	KindCluster *infrav1.KindCluster
	Context     context.Context
}

// NewClusterScope creates a new Scope from the supplied parameters.
// This is meant to be called for each reconcile iteration.
func NewClusterScope(params ClusterScopeParams) (*ClusterScope, error) {
	if params.Cluster == nil {
		return nil, errors.New("failed to generate new scope from nil Cluster")
	}
	if params.KindCluster == nil {
		return nil, errors.New("failed to generate new scope from nil KindCluster")
	}

	clusterScope := &ClusterScope{
		Log:         params.Log,
		Client:      params.Client,
		Cluster:     params.Cluster,
		KindCluster: params.KindCluster,
		Context:     params.Context,
	}

	helper, err := patch.NewHelper(params.KindCluster, params.Client)
	if err != nil {
		return nil, errors.Wrap(err, "failed to init patch helper")
	}

	clusterScope.patchHelper = helper

	return clusterScope, nil
}

// PatchObject persists the cluster configuration and status.
func (s *ClusterScope) PatchObject() error {
	return s.patchHelper.Patch(
		context.TODO(),
		s.KindCluster,
		patch.WithOwnedConditions{Conditions: []clusterv1.ConditionType{
			clusterv1.ReadyCondition,
		}})
}

// ClusterScope defines the basic context for an actuator to operate upon.
type ClusterScope struct {
	patchHelper *patch.Helper

	Client      client.Client
	Log         logr.Logger
	Cluster     *clusterv1.Cluster
	Context     context.Context
	KindCluster *infrav1.KindCluster
}
