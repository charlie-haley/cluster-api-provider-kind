/*
Copyright 2023.

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

package controllers

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/cluster-api/util"
	"sigs.k8s.io/cluster-api/util/annotations"
	"sigs.k8s.io/cluster-api/util/patch"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	infrav1 "github.com/charlie-haley/cluster-api-provider-kind/api/v1alpha1"
	"github.com/charlie-haley/cluster-api-provider-kind/pkg/kind"
	"github.com/charlie-haley/cluster-api-provider-kind/pkg/scope"
	"github.com/pkg/errors"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
)

// KindClusterReconciler reconciles a KindCluster object
type KindClusterReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=kindclusters,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=kindclusters/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=kindclusters/finalizers,verbs=update
// +kubebuilder:rbac:groups=cluster.x-k8s.io,resources=clusters;clusters/status,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=configmaps,verbs=get;list;watch;

func (r *KindClusterReconciler) Reconcile(ctx context.Context, req ctrl.Request) (_ ctrl.Result, resErr error) {
	log := log.FromContext(ctx)

	// fetch KindCluster
	kindCluster := &infrav1.KindCluster{}
	err := r.Get(ctx, req.NamespacedName, kindCluster)
	if err != nil {
		if k8serrors.IsNotFound(err) {
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, err
	}

	// fetch Cluster
	cluster, err := util.GetOwnerCluster(ctx, r.Client, kindCluster.ObjectMeta)
	if err != nil {
		return reconcile.Result{}, err
	}
	if cluster == nil {
		log.Info("Cluster Controller has not yet set OwnerRef")
		return reconcile.Result{}, nil
	}

	// check if cluster is paused, cancel reconcile if so
	if annotations.IsPaused(cluster, kindCluster) {
		log.Info("KindCluster or linked Cluster is marked as paused. Won't reconcile")
		return reconcile.Result{}, nil
	}

	helper, err := patch.NewHelper(kindCluster, r.Client)
	if err != nil {
		return reconcile.Result{}, errors.Wrap(err, "failed to init patch helper")
	}

	defer func() {
		log.Info("Patching cluster status to ready")
		err = helper.Patch(ctx, kindCluster, patch.WithOwnedConditions{
			Conditions: []clusterv1.ConditionType{
				clusterv1.ReadyCondition,
			},
		})
		if err != nil {
			fmt.Println("patching cluster object: %w", err)
			if resErr == nil {
				resErr = err
			}
		}
	}()

	// create the scope to be passed to other reconcile functions
	clusterScope, err := scope.NewClusterScope(scope.ClusterScopeParams{
		Log:         log,
		Client:      r.Client,
		Cluster:     cluster,
		KindCluster: kindCluster,
		Context:     ctx,
	})
	if err != nil {
		return reconcile.Result{}, errors.Errorf("failed to create scope: %+v", err)
	}

	// handle cluster deletion
	if !kindCluster.ObjectMeta.DeletionTimestamp.IsZero() {
		return r.reconcileDelete(clusterScope)
	}

	// handle cluster reconcile
	return r.reconcileNormal(clusterScope)
}

// reconcileNormal handles normal reconciles
func (r *KindClusterReconciler) reconcileNormal(clusterScope *scope.ClusterScope) (ctrl.Result, error) {
	log := clusterScope.Log
	log.Info("Reconciling KindCluster")

	kindCluster := clusterScope.KindCluster

	// add finalizer to the KindCluster
	controllerutil.AddFinalizer(kindCluster, infrav1.ClusterFinalizer)
	err := clusterScope.PatchObject()
	if err != nil {
		return ctrl.Result{}, err
	}

	var kindConfig string
	if clusterScope.KindCluster.Spec.KindConfig != nil {
		log.Info("Fetching KindCluster config map")
		cfg, err := r.getKindConfig(clusterScope)
		if err != nil {
			return ctrl.Result{}, err
		}
		kindConfig = *cfg
	}

	log.Info("Creating kind cluster")
	err = kind.CreateCluster(clusterScope, kindConfig)
	if err != nil {
		log.Error(err, "error creating kind cluster")
	}

	kc, err := kind.GetKubeConfig(clusterScope)
	if err != nil {
		log.Error(err, "error fetching kind cluster kubeconfig")
	}
	err = createCAPIKubeconfigSecret(kc, clusterScope)
	if err != nil {
		log.Error(err, "failed to create CAPI kubeconfig secret")
	}

	endpoint, err := getControlPlaneEndpoint(clusterScope)
	if err != nil {
		log.Error(err, "error fetching kind cluster control plane endpoint")
	}

	kindCluster.Spec.ControlPlaneEndpoint = *endpoint
	kindCluster.Status.Ready = true
	return ctrl.Result{}, nil
}

// reconcileDelete handles a deletion during the reconcile
func (r *KindClusterReconciler) reconcileDelete(clusterScope *scope.ClusterScope) (ctrl.Result, error) {
	clusterScope.Log.Info("Deleting KindCluster")
	err := kind.DeleteCluster(clusterScope)
	if err != nil {
		fmt.Println(err)
	}

	// remove finalizer
	controllerutil.RemoveFinalizer(clusterScope.KindCluster, infrav1.ClusterFinalizer)

	return reconcile.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *KindClusterReconciler) SetupWithManager(mgr ctrl.Manager, options controller.Options) error {
	return ctrl.NewControllerManagedBy(mgr).
		WithOptions(options).
		For(&infrav1.KindCluster{}).
		Complete(r)
}

// getKindConfig gets the Kind config map based on the reference in the KindCluster custom resource and returns the value as a string.
func (r *KindClusterReconciler) getKindConfig(clusterScope *scope.ClusterScope) (*string, error) {
	clusterScope.Log.Info("Fetching Kind ConfigMap")

	namespace := clusterScope.KindCluster.Spec.KindConfig.Namespace
	// if namespace is null, assume namespace of KindCluster
	if namespace == "" {
		namespace = clusterScope.KindCluster.ObjectMeta.Namespace
	}
	namespacedName := types.NamespacedName{
		Name:      clusterScope.KindCluster.Spec.KindConfig.Name,
		Namespace: namespace,
	}

	kindConfig := &corev1.ConfigMap{}
	err := r.Get(clusterScope.Context, namespacedName, kindConfig)
	if err != nil {
		if k8serrors.IsNotFound(err) {
			return nil, err
		}
	}
	cfg := kindConfig.Data[clusterScope.KindCluster.Spec.KindConfig.Key]
	return &cfg, nil
}

// createCAPIKubeconfigSecret creates a secret containing the kubeconfig on the management cluster
func createCAPIKubeconfigSecret(kubeconfigData string, clusterScope *scope.ClusterScope) error {
	clusterScope.Log.Info("Creating CAPI kubeconfig secret")
	kubeSecret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: fmt.Sprintf("%s-kubeconfig", clusterScope.Cluster.Name), Namespace: clusterScope.Cluster.Namespace}}
	_, err := controllerutil.CreateOrPatch(clusterScope.Context, clusterScope.Client, kubeSecret, func() error {
		if kubeSecret.Data == nil {
			kubeSecret.Data = make(map[string][]byte)
		}
		kubeSecret.Data["value"] = []byte(kubeconfigData)
		return nil
	})
	return err
}

// getControlPlaneEndpoint gets the Kind cluster control plane endpoint and returns a cluster api APIEndpoint
func getControlPlaneEndpoint(clusterScope *scope.ClusterScope) (*clusterv1.APIEndpoint, error) {
	cpe, err := kind.GetControlPlaneEndpoint(clusterScope)
	if err != nil {
		return nil, err
	}

	u, _ := url.Parse(cpe)
	s := strings.Split(u.Host, ":")

	host := s[0]
	port, err := strconv.Atoi(s[1])
	if err != nil {
		return nil, err
	}

	return &clusterv1.APIEndpoint{
		Host: u.Scheme + "://" + host,
		Port: int32(port),
	}, nil
}
