/*
Copyright 2026.

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
	"time"

	webv1 "example.com/nginx-operator/api/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	appsv1 "k8s.io/api/apps/v1"
)

// NginxClusterReconciler reconciles a NginxCluster object
type NginxClusterReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=web.example.com,resources=nginxclusters,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=web.example.com,resources=nginxclusters/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=web.example.com,resources=nginxclusters/finalizers,verbs=update
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the NginxCluster object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.25.0/pkg/reconcile
func (r *NginxClusterReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	// 1. 获取当前 CR 实例
	var cluster webv1.NginxCluster
	if err := r.Get(ctx, req.NamespacedName, &cluster); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// 2. 确保期望状态存在（创建/更新 Deployment）
	if err := r.ensureDeployment(ctx, &cluster); err != nil {
		return ctrl.Result{}, err
	}

	// 3. 更新 Status 字段
	cluster.Status.ReadyReplicas = r.getReadyReplicas(&cluster)
	r.Status().Update(ctx, &cluster)

	// 4. 返回结果（如需延迟重试则设置 RequeueAfter）
	return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
}

// SetupWithManager sets up the controller with the Manager. list watch
func (r *NginxClusterReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&webv1.NginxCluster{}, builder.WithPredicates(predicate.GenerationChangedPredicate{})).
		Named("nginxcluster").
		Complete(r)
}

func (r *NginxClusterReconciler) getReadyReplicas(cluster *webv1.NginxCluster) int32 {
	dep := &appsv1.Deployment{}
	err := r.Get(context.Background(), types.NamespacedName{
		Name:      cluster.Name + "-nginx",
		Namespace: cluster.Namespace,
	}, dep)
	if err != nil {
		return 0
	}
	return dep.Status.ReadyReplicas
}

func (r *NginxClusterReconciler) ensureDeployment(ctx context.Context, cluster *webv1.NginxCluster) error {
	replicas := int32(1)
	if cluster.Spec.Replicas != nil {
		replicas = *cluster.Spec.Replicas
	}
	image := cluster.Spec.Image
	if image == "" {
		image = "nginx:latest"
	}

	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cluster.Name + "-nginx",
			Namespace: cluster.Namespace,
		},
	}

	_, err := controllerutil.CreateOrUpdate(ctx, r.Client, dep, func() error {
		if dep.Labels == nil {
			dep.Labels = map[string]string{}
		}
		dep.Labels["app"] = cluster.Name + "-nginx"

		dep.Spec.Replicas = &replicas
		dep.Spec.Selector = &metav1.LabelSelector{
			MatchLabels: map[string]string{"app": cluster.Name + "-nginx"},
		}
		dep.Spec.Template.ObjectMeta.Labels = map[string]string{"app": cluster.Name + "-nginx"}
		dep.Spec.Template.Spec.Containers = []corev1.Container{
			{
				Name:  "nginx",
				Image: image,
				Ports: []corev1.ContainerPort{{ContainerPort: 80}},
			},
		}

		return controllerutil.SetControllerReference(cluster, dep, r.Scheme)
	})

	return err
}
