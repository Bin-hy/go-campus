// Package v1 WebApp 自定义资源的类型定义（与 manifests/10_crd/crd-webapp.yaml 对应）。
package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// WebAppSpec WebApp 的期望状态。
type WebAppSpec struct {
	// Replicas 期望副本数。
	Replicas int32 `json:"replicas"`
	// Image 容器镜像。
	Image string `json:"image"`
}

// WebAppStatus WebApp 的实际状态。
type WebAppStatus struct {
	// ReadyReplicas 当前就绪副本数（由控制器回写）。
	ReadyReplicas int32 `json:"readyReplicas,omitempty"`
}

// WebApp 是 CRD apps.example.com 的 Go 表示。
type WebApp struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   WebAppSpec   `json:"spec,omitempty"`
	Status WebAppStatus `json:"status,omitempty"`
}

// WebAppList WebApp 列表。
type WebAppList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []WebApp `json:"items"`
}

// GroupVersion 该资源组的版本。
var GroupVersion = schema.GroupVersion{Group: "apps.example.com", Version: "v1"}

func (in *WebApp) DeepCopyObject() runtime.Object {
	out := new(WebApp)
	*out = *in
	out.ObjectMeta = *in.ObjectMeta.DeepCopy()
	return out
}

func (in *WebAppList) DeepCopyObject() runtime.Object {
	out := new(WebAppList)
	out.TypeMeta = in.TypeMeta
	out.ListMeta = in.ListMeta
	if in.Items != nil {
		out.Items = make([]WebApp, len(in.Items))
		for i := range in.Items {
			out.Items[i] = *in.Items[i].DeepCopyObject().(*WebApp)
		}
	}
	return out
}

func (in *WebApp) DeepCopy() *WebApp         { return in.DeepCopyObject().(*WebApp) }
func (in *WebAppList) DeepCopy() *WebAppList { return in.DeepCopyObject().(*WebAppList) }

// AddToScheme 把 WebApp 类型注册进 scheme，controller-runtime 才能识别它。
func AddToScheme(s *runtime.Scheme) error {
	s.AddKnownTypes(GroupVersion, &WebApp{}, &WebAppList{})
	metav1.AddToGroupVersion(s, GroupVersion)
	return nil
}
