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
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
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

	deploymentFound := &appsv1.Deployment{}
	err = r.Client.Get(ctx, types.NamespacedName{Name: site.Name, Namespace: site.Namespace}, deploymentFound)
	if err != nil && errors.IsNotFound(err) {
		mylog.Info("Deployment is not found, so we create it")
		//create pod
		deployment := r.createDeploymentSite(site)
		err := r.Client.Create(ctx, deployment)
		if err != nil {
			mylog.Error(err, "Failed to create new Deployment", "Deployment.Namespace", deployment.Namespace, "Deployment.Name", deployment.Name)
			return ctrl.Result{}, err
		}
		// Deployment created successfully - return and requeue
		return ctrl.Result{Requeue: true}, nil
	} else if err != nil {
		mylog.Error(err, "Failed to get Deployment")
		return ctrl.Result{}, err
	}

	updated := false
	recreate := false
	if *deploymentFound.Spec.Replicas != site.Spec.Replicas {
		deploymentFound.Spec.Replicas = &site.Spec.Replicas
		updated = true
	}

	if deploymentFound.Spec.Template.Spec.InitContainers[0].Args[2] != site.Spec.Git.Branch {
		deploymentFound.Spec.Template.Spec.InitContainers[0].Args[2] = site.Spec.Git.Branch
		recreate = true
		updated = true
	}

	if deploymentFound.Spec.Template.Spec.InitContainers[0].Args[3] != site.Spec.Git.Url {
		deploymentFound.Spec.Template.Spec.InitContainers[0].Args[3] = site.Spec.Git.Url
		recreate = true
		updated = true
	}

	//deploymentFound.Annotations.

	if updated {
		err = r.Client.Update(ctx, deploymentFound)
		if err != nil {
			mylog.Error(err, "Failed to update Deployment", "Deployment.Namespace", deploymentFound.Namespace, "Deployment.Name", deploymentFound.Name)
			return ctrl.Result{}, err
		}
		// Spec updated - return and requeue
		return ctrl.Result{Requeue: true}, nil
	}

	if recreate {
		pod := &corev1.Pod{}
		opts := []client.DeleteAllOfOption{
			client.InNamespace(site.Namespace),
			client.MatchingLabels{"app": site.Name},
			client.GracePeriodSeconds(5),
		}
		err := r.DeleteAllOf(ctx, pod, opts...)
		if err != nil {
			mylog.Error(err, "Failed to delete Pod from Deployment", "Deployment.Namespace", deploymentFound.Namespace, "Deployment.Name", deploymentFound.Name)
			return ctrl.Result{}, err
		}
	}

	serviceFound := &corev1.Service{}
	err = r.Client.Get(ctx, types.NamespacedName{Name: site.Name, Namespace: site.Namespace}, serviceFound)
	if err != nil && errors.IsNotFound(err) {
		mylog.Info("Service is not found, so we create it")
		service := r.createServiceSite(site)
		err := r.Client.Create(ctx, service)
		if err != nil {
			mylog.Error(err, "Failed to create new Service", "Service.Namespace", service.Namespace, "Service.Name", service.Name)
			return ctrl.Result{}, err
		}
		// Deployment created successfully - return and requeue
		return ctrl.Result{Requeue: true}, nil
	} else if err != nil {
		mylog.Error(err, "Failed to get Service")
		return ctrl.Result{}, err
	}

	ingressFound := &networkingv1.Ingress{}
	err = r.Client.Get(ctx, types.NamespacedName{Name: site.Name, Namespace: site.Namespace}, ingressFound)
	if err != nil && errors.IsNotFound(err) {
		mylog.Info("Ingress is not found, so we create it")
		ingress := r.createIngressSite(site)
		err := r.Client.Create(ctx, ingress)
		if err != nil {
			mylog.Error(err, "Failed to create new Ingress", "Ingress.Namespace", ingress.Namespace, "Ingress.Name", ingress.Name)
			return ctrl.Result{}, err
		}
		// Deployment created successfully - return and requeue
		return ctrl.Result{Requeue: true}, nil
	} else if err != nil {
		mylog.Error(err, "Failed to get createIngressSite")
		return ctrl.Result{}, err
	}

	if ingressFound.Spec.Rules[0].Host != site.Spec.Url {
		ingressFound.Spec.Rules[0].Host = site.Spec.Url
		err = r.Client.Update(ctx, ingressFound)
		if err != nil {
			mylog.Error(err, "Failed to update Ingress", "Ingress.Namespace", ingressFound.Namespace, "Ingress.Name", ingressFound.Name)
			return ctrl.Result{}, err
		}
		return ctrl.Result{Requeue: true}, nil
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *SiteReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&testv1.Site{}).
		Owns(&appsv1.Deployment{}).
		Owns(&corev1.Service{}).
		Owns(&networkingv1.Ingress{}).
		Complete(r)
}

