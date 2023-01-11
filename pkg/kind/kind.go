package kind

import (
	"github.com/charlie-haley/cluster-api-provider-kind/pkg/scope"
	"github.com/pkg/errors"
	"sigs.k8s.io/kind/pkg/cluster"
	"sigs.k8s.io/yaml"
)

type kubeConfig struct {
	Clusters []struct {
		Name    string `json:"name"`
		Cluster struct {
			Server string `json:"server"`
		} `json:"cluster"`
	} `json:"clusters"`
}

// CreateCluster creates a Kind cluster
func CreateCluster(scope *scope.ClusterScope) error {
	provider := cluster.NewProvider()

	// create the cluster
	err := provider.Create(
		scope.Cluster.ObjectMeta.Name,
		cluster.CreateWithDisplayUsage(true),
		cluster.CreateWithDisplaySalutation(true),
	)
	if err != nil {
		return errors.Wrap(err, "failed to create kind cluster")
	}

	return nil
}

// DeleteCluster deletes a Kind cluster
func DeleteCluster(scope *scope.ClusterScope) error {
	provider := cluster.NewProvider()

	// delete individual cluster
	if err := provider.Delete(scope.Cluster.ObjectMeta.Name, ""); err != nil {
		return err
	}
	return nil
}

// GetKubeConfig gets the kubeconfig of the cluster as a string
func GetKubeConfig(scope *scope.ClusterScope) (string, error) {
	provider := cluster.NewProvider()

	return provider.KubeConfig(scope.Cluster.ObjectMeta.Name, false)
}

// GetControlPlaneEndpoint gets and parses the kubeconfig to get the control plane endpoint
func GetControlPlaneEndpoint(scope *scope.ClusterScope) (string, error) {
	kc, err := GetKubeConfig(scope)
	if err != nil {
		return "", err
	}

	var config kubeConfig
	err = yaml.Unmarshal([]byte(kc), &config)
	if err != nil {
		return "", err
	}

	endpoint := config.Clusters[0].Cluster.Server
	return endpoint, nil
}
