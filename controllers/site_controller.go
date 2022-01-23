/*
Copyright 2022.

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
	testv1 "github.com/dtrouillet/site-operator/api/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// SiteReconciler reconciles a Site object
type SiteReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=test.faya.fr,resources=sites,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=test.faya.fr,resources=sites/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=test.faya.fr,resources=sites/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the Site object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.10.0/pkg/reconcile
func (r *SiteReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	mylog := log.FromContext(ctx)
	site := &testv1.Site{}
	err := r.Client.Get(ctx, req.NamespacedName, site)

	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			mylog.Info("Site resource not found. Ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		// Error reading the object - requeue the request.
		mylog.Error(err, "Failed to get Site")
		return ctrl.Result{}, err
	}

	podFound := &corev1.Pod{}
	err = r.Client.Get(ctx, types.NamespacedName{Name: site.Name, Namespace: site.Namespace}, podFound)
	if err != nil && errors.IsNotFound(err) {
		mylog.Info("Pod is not found, so we create it")
		//create pod
		pod := r.createPodSite(site)
		err := r.Client.Create(ctx, pod)
		if err != nil {
			mylog.Error(err, "Failed to create new Pod", "Pod.Namespace", pod.Namespace, "Pod.Name", pod.Name)
			return ctrl.Result{}, err
		}
		// Deployment created successfully - return and requeue
		return ctrl.Result{Requeue: true}, nil
	} else if err != nil {
		mylog.Error(err, "Failed to get Pod")
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *SiteReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&testv1.Site{}).
		Complete(r)
}

func (r *SiteReconciler) createPodSite(site *testv1.Site) *corev1.Pod {
	labels := map[string]string{"app": "site-test"}
	image := "nginx"
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      site.Name,
			Namespace: site.Namespace,
			Labels:    labels,
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{
				Image: image,
				Name:  site.Name,
			}},
		},
	}
	_ = ctrl.SetControllerReference(site, pod, r.Scheme)
	return pod
}