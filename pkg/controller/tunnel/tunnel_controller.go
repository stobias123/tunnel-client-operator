package tunnel

import (
	"bytes"
	"context"
	"text/template"

	tunneldv1alpha1 "github.com/stobias123/tunnel-client-operator/pkg/apis/tunneld/v1alpha1"
	apps "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

var log = logf.Log.WithName("controller_tunnel")

/**
* USER ACTION REQUIRED: This is a scaffold file intended for the user to modify with their own Controller
* business logic.  Delete these comments after modifying this file.*
 */

// Add creates a new Tunnel Controller and adds it to the Manager. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) reconcile.Reconciler {
	return &ReconcileTunnel{client: mgr.GetClient(), scheme: mgr.GetScheme()}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New("tunnel-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to primary resource Tunnel
	err = c.Watch(&source.Kind{Type: &tunneldv1alpha1.Tunnel{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	// TODO(user): Modify this to be the types you create that are owned by the primary resource
	// Watch for changes to secondary resource Pods and requeue the owner Tunnel
	err = c.Watch(&source.Kind{Type: &corev1.Pod{}}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &tunneldv1alpha1.Tunnel{},
	})
	if err != nil {
		return err
	}

	return nil
}

// blank assignment to verify that ReconcileTunnel implements reconcile.Reconciler
var _ reconcile.Reconciler = &ReconcileTunnel{}

// ReconcileTunnel reconciles a Tunnel object
type ReconcileTunnel struct {
	// This client, initialized using mgr.Client() above, is a split client
	// that reads objects from the cache and writes to the apiserver
	client client.Client
	scheme *runtime.Scheme
}

// Reconcile reads that state of the cluster for a Tunnel object and makes changes based on the state read
// and what is in the Tunnel.Spec
// TODO(user): Modify this Reconcile function to implement your Controller logic.  This example creates
// a Pod as an example
// Note:
// The Controller will requeue the Request to be processed again if the returned error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (r *ReconcileTunnel) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	reqLogger := log.WithValues("Request.Namespace", request.Namespace, "Request.Name", request.Name)
	reqLogger.Info("Reconciling Tunnel")

	// Fetch the Tunnel instance
	instance := &tunneldv1alpha1.Tunnel{}
	err := r.client.Get(context.TODO(), request.NamespacedName, instance)
	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return reconcile.Result{}, err
	}

	// Check if the config already exists
	config := createTunnelConfigForCR(instance)
	// Set Tunnel instance as the owner and controller
	if err := controllerutil.SetControllerReference(instance, config, r.scheme); err != nil {
		return reconcile.Result{}, err
	}

	cmFound := &corev1.ConfigMap{}
	err = r.client.Get(context.TODO(), types.NamespacedName{Name: config.Name, Namespace: config.Namespace}, cmFound)
	if err != nil && errors.IsNotFound(err) {
		reqLogger.Info("Creating a new config", "config.Namespace", config.Namespace, "config.Name", config.Name)
		err = r.client.Create(context.TODO(), config)
		if err != nil {
			return reconcile.Result{}, err
		}

		// deployment created successfully - don't requeue
		// Dont return yet, we're not done.
		//return reconcile.Result{}, nil
	} else if err != nil {
		return reconcile.Result{}, err
	}

	// Check if this deployment already exists
	deployment := createServiceProxyForCR(instance, config)
	// Set Tunnel instance as the owner and controller
	if err := controllerutil.SetControllerReference(instance, deployment, r.scheme); err != nil {
		return reconcile.Result{}, err
	}

	found := &apps.Deployment{}
	err = r.client.Get(context.TODO(), types.NamespacedName{Name: deployment.Name, Namespace: deployment.Namespace}, found)
	if err != nil && errors.IsNotFound(err) {
		reqLogger.Info("Creating a new deployment", "deployment.Namespace", deployment.Namespace, "deployment.Name", deployment.Name)
		err = r.client.Create(context.TODO(), deployment)
		if err != nil {
			return reconcile.Result{}, err
		}

		// deployment created successfully - don't requeue
		// return reconcile.Result{}, nil
	} else if err != nil {
		return reconcile.Result{}, err
	}

	// deployment already exists - don't requeue
	// If we reach here then we're good to go.
	reqLogger.Info("Skip reconcile: deployment already exists", "deployment.Namespace", found.Namespace, "deployment.Name", found.Name)
	return reconcile.Result{}, nil
}

func createTunnelConfigForCR(cr *tunneldv1alpha1.Tunnel) *corev1.ConfigMap {
	tunnelTemplate := `server_addr: {{ .Spec.ServerAddr }}
tls_crt: /certs/client.crt
tls_key: /certs/client.key
tunnels:
  {{ .Name }}:
    proto: http
    {{ if .Spec.Auth }}
    auth: {{ .Spec.Auth }}
    {{ end }}
    addr: {{ .Spec.Addr }}
    host: {{ .Spec.Host }}`

	labels := map[string]string{
		"app": cr.Name,
	}

	var configMapString bytes.Buffer
	tmpl, err := template.New("tunnel_config").Parse(tunnelTemplate)
	if err != nil {
		panic(err)
	}
	tmpl.Execute(&configMapString, cr)
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "tunnel-config",
			Namespace: cr.Namespace,
			Labels:    labels,
		},
		Data: map[string]string{
			"tunnel_config.yaml": configMapString.String(),
		},
	}
}

// createServiceProxyForCR returns a pod with the same name/namespace as the cr
func createServiceProxyForCR(cr *tunneldv1alpha1.Tunnel, tunnelConfig *corev1.ConfigMap) *apps.Deployment {
	labels := map[string]string{
		"app": cr.Name,
	}
	var count = int32(1)
	return &apps.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cr.Name + "-proxy",
			Namespace: cr.Namespace,
			Labels:    labels,
		},
		Spec: apps.DeploymentSpec{
			Replicas: &count,
			Selector: &metav1.LabelSelector{
				MatchLabels: labels,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Name:      cr.Name + "-pod",
					Namespace: cr.Namespace,
					Labels:    labels,
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "go-http-proxy",
							Image: "stobias123/tunnel",
							Args: []string{
								"-config",
								"/config/tunnel_config.yaml",
								"start-all",
							},
							VolumeMounts: []corev1.VolumeMount{
								corev1.VolumeMount{
									Name:      "tunnel-config",
									ReadOnly:  true,
									MountPath: "/config/tunnel_config.yaml",
									SubPath:   "tunnel_config.yaml",
								},
							},
						},
					},
					Volumes: []corev1.Volume{
						corev1.Volume{
							Name: "tunnel-config",
							VolumeSource: corev1.VolumeSource{
								ConfigMap: &corev1.ConfigMapVolumeSource{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: tunnelConfig.ObjectMeta.Name,
									},
								},
							},
						},
					},
				},
			},
		},
	}
}