func (r *SiteReconciler) createIngressSite(site *testv1.Site) *networkingv1.Ingress {
	public := "public"
	prefix := networkingv1.PathType("Prefix")
	labels := map[string]string{"app": site.Name}
	ingress := &networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:      site.Name,
			Namespace: site.Namespace,
			Labels:    labels,
		},
		Spec: networkingv1.IngressSpec{
			IngressClassName: &public,
			Rules: []networkingv1.IngressRule{{
				Host: site.Spec.Url,
				IngressRuleValue: networkingv1.IngressRuleValue{
					HTTP: &networkingv1.HTTPIngressRuleValue{
						Paths: []networkingv1.HTTPIngressPath{{
							Path:     "/",
							PathType: &prefix,
							Backend: networkingv1.IngressBackend{
								Service: &networkingv1.IngressServiceBackend{
									Name: site.Name,
									Port: networkingv1.ServiceBackendPort{Name: "http"},
								},
							},
						}},
					},
				},
			}},
		},
	}
	_ = ctrl.SetControllerReference(site, ingress, r.Scheme)
	return ingress
}

func (r *SiteReconciler) createServiceSite(site *testv1.Site) *corev1.Service {
	labels := map[string]string{"app": site.Name}
	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      site.Name,
			Namespace: site.Namespace,
			Labels:    labels,
		},
		Spec: corev1.ServiceSpec{
			Type:     "ClusterIP",
			Selector: labels,
			Ports: []corev1.ServicePort{{
				Name:       "http",
				Port:       80,
				TargetPort: intstr.IntOrString{IntVal: 80},
			}},
		},
	}
	_ = ctrl.SetControllerReference(site, service, r.Scheme)
	return service
}

func (r *SiteReconciler) createDeploymentSite(site *testv1.Site) *appsv1.Deployment {
	labels := map[string]string{"app": site.Name}
	image := "nginx"
	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      site.Name,
			Namespace: site.Namespace,
			Labels:    labels,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &site.Spec.Replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: labels,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Name:      site.Name,
					Namespace: site.Namespace,
					Labels:    labels,
				},
				Spec: corev1.PodSpec{
					InitContainers: []corev1.Container{{
						Image: "alpine/git",
						Args:  []string{"clone", "--branch", site.Spec.Git.Branch, site.Spec.Git.Url, "/data"},
						Name:  "git-init",
						VolumeMounts: []corev1.VolumeMount{{
							Name:      "data",
							MountPath: "/data",
						}},
					}},
					Containers: []corev1.Container{{
						Image: image,
						Name:  site.Name,
						Ports: []corev1.ContainerPort{{
							Name:          "http",
							ContainerPort: 80,
						}},
						VolumeMounts: []corev1.VolumeMount{{
							Name:      "data",
							MountPath: "/usr/share/nginx/html",
						}},
					}},
					Volumes: []corev1.Volume{{
						Name: "data",
					}},
				},
			},
		},
	}
	_ = ctrl.SetControllerReference(site, deployment, r.Scheme)
	return deployment
}
