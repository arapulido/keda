/*
Copyright 2022 The KEDA Authors

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

package resolver

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	kedav1alpha1 "github.com/kedacore/keda/v2/apis/keda/v1alpha1"
)

type AzureKeyVaultHandlerTestCase struct {
	Name             string
	Config           *kedav1alpha1.AzureKeyVault
	TriggerNamespace string
	ExpectedError    string
}

func TestAzureKeyVaultHandlerInitialize(t *testing.T) {
	testCases := []AzureKeyVaultHandlerTestCase{
		{
			Name: "Invalid Pod identity provider",
			Config: &kedav1alpha1.AzureKeyVault{
				PodIdentity: &kedav1alpha1.AuthPodIdentity{
					Provider: "xyz",
				},
			},
			TriggerNamespace: "testNamespace",
			ExpectedError:    "key vault does not support pod identity provider - xyz",
		},
		{
			Name: "Missing credentials and pod identity provider",
			Config: &kedav1alpha1.AzureKeyVault{
				Credentials: nil,
				PodIdentity: &kedav1alpha1.AuthPodIdentity{
					Provider: "",
				},
			},
			TriggerNamespace: "testNamespace",
			ExpectedError:    "clientID, tenantID and clientSecret are expected when not using a pod identity provider",
		},
		{
			Name: "Empty trigger namespace",
			Config: &kedav1alpha1.AzureKeyVault{
				Credentials: &kedav1alpha1.AzureKeyVaultCredentials{
					ClientSecret: &kedav1alpha1.AzureKeyVaultClientSecret{
						ValueFrom: kedav1alpha1.ValueFromSecret{
							SecretKeyRef: kedav1alpha1.SecretKeyRef{
								Name: "testSecretName",
								Key:  "testSecretKey",
							},
						},
					},
				},
				PodIdentity: &kedav1alpha1.AuthPodIdentity{
					Provider: kedav1alpha1.PodIdentityProviderNone,
				},
			},
			TriggerNamespace: "",
			ExpectedError:    "clientID, tenantID and clientSecret are expected when not using a pod identity provider",
		},
		{
			Name: "Empty credentials secret name",
			Config: &kedav1alpha1.AzureKeyVault{
				Credentials: &kedav1alpha1.AzureKeyVaultCredentials{
					ClientSecret: &kedav1alpha1.AzureKeyVaultClientSecret{
						ValueFrom: kedav1alpha1.ValueFromSecret{
							SecretKeyRef: kedav1alpha1.SecretKeyRef{
								Name: "",
								Key:  "testSecretKey",
							},
						},
					},
				},
				PodIdentity: &kedav1alpha1.AuthPodIdentity{
					Provider: kedav1alpha1.PodIdentityProviderNone,
				},
			},
			TriggerNamespace: "testNamespace",
			ExpectedError:    "clientID, tenantID and clientSecret are expected when not using a pod identity provider",
		},
		{
			Name: "Empty credentials secret key",
			Config: &kedav1alpha1.AzureKeyVault{
				Credentials: &kedav1alpha1.AzureKeyVaultCredentials{
					ClientSecret: &kedav1alpha1.AzureKeyVaultClientSecret{
						ValueFrom: kedav1alpha1.ValueFromSecret{
							SecretKeyRef: kedav1alpha1.SecretKeyRef{
								Name: "testSecretName",
								Key:  "",
							},
						},
					},
				},
				PodIdentity: &kedav1alpha1.AuthPodIdentity{
					Provider: kedav1alpha1.PodIdentityProviderNone,
				},
			},
			TriggerNamespace: "testNamespace",
			ExpectedError:    "clientID, tenantID and clientSecret are expected when not using a pod identity provider",
		},
	}

	for _, testCase := range testCases {
		fake.NewClientBuilder()
		t.Run(testCase.Name, func(t *testing.T) {
			handler := NewAzureKeyVaultHandler(testCase.Config)
			err := handler.Initialize(context.TODO(), nil, logf.Log.WithName("test"), "", nil)
			if testCase.ExpectedError != "" {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), testCase.ExpectedError)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
