// Copyright (c) 2019 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package controlplane

import (
	"context"
	"encoding/json"

	apisgcp "github.com/gardener/gardener-extension-provider-gcp/pkg/apis/gcp"
	"github.com/gardener/gardener-extension-provider-gcp/pkg/gcp"
	"github.com/gardener/gardener-extension-provider-gcp/pkg/internal"

	extensionscontroller "github.com/gardener/gardener/extensions/pkg/controller"
	"github.com/gardener/gardener/extensions/pkg/controller/controlplane/genericactuator"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	mockclient "github.com/gardener/gardener/pkg/mock/controller-runtime/client"
	"github.com/gardener/gardener/pkg/utils"
	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/runtime/inject"
)

const namespace = "test"

var _ = Describe("ValuesProvider", func() {
	var (
		ctrl *gomock.Controller
		ctx  = context.TODO()

		scheme = runtime.NewScheme()
		_      = apisgcp.AddToScheme(scheme)

		vp genericactuator.ValuesProvider
		c  *mockclient.MockClient

		cp = &extensionsv1alpha1.ControlPlane{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "control-plane",
				Namespace: namespace,
			},
			Spec: extensionsv1alpha1.ControlPlaneSpec{
				SecretRef: corev1.SecretReference{
					Name:      v1beta1constants.SecretNameCloudProvider,
					Namespace: namespace,
				},
				DefaultSpec: extensionsv1alpha1.DefaultSpec{
					ProviderConfig: &runtime.RawExtension{
						Raw: encode(&apisgcp.ControlPlaneConfig{
							Zone: "europe-west1a",
							CloudControllerManager: &apisgcp.CloudControllerManagerConfig{
								FeatureGates: map[string]bool{
									"CustomResourceValidation": true,
								},
							},
						}),
					},
				},
				InfrastructureProviderStatus: &runtime.RawExtension{
					Raw: encode(&apisgcp.InfrastructureStatus{
						Networks: apisgcp.NetworkStatus{
							VPC: apisgcp.VPC{
								Name: "vpc-1234",
							},
							Subnets: []apisgcp.Subnet{
								{
									Name:    "subnet-acbd1234",
									Purpose: apisgcp.PurposeInternal,
								},
							},
						},
					}),
				},
			},
		}

		cidr                  = "10.250.0.0/19"
		clusterK8sLessThan118 = &extensionscontroller.Cluster{
			Shoot: &gardencorev1beta1.Shoot{
				Spec: gardencorev1beta1.ShootSpec{
					Networking: gardencorev1beta1.Networking{
						Pods: &cidr,
					},
					Kubernetes: gardencorev1beta1.Kubernetes{
						Version: "1.15.4",
					},
				},
			},
		}
		clusterK8sAtLeast118 = &extensionscontroller.Cluster{
			Shoot: &gardencorev1beta1.Shoot{
				Spec: gardencorev1beta1.ShootSpec{
					Networking: gardencorev1beta1.Networking{
						Pods: &cidr,
					},
					Kubernetes: gardencorev1beta1.Kubernetes{
						Version: "1.18.0",
						VerticalPodAutoscaler: &gardencorev1beta1.VerticalPodAutoscaler{
							Enabled: true,
						},
					},
				},
			},
		}

		projectID = "abc"
		zone      = "europe-west1a"

		cpSecretKey = client.ObjectKey{Namespace: namespace, Name: v1beta1constants.SecretNameCloudProvider}
		cpSecret    = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      v1beta1constants.SecretNameCloudProvider,
				Namespace: namespace,
			},
			Type: corev1.SecretTypeOpaque,
			Data: map[string][]byte{
				gcp.ServiceAccountJSONField: []byte(`{"project_id":"` + projectID + `"}`),
			},
		}

		checksums = map[string]string{
			v1beta1constants.SecretNameCloudProvider:   "8bafb35ff1ac60275d62e1cbd495aceb511fb354f74a20f7d06ecb48b3a68432",
			internal.CloudProviderConfigName:           "08a7bc7fe8f59b055f173145e211760a83f02cf89635cef26ebb351378635606",
			gcp.CloudControllerManagerName:             "3d791b164a808638da9a8df03924be2a41e34cd664e42231c00fe369e3588272",
			gcp.CloudControllerManagerName + "-server": "6dff2a2e6f14444b66d8e4a351c049f7e89ee24ba3eaab95dbec40ba6bdebb52",
			gcp.CSIProvisionerName:                     "65b1dac6b50673535cff480564c2e5c71077ed19b1b6e0e2291207225bdf77d4",
			gcp.CSIAttacherName:                        "3f22909841cdbb80e5382d689d920309c0a7d995128e52c79773f9608ed7c289",
			gcp.CSISnapshotterName:                     "6a5bfc847638c499062f7fb44e31a30a9760bf4179e1dbf85e0ff4b4f162cd68",
			gcp.CSIResizerName:                         "a77e663ba1af340fb3dd7f6f8a1be47c7aa9e658198695480641e6b934c0b9ed",
			gcp.CSISnapshotControllerName:              "84cba346d2e2cf96c3811b55b01f57bdd9b9bcaed7065760470942d267984eaf",
		}

		enabledTrue  = map[string]interface{}{"enabled": true}
		enabledFalse = map[string]interface{}{"enabled": false}

		logger = log.Log.WithName("test")
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())

		c = mockclient.NewMockClient(ctrl)

		vp = NewValuesProvider(logger)
		err := vp.(inject.Scheme).InjectScheme(scheme)
		Expect(err).NotTo(HaveOccurred())
		err = vp.(inject.Client).InjectClient(c)
		Expect(err).NotTo(HaveOccurred())

	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Describe("#GetConfigChartValues", func() {
		It("should return correct config chart values", func() {
			c.EXPECT().Get(context.TODO(), cpSecretKey, &corev1.Secret{}).DoAndReturn(clientGet(cpSecret))

			values, err := vp.GetConfigChartValues(ctx, cp, clusterK8sLessThan118)
			Expect(err).NotTo(HaveOccurred())
			Expect(values).To(Equal(map[string]interface{}{
				"projectID":      projectID,
				"networkName":    "vpc-1234",
				"subNetworkName": "subnet-acbd1234",
				"zone":           zone,
				"nodeTags":       namespace,
			}))
		})
	})

	Describe("#GetControlPlaneChartValues", func() {
		ccmChartValues := utils.MergeMaps(enabledTrue, map[string]interface{}{
			"replicas":    1,
			"clusterName": namespace,
			"podNetwork":  cidr,
			"podAnnotations": map[string]interface{}{
				"checksum/secret-" + gcp.CloudControllerManagerName:             "3d791b164a808638da9a8df03924be2a41e34cd664e42231c00fe369e3588272",
				"checksum/secret-" + gcp.CloudControllerManagerName + "-server": "6dff2a2e6f14444b66d8e4a351c049f7e89ee24ba3eaab95dbec40ba6bdebb52",
				"checksum/secret-" + v1beta1constants.SecretNameCloudProvider:   "8bafb35ff1ac60275d62e1cbd495aceb511fb354f74a20f7d06ecb48b3a68432",
				"checksum/configmap-" + internal.CloudProviderConfigName:        "08a7bc7fe8f59b055f173145e211760a83f02cf89635cef26ebb351378635606",
			},
			"podLabels": map[string]interface{}{
				"maintenance.gardener.cloud/restart": "true",
			},
			"featureGates": map[string]bool{
				"CustomResourceValidation": true,
			},
			"tlsCipherSuites": []string{
				"TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256",
				"TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384",
				"TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305",
				"TLS_RSA_WITH_AES_128_CBC_SHA",
				"TLS_RSA_WITH_AES_256_CBC_SHA",
				"TLS_ECDHE_RSA_WITH_AES_256_CBC_SHA",
			},
		})

		BeforeEach(func() {
			c.EXPECT().Get(context.TODO(), cpSecretKey, &corev1.Secret{}).DoAndReturn(clientGet(cpSecret))
		})

		It("should return correct control plane chart values (k8s < 1.18)", func() {
			values, err := vp.GetControlPlaneChartValues(ctx, cp, clusterK8sLessThan118, checksums, false)
			Expect(err).NotTo(HaveOccurred())
			Expect(values).To(Equal(map[string]interface{}{
				gcp.CloudControllerManagerName: utils.MergeMaps(ccmChartValues, map[string]interface{}{
					"kubernetesVersion": clusterK8sLessThan118.Shoot.Spec.Kubernetes.Version,
				}),
				gcp.CSIControllerName: enabledFalse,
			}))
		})

		It("should return correct control plane chart values (k8s >= 1.18)", func() {
			values, err := vp.GetControlPlaneChartValues(ctx, cp, clusterK8sAtLeast118, checksums, false)
			Expect(err).NotTo(HaveOccurred())
			Expect(values).To(Equal(map[string]interface{}{
				gcp.CloudControllerManagerName: utils.MergeMaps(ccmChartValues, map[string]interface{}{
					"kubernetesVersion": clusterK8sAtLeast118.Shoot.Spec.Kubernetes.Version,
				}),
				gcp.CSIControllerName: utils.MergeMaps(enabledTrue, map[string]interface{}{
					"replicas":  1,
					"projectID": projectID,
					"zone":      zone,
					"podAnnotations": map[string]interface{}{
						"checksum/secret-" + gcp.CSIProvisionerName:                   checksums[gcp.CSIProvisionerName],
						"checksum/secret-" + gcp.CSIAttacherName:                      checksums[gcp.CSIAttacherName],
						"checksum/secret-" + gcp.CSISnapshotterName:                   checksums[gcp.CSISnapshotterName],
						"checksum/secret-" + gcp.CSIResizerName:                       checksums[gcp.CSIResizerName],
						"checksum/secret-" + v1beta1constants.SecretNameCloudProvider: checksums[v1beta1constants.SecretNameCloudProvider],
					},
					"csiSnapshotController": map[string]interface{}{
						"replicas": 1,
						"podAnnotations": map[string]interface{}{
							"checksum/secret-" + gcp.CSISnapshotControllerName: checksums[gcp.CSISnapshotControllerName],
						},
					},
				}),
			}))
		})
	})

	Describe("#GetControlPlaneShootChartValues", func() {
		It("should return correct shoot control plane chart values (k8s < 1.18)", func() {
			values, err := vp.GetControlPlaneShootChartValues(ctx, cp, clusterK8sLessThan118, nil)
			Expect(err).NotTo(HaveOccurred())
			Expect(values).To(Equal(map[string]interface{}{
				gcp.CloudControllerManagerName: enabledTrue,
				gcp.CSINodeName: utils.MergeMaps(enabledFalse, map[string]interface{}{
					"kubernetesVersion": "1.15.4",
					"vpaEnabled":        false,
				}),
			}))
		})

		It("should return correct shoot control plane chart values (k8s >= 1.18)", func() {
			values, err := vp.GetControlPlaneShootChartValues(ctx, cp, clusterK8sAtLeast118, nil)
			Expect(err).NotTo(HaveOccurred())
			Expect(values).To(Equal(map[string]interface{}{
				gcp.CloudControllerManagerName: enabledTrue,
				gcp.CSINodeName: utils.MergeMaps(enabledTrue, map[string]interface{}{
					"kubernetesVersion": "1.18.0",
					"vpaEnabled":        true,
				}),
			}))
		})
	})

	Describe("#GetControlPlaneShootCRDsChartValues", func() {
		It("should return correct control plane shoot CRDs chart values (k8s < 1.18)", func() {
			values, err := vp.GetControlPlaneShootCRDsChartValues(ctx, cp, clusterK8sLessThan118)
			Expect(err).NotTo(HaveOccurred())
			Expect(values).To(Equal(map[string]interface{}{"volumesnapshots": map[string]interface{}{"enabled": false}}))
		})

		It("should return correct control plane shoot CRDs chart values (k8s >= 1.18)", func() {
			values, err := vp.GetControlPlaneShootCRDsChartValues(ctx, cp, clusterK8sAtLeast118)
			Expect(err).NotTo(HaveOccurred())
			Expect(values).To(Equal(map[string]interface{}{"volumesnapshots": map[string]interface{}{"enabled": true}}))
		})
	})

	Describe("#GetStorageClassesChartValues", func() {
		It("should return correct storage class chart values (k8s < 1.18)", func() {
			values, err := vp.GetStorageClassesChartValues(ctx, cp, clusterK8sLessThan118)
			Expect(err).NotTo(HaveOccurred())
			Expect(values).To(Equal(map[string]interface{}{"useLegacyProvisioner": true}))
		})

		It("should return correct storage class chart values (k8s >= 1.18)", func() {
			values, err := vp.GetStorageClassesChartValues(ctx, cp, clusterK8sAtLeast118)
			Expect(err).NotTo(HaveOccurred())
			Expect(values).To(Equal(map[string]interface{}{"useLegacyProvisioner": false}))
		})
	})
})

func encode(obj runtime.Object) []byte {
	data, _ := json.Marshal(obj)
	return data
}

func clientGet(result runtime.Object) interface{} {
	return func(ctx context.Context, key client.ObjectKey, obj runtime.Object) error {
		switch obj.(type) {
		case *corev1.Secret:
			*obj.(*corev1.Secret) = *result.(*corev1.Secret)
		}
		return nil
	}
}
