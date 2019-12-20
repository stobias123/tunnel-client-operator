package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// TunnelSpec defines the desired state of Tunnel
// Add custom validation using kubebuilder tags: https://book-v1.book.kubebuilder.io/beyond_basics/generating_crd.html
type TunnelSpec struct {
	ServerAddr string `json:"server_addr"`
	// Protocol specifies tunnel protocol, must be one of protocols known
	// by the server.
	Protocol string `json:"protocol,omitempty"`
	// Host specified HTTP request host, it's required for HTTP and WS
	// tunnels.
	Host string `json:"host"`
	// Auth specifies HTTP basic auth credentials in form "user:password",
	// if set server would protect HTTP and WS tunnels with basic auth.
	Auth string `json:"auth,omitempty"`
	// Addr specifies TCP address server would listen on, it's required
	// for TCP tunnels.
	Addr string `json:"addr"`
}

// TunnelStatus defines the observed state of Tunnel
type TunnelStatus struct {
	// Important: Run "operator-sdk generate k8s" to regenerate code after modifying this file
	// Add custom validation using kubebuilder tags: https://book-v1.book.kubebuilder.io/beyond_basics/generating_crd.html
	Connection string `json:"connection"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Tunnel is the Schema for the tunnels API
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=tunnels,scope=Namespaced
type Tunnel struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   TunnelSpec   `json:"spec,omitempty"`
	Status TunnelStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// TunnelList contains a list of Tunnel
type TunnelList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Tunnel `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Tunnel{}, &TunnelList{})
}
