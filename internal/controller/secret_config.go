/*
Copyright 2025.

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
	"time"
)

// SecretConfig holds the configuration for secret management
type SecretConfig struct {
	Name       string           // Secret name for Kubernetes secret mode
	Management SecretManagement // Secret management configuration
}

// SecretManagement defines how secrets are managed
type SecretManagement struct {
	Mode    string        // "auto", "openbao", "eso", "kubernetes"
	OpenBao OpenBaoConfig // OpenBao (Vault) configuration
	ESO     ESOConfig     // External Secrets Operator configuration
}

// OpenBaoConfig holds OpenBao/Vault configuration for AppRole authentication
type OpenBaoConfig struct {
	Address       string // OpenBao server address
	Path          string // Secret path in OpenBao
	Role          string // AppRole role name
	Namespace     string // Kubernetes namespace for AppRole secret
	AppRoleSecret string // Secret name containing VAULT_ROLE_ID and VAULT_SECRET_ID
}

// ESOConfig holds External Secrets Operator configuration
type ESOConfig struct {
	ExternalSecretName string        // Name of the ExternalSecret resource
	Timeout            time.Duration // Timeout for waiting ESO to sync secrets
}

// DefaultSecretConfig returns the default secret configuration for Naver Cloud
func DefaultSecretConfig() SecretConfig {
	return SecretConfig{
		Name: "naver-cloud-credentials",
		Management: SecretManagement{
			Mode: "auto", // Default to auto-detection: OpenBao -> ESO -> Kubernetes
			OpenBao: OpenBaoConfig{
				Address:       "http://controller-vault.k-paas-system.svc.cluster.local:8200",
				Path:          "secret/data/csp/naver-cloud",
				Role:          "naver-controller",
				Namespace:     "k-paas-system",
				AppRoleSecret: "controller-manager",
			},
			ESO: ESOConfig{
				ExternalSecretName: "naver-cloud-credentials-external",
				Timeout:            5 * time.Minute,
			},
		},
	}
}

// SecretModeAuto is the auto-detection mode
const SecretModeAuto = "auto"

// SecretModeOpenBao is the OpenBao/Vault mode
const SecretModeOpenBao = "openbao"

// SecretModeESO is the External Secrets Operator mode
const SecretModeESO = "eso"

// SecretModeKubernetes is the native Kubernetes secret mode
const SecretModeKubernetes = "kubernetes"

// GetSecretMode returns the current secret management mode
func (c *SecretConfig) GetSecretMode() string {
	if c.Management.Mode == "" {
		return SecretModeAuto
	}
	return c.Management.Mode
}

// IsOpenBaoEnabled checks if OpenBao configuration is valid
func (c *SecretConfig) IsOpenBaoEnabled() bool {
	return c.Management.OpenBao.Address != "" &&
		c.Management.OpenBao.Path != "" &&
		c.Management.OpenBao.Role != ""
}

// IsESOEnabled checks if ESO configuration is valid
func (c *SecretConfig) IsESOEnabled() bool {
	return c.Management.ESO.ExternalSecretName != ""
}
