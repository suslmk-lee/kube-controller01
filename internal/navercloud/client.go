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

package navercloud

import (
	"github.com/NaverCloudPlatform/ncloud-sdk-go-v2/services/vloadbalancer"
	"github.com/NaverCloudPlatform/ncloud-sdk-go-v2/services/vserver"
)

// Client는 네이버 클라우드 API 호출을 위한 인터페이스입니다.
// 이 인터페이스를 통해 실제 API 호출과 테스트용 모킹을 분리할 수 있습니다.
type Client interface {
	// LoadBalancer 관련
	CreateLoadBalancerInstance(req *vloadbalancer.CreateLoadBalancerInstanceRequest) (*vloadbalancer.CreateLoadBalancerInstanceResponse, error)
	GetLoadBalancerInstanceList(req *vloadbalancer.GetLoadBalancerInstanceListRequest) (*vloadbalancer.GetLoadBalancerInstanceListResponse, error)
	DeleteLoadBalancerInstances(req *vloadbalancer.DeleteLoadBalancerInstancesRequest) (*vloadbalancer.DeleteLoadBalancerInstancesResponse, error)

	// Target Group 관련
	CreateTargetGroup(req *vloadbalancer.CreateTargetGroupRequest) (*vloadbalancer.CreateTargetGroupResponse, error)
	DeleteTargetGroups(req *vloadbalancer.DeleteTargetGroupsRequest) (*vloadbalancer.DeleteTargetGroupsResponse, error)
	GetTargetGroupList(req *vloadbalancer.GetTargetGroupListRequest) (*vloadbalancer.GetTargetGroupListResponse, error)

	// Listener 관련
	CreateLoadBalancerListener(req *vloadbalancer.CreateLoadBalancerListenerRequest) (*vloadbalancer.CreateLoadBalancerListenerResponse, error)

	// Target 관련
	AddTarget(req *vloadbalancer.AddTargetRequest) (*vloadbalancer.AddTargetResponse, error)

	// Server 관련
	GetServerInstanceList(req *vserver.GetServerInstanceListRequest) (*vserver.GetServerInstanceListResponse, error)
}

// RealClient는 실제 네이버 클라우드 API를 호출하는 클라이언트입니다.
type RealClient struct {
	VLoadBalancerClient *vloadbalancer.APIClient
	VServerClient       *vserver.APIClient
}

// NewRealClient는 실제 네이버 클라우드 API 클라이언트를 생성합니다.
func NewRealClient(lbClient *vloadbalancer.APIClient, serverClient *vserver.APIClient) Client {
	return &RealClient{
		VLoadBalancerClient: lbClient,
		VServerClient:       serverClient,
	}
}

// CreateLoadBalancerInstance는 로드밸런서 인스턴스를 생성합니다.
func (c *RealClient) CreateLoadBalancerInstance(req *vloadbalancer.CreateLoadBalancerInstanceRequest) (*vloadbalancer.CreateLoadBalancerInstanceResponse, error) {
	return c.VLoadBalancerClient.V2Api.CreateLoadBalancerInstance(req)
}

// GetLoadBalancerInstanceList는 로드밸런서 인스턴스 목록을 조회합니다.
func (c *RealClient) GetLoadBalancerInstanceList(req *vloadbalancer.GetLoadBalancerInstanceListRequest) (*vloadbalancer.GetLoadBalancerInstanceListResponse, error) {
	return c.VLoadBalancerClient.V2Api.GetLoadBalancerInstanceList(req)
}

// DeleteLoadBalancerInstances는 로드밸런서 인스턴스를 삭제합니다.
func (c *RealClient) DeleteLoadBalancerInstances(req *vloadbalancer.DeleteLoadBalancerInstancesRequest) (*vloadbalancer.DeleteLoadBalancerInstancesResponse, error) {
	return c.VLoadBalancerClient.V2Api.DeleteLoadBalancerInstances(req)
}

// CreateTargetGroup은 타겟 그룹을 생성합니다.
func (c *RealClient) CreateTargetGroup(req *vloadbalancer.CreateTargetGroupRequest) (*vloadbalancer.CreateTargetGroupResponse, error) {
	return c.VLoadBalancerClient.V2Api.CreateTargetGroup(req)
}

// DeleteTargetGroups는 타겟 그룹을 삭제합니다.
func (c *RealClient) DeleteTargetGroups(req *vloadbalancer.DeleteTargetGroupsRequest) (*vloadbalancer.DeleteTargetGroupsResponse, error) {
	return c.VLoadBalancerClient.V2Api.DeleteTargetGroups(req)
}

// GetTargetGroupList는 타겟 그룹 목록을 조회합니다.
func (c *RealClient) GetTargetGroupList(req *vloadbalancer.GetTargetGroupListRequest) (*vloadbalancer.GetTargetGroupListResponse, error) {
	return c.VLoadBalancerClient.V2Api.GetTargetGroupList(req)
}

// CreateLoadBalancerListener는 로드밸런서 리스너를 생성합니다.
func (c *RealClient) CreateLoadBalancerListener(req *vloadbalancer.CreateLoadBalancerListenerRequest) (*vloadbalancer.CreateLoadBalancerListenerResponse, error) {
	return c.VLoadBalancerClient.V2Api.CreateLoadBalancerListener(req)
}

// AddTarget은 타겟 그룹에 타겟을 추가합니다.
func (c *RealClient) AddTarget(req *vloadbalancer.AddTargetRequest) (*vloadbalancer.AddTargetResponse, error) {
	return c.VLoadBalancerClient.V2Api.AddTarget(req)
}

// GetServerInstanceList는 서버 인스턴스 목록을 조회합니다.
func (c *RealClient) GetServerInstanceList(req *vserver.GetServerInstanceListRequest) (*vserver.GetServerInstanceListResponse, error) {
	return c.VServerClient.V2Api.GetServerInstanceList(req)
}
