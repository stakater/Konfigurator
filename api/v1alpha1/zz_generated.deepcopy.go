// +build !ignore_autogenerated

/*
Copyright 2021.

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

// Code generated by controller-gen. DO NOT EDIT.

package v1alpha1

import (
	runtime "k8s.io/apimachinery/pkg/runtime"
)

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *App) DeepCopyInto(out *App) {
	*out = *in
	if in.VolumeMounts != nil {
		in, out := &in.VolumeMounts, &out.VolumeMounts
		*out = make([]VolumeMount, len(*in))
		copy(*out, *in)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new App.
func (in *App) DeepCopy() *App {
	if in == nil {
		return nil
	}
	out := new(App)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *KonfiguratorTemplate) DeepCopyInto(out *KonfiguratorTemplate) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	out.Status = in.Status
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new KonfiguratorTemplate.
func (in *KonfiguratorTemplate) DeepCopy() *KonfiguratorTemplate {
	if in == nil {
		return nil
	}
	out := new(KonfiguratorTemplate)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *KonfiguratorTemplate) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *KonfiguratorTemplateList) DeepCopyInto(out *KonfiguratorTemplateList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]KonfiguratorTemplate, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new KonfiguratorTemplateList.
func (in *KonfiguratorTemplateList) DeepCopy() *KonfiguratorTemplateList {
	if in == nil {
		return nil
	}
	out := new(KonfiguratorTemplateList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *KonfiguratorTemplateList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *KonfiguratorTemplateSpec) DeepCopyInto(out *KonfiguratorTemplateSpec) {
	*out = *in
	in.App.DeepCopyInto(&out.App)
	if in.Templates != nil {
		in, out := &in.Templates, &out.Templates
		*out = make(map[string]string, len(*in))
		for key, val := range *in {
			(*out)[key] = val
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new KonfiguratorTemplateSpec.
func (in *KonfiguratorTemplateSpec) DeepCopy() *KonfiguratorTemplateSpec {
	if in == nil {
		return nil
	}
	out := new(KonfiguratorTemplateSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *KonfiguratorTemplateStatus) DeepCopyInto(out *KonfiguratorTemplateStatus) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new KonfiguratorTemplateStatus.
func (in *KonfiguratorTemplateStatus) DeepCopy() *KonfiguratorTemplateStatus {
	if in == nil {
		return nil
	}
	out := new(KonfiguratorTemplateStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *VolumeMount) DeepCopyInto(out *VolumeMount) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new VolumeMount.
func (in *VolumeMount) DeepCopy() *VolumeMount {
	if in == nil {
		return nil
	}
	out := new(VolumeMount)
	in.DeepCopyInto(out)
	return out
}
