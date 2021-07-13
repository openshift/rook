/*
Copyright 2018 The Kubernetes Authors.

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

package bucket

import (
	"context"
	"crypto/rand"

	"github.com/coreos/pkg/capnslog"
	bktv1alpha1 "github.com/kube-object-storage/lib-bucket-provisioner/pkg/apis/objectbucket.io/v1alpha1"
	"github.com/kube-object-storage/lib-bucket-provisioner/pkg/provisioner"
	apibkt "github.com/kube-object-storage/lib-bucket-provisioner/pkg/provisioner/api"
	"github.com/pkg/errors"
	v1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	cephv1 "github.com/rook/rook/pkg/apis/ceph.rook.io/v1"
	cephObject "github.com/rook/rook/pkg/operator/ceph/object"
)

var logger = capnslog.NewPackageLogger("github.com/rook/rook", "op-bucket-prov")

const (
	genUserLen           = 8
	cephUser             = "cephUser"
	objectStoreName      = "objectStoreName"
	objectStoreNamespace = "objectStoreNamespace"
	objectStoreEndpoint  = "endpoint"
)

func NewBucketController(cfg *rest.Config, p *Provisioner) (*provisioner.Provisioner, error) {
	const allNamespaces = ""
	provName := cephObject.GetObjectBucketProvisioner(p.context, p.clusterInfo.Namespace)

	logger.Infof("ceph bucket provisioner launched watching for provisioner %q", provName)
	return provisioner.NewProvisioner(cfg, provName, p, allNamespaces)
}

func getObjectStoreName(sc *storagev1.StorageClass) string {
	return sc.Parameters[objectStoreName]
}

func getObjectStoreNameSpace(sc *storagev1.StorageClass) string {
	return sc.Parameters[objectStoreNamespace]
}

func getObjectStoreEndpoint(sc *storagev1.StorageClass) string {
	return sc.Parameters[objectStoreEndpoint]
}

func getBucketName(ob *bktv1alpha1.ObjectBucket) string {
	return ob.Spec.Endpoint.BucketName
}

func isStaticBucket(sc *storagev1.StorageClass) (string, bool) {
	const key = "bucketName"
	val, ok := sc.Parameters[key]
	return val, ok
}

func getCephUser(ob *bktv1alpha1.ObjectBucket) string {
	return ob.Spec.AdditionalState[cephUser]
}

func (p *Provisioner) getObjectStore() (*cephv1.CephObjectStore, error) {
	ctx := context.TODO()
	// Verify the object store API object actually exists
	store, err := p.context.RookClientset.CephV1().CephObjectStores(p.clusterInfo.Namespace).Get(ctx, p.objectStoreName, metav1.GetOptions{})
	if err != nil {
		if kerrors.IsNotFound(err) {
			return nil, errors.Wrap(err, "cephObjectStore not found")
		}
		return nil, errors.Wrapf(err, "failed to get ceph object store %q", p.objectStoreName)
	}
	return store, err
}

func getService(c kubernetes.Interface, namespace, name string) (*v1.Service, error) {
	ctx := context.TODO()
	// Verify the object store's service actually exists
	svc, err := c.CoreV1().Services(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		if kerrors.IsNotFound(err) {
			return nil, errors.Wrap(err, "cephObjectStore service not found")
		}
		return nil, errors.Wrapf(err, "failed to get ceph object store service %q", name)
	}
	return svc, nil
}

func (p *Provisioner) getCephCluster() (*cephv1.CephCluster, error) {
	cephCluster, err := p.context.RookClientset.CephV1().CephClusters(p.clusterInfo.Namespace).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return nil, errors.Wrapf(err, "failed to list ceph clusters in namespace %q", p.clusterInfo.Namespace)
	}
	if len(cephCluster.Items) == 0 {
		return nil, errors.Errorf("failed to find ceph cluster in namespace %q", p.clusterInfo.Namespace)
	}

	// This is a bit weak, but there will always be a single cluster per namespace anyway
	return &cephCluster.Items[0], err
}

func randomString(n int) string {

	var letterRunes = "0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return ""
	}
	for k, v := range b {
		b[k] = letterRunes[v%byte(len(letterRunes))]
	}
	return string(b)
}

func MaxObjectQuota(options *apibkt.BucketOptions) string {
	return options.ObjectBucketClaim.Spec.AdditionalConfig["maxObjects"]
}

func MaxSizeQuota(options *apibkt.BucketOptions) string {
	return options.ObjectBucketClaim.Spec.AdditionalConfig["maxSize"]
}
