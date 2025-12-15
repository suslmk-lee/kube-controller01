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
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// NaverCloudCredentials holds the Naver Cloud API credentials
type NaverCloudCredentials struct {
	APIKey    string
	APISecret string
	Region    string
	VpcNo     string
	SubnetNo  string
}

// SecretProvider interface for retrieving secrets from different backends
type SecretProvider interface {
	GetCredentials(ctx context.Context) (*NaverCloudCredentials, error)
}

// OpenBaoProvider retrieves secrets from OpenBao using AppRole authentication
type OpenBaoProvider struct {
	Client      client.Client
	Config      OpenBaoConfig
	httpClient  *http.Client
	cachedToken string
	tokenExpiry time.Time
	tokenMutex  sync.RWMutex
}

// NewOpenBaoProvider creates a new OpenBao provider
func NewOpenBaoProvider(c client.Client, config OpenBaoConfig) *OpenBaoProvider {
	return &OpenBaoProvider{
		Client: c,
		Config: config,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// GetCredentials retrieves Naver Cloud credentials from OpenBao
func (p *OpenBaoProvider) GetCredentials(ctx context.Context) (*NaverCloudCredentials, error) {
	logger := log.FromContext(ctx)

	// Step 1: Get AppRole credentials from Kubernetes Secret
	roleID, secretID, err := p.getAppRoleCredentials(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get AppRole credentials: %w", err)
	}

	// Step 2: Authenticate with OpenBao using AppRole
	token, err := p.authenticateAppRole(ctx, roleID, secretID)
	if err != nil {
		return nil, fmt.Errorf("failed to authenticate with OpenBao: %w", err)
	}

	// Step 3: Read secrets from OpenBao
	credentials, err := p.readSecret(ctx, token)
	if err != nil {
		return nil, fmt.Errorf("failed to read secret from OpenBao: %w", err)
	}

	logger.Info("Successfully retrieved credentials from OpenBao",
		"path", p.Config.Path)

	return credentials, nil
}

// getAppRoleCredentials retrieves VAULT_ROLE_ID and VAULT_SECRET_ID from Kubernetes Secret
func (p *OpenBaoProvider) getAppRoleCredentials(ctx context.Context) (string, string, error) {
	secret := &corev1.Secret{}
	secretKey := types.NamespacedName{
		Namespace: p.Config.Namespace,
		Name:      p.Config.AppRoleSecret,
	}

	if err := p.Client.Get(ctx, secretKey, secret); err != nil {
		return "", "", fmt.Errorf("failed to get AppRole secret %s/%s: %w",
			p.Config.Namespace, p.Config.AppRoleSecret, err)
	}

	roleID := string(secret.Data["VAULT_ROLE_ID"])
	secretID := string(secret.Data["VAULT_SECRET_ID"])

	if roleID == "" {
		return "", "", fmt.Errorf("VAULT_ROLE_ID not found in secret %s/%s",
			p.Config.Namespace, p.Config.AppRoleSecret)
	}
	if secretID == "" {
		return "", "", fmt.Errorf("VAULT_SECRET_ID not found in secret %s/%s",
			p.Config.Namespace, p.Config.AppRoleSecret)
	}

	return roleID, secretID, nil
}

// authenticateAppRole authenticates with OpenBao using AppRole and returns a token
func (p *OpenBaoProvider) authenticateAppRole(ctx context.Context, roleID, secretID string) (string, error) {
	// Check if we have a valid cached token (with read lock)
	p.tokenMutex.RLock()
	if p.cachedToken != "" && time.Now().Before(p.tokenExpiry) {
		token := p.cachedToken
		p.tokenMutex.RUnlock()
		return token, nil
	}
	p.tokenMutex.RUnlock()

	loginURL := fmt.Sprintf("%s/v1/auth/approle/login", p.Config.Address)

	payload := map[string]string{
		"role_id":   roleID,
		"secret_id": secretID,
	}
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("failed to marshal login payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, loginURL, bytes.NewReader(payloadBytes))
	if err != nil {
		return "", fmt.Errorf("failed to create login request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to send login request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("OpenBao login failed with status %d: %s", resp.StatusCode, string(body))
	}

	var loginResp struct {
		Auth struct {
			ClientToken   string `json:"client_token"`
			LeaseDuration int    `json:"lease_duration"`
		} `json:"auth"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&loginResp); err != nil {
		return "", fmt.Errorf("failed to decode login response: %w", err)
	}

	// Cache the token with expiry (use 80% of lease duration for safety margin)
	p.tokenMutex.Lock()
	p.cachedToken = loginResp.Auth.ClientToken
	p.tokenExpiry = time.Now().Add(time.Duration(loginResp.Auth.LeaseDuration*80/100) * time.Second)
	p.tokenMutex.Unlock()

	return loginResp.Auth.ClientToken, nil
}

// readSecret reads the Naver Cloud credentials from OpenBao
func (p *OpenBaoProvider) readSecret(ctx context.Context, token string) (*NaverCloudCredentials, error) {
	secretURL := fmt.Sprintf("%s/v1/%s", p.Config.Address, p.Config.Path)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, secretURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create secret request: %w", err)
	}
	req.Header.Set("X-Vault-Token", token)

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send secret request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("OpenBao read secret failed with status %d: %s", resp.StatusCode, string(body))
	}

	var secretResp struct {
		Data struct {
			Data map[string]interface{} `json:"data"`
		} `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&secretResp); err != nil {
		return nil, fmt.Errorf("failed to decode secret response: %w", err)
	}

	data := secretResp.Data.Data
	credentials := &NaverCloudCredentials{
		APIKey:    getStringValue(data, "NAVER_CLOUD_API_KEY"),
		APISecret: getStringValue(data, "NAVER_CLOUD_API_SECRET"),
		Region:    getStringValue(data, "NAVER_CLOUD_REGION"),
		VpcNo:     getStringValue(data, "NAVER_CLOUD_VPC_NO"),
		SubnetNo:  getStringValue(data, "NAVER_CLOUD_SUBNET_NO"),
	}

	// Set default region if not specified
	if credentials.Region == "" {
		credentials.Region = "KR"
	}

	// Validate required credentials
	if credentials.APIKey == "" {
		return nil, fmt.Errorf("NAVER_CLOUD_API_KEY not found in OpenBao secret")
	}
	if credentials.APISecret == "" {
		return nil, fmt.Errorf("NAVER_CLOUD_API_SECRET not found in OpenBao secret")
	}

	return credentials, nil
}

// KubernetesSecretProvider retrieves secrets from Kubernetes Secret
type KubernetesSecretProvider struct {
	Client     client.Client
	SecretName string
	Namespace  string
}

// NewKubernetesSecretProvider creates a new Kubernetes Secret provider
func NewKubernetesSecretProvider(c client.Client, secretName, namespace string) *KubernetesSecretProvider {
	return &KubernetesSecretProvider{
		Client:     c,
		SecretName: secretName,
		Namespace:  namespace,
	}
}

// GetCredentials retrieves Naver Cloud credentials from Kubernetes Secret
func (p *KubernetesSecretProvider) GetCredentials(ctx context.Context) (*NaverCloudCredentials, error) {
	logger := log.FromContext(ctx)

	secret := &corev1.Secret{}
	secretKey := types.NamespacedName{
		Namespace: p.Namespace,
		Name:      p.SecretName,
	}

	if err := p.Client.Get(ctx, secretKey, secret); err != nil {
		return nil, fmt.Errorf("failed to get secret %s/%s: %w", p.Namespace, p.SecretName, err)
	}

	credentials := &NaverCloudCredentials{
		APIKey:    string(secret.Data["NAVER_CLOUD_API_KEY"]),
		APISecret: string(secret.Data["NAVER_CLOUD_API_SECRET"]),
		Region:    string(secret.Data["NAVER_CLOUD_REGION"]),
		VpcNo:     string(secret.Data["NAVER_CLOUD_VPC_NO"]),
		SubnetNo:  string(secret.Data["NAVER_CLOUD_SUBNET_NO"]),
	}

	// Set default region if not specified
	if credentials.Region == "" {
		credentials.Region = "KR"
	}

	// Validate required credentials
	if credentials.APIKey == "" {
		return nil, fmt.Errorf("NAVER_CLOUD_API_KEY not found in secret %s/%s", p.Namespace, p.SecretName)
	}
	if credentials.APISecret == "" {
		return nil, fmt.Errorf("NAVER_CLOUD_API_SECRET not found in secret %s/%s", p.Namespace, p.SecretName)
	}

	logger.Info("Successfully retrieved credentials from Kubernetes Secret",
		"secret", p.SecretName)

	return credentials, nil
}

// AutoSecretProvider automatically detects and uses the appropriate secret provider
type AutoSecretProvider struct {
	Client       client.Client
	SecretConfig SecretConfig
	Namespace    string
	provider     SecretProvider
}

// NewAutoSecretProvider creates a new auto-detecting secret provider
func NewAutoSecretProvider(c client.Client, config SecretConfig, namespace string) *AutoSecretProvider {
	return &AutoSecretProvider{
		Client:       c,
		SecretConfig: config,
		Namespace:    namespace,
	}
}

// GetCredentials retrieves credentials using auto-detection: OpenBao -> ESO -> Kubernetes
func (p *AutoSecretProvider) GetCredentials(ctx context.Context) (*NaverCloudCredentials, error) {
	logger := log.FromContext(ctx)

	mode := p.SecretConfig.GetSecretMode()

	switch mode {
	case SecretModeOpenBao:
		return p.getFromOpenBao(ctx)
	case SecretModeESO:
		// ESO creates a Kubernetes Secret, so we read from it
		logger.Info("Using External Secrets Operator for secret management")
		return p.getFromKubernetesSecret(ctx)
	case SecretModeKubernetes:
		return p.getFromKubernetesSecret(ctx)
	case SecretModeAuto:
		// Try OpenBao first
		if p.SecretConfig.IsOpenBaoEnabled() {
			creds, err := p.getFromOpenBao(ctx)
			if err == nil {
				logger.Info("Using OpenBao for secret management")
				return creds, nil
			}
			logger.Info("OpenBao not available, falling back", "error", err.Error())
		}

		// Fall back to Kubernetes Secret
		logger.Info("Using Kubernetes Secret for secret management")
		return p.getFromKubernetesSecret(ctx)
	default:
		return nil, fmt.Errorf("unknown secret mode: %s", mode)
	}
}

func (p *AutoSecretProvider) getFromOpenBao(ctx context.Context) (*NaverCloudCredentials, error) {
	logger := log.FromContext(ctx)
	provider := NewOpenBaoProvider(p.Client, p.SecretConfig.Management.OpenBao)
	creds, err := provider.GetCredentials(ctx)
	if err != nil {
		return nil, err
	}

	// OpenBao에서 VpcNo, SubnetNo, Region이 없으면 ConfigMap에서 보완
	if creds.VpcNo == "" || creds.SubnetNo == "" || creds.Region == "" {
		logger.Info("OpenBao credentials에 VpcNo/SubnetNo/Region 누락, ConfigMap에서 보완 시도")
		p.supplementFromConfigMap(ctx, creds)
	}

	return creds, nil
}

// supplementFromConfigMap은 ConfigMap에서 VpcNo, SubnetNo, Region을 보완합니다
func (p *AutoSecretProvider) supplementFromConfigMap(ctx context.Context, creds *NaverCloudCredentials) {
	logger := log.FromContext(ctx)

	// ConfigMap 이름: naver-cloud-config
	configMapName := "naver-cloud-config"

	var configMap corev1.ConfigMap
	err := p.Client.Get(ctx, client.ObjectKey{
		Namespace: p.Namespace,
		Name:      configMapName,
	}, &configMap)

	if err != nil {
		logger.Info("ConfigMap 조회 실패, 기본값 사용", "configMap", configMapName, "error", err.Error())
		// 기본값 설정
		if creds.Region == "" {
			creds.Region = "KR"
		}
		return
	}

	// ConfigMap에서 값 보완
	if creds.VpcNo == "" {
		if vpcNo, ok := configMap.Data["NAVER_CLOUD_VPC_NO"]; ok {
			creds.VpcNo = vpcNo
			logger.Info("ConfigMap", "vpcNo", vpcNo)
		}
	}

	if creds.SubnetNo == "" {
		if subnetNo, ok := configMap.Data["NAVER_CLOUD_SUBNET_NO"]; ok {
			creds.SubnetNo = subnetNo
			logger.Info("ConfigMap", "subnetNo", subnetNo)
		}
	}

	if creds.Region == "" {
		if region, ok := configMap.Data["NAVER_CLOUD_REGION"]; ok {
			creds.Region = region
			logger.Info("ConfigMap", "region", region)
		} else {
			creds.Region = "KR" // 기본값
		}
	}
}

func (p *AutoSecretProvider) getFromKubernetesSecret(ctx context.Context) (*NaverCloudCredentials, error) {
	provider := NewKubernetesSecretProvider(p.Client, p.SecretConfig.Name, p.Namespace)
	return provider.GetCredentials(ctx)
}

// Helper function to safely get string value from map
func getStringValue(data map[string]interface{}, key string) string {
	if val, ok := data[key]; ok {
		if strVal, ok := val.(string); ok {
			return strVal
		}
	}
	return ""
}
