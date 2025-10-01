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
	"fmt"

	"github.com/NaverCloudPlatform/ncloud-sdk-go-v2/ncloud"
	"github.com/NaverCloudPlatform/ncloud-sdk-go-v2/services/vloadbalancer"
	"github.com/NaverCloudPlatform/ncloud-sdk-go-v2/services/vserver"
)

// MockClient는 테스트용 모킹 클라이언트입니다.
type MockClient struct {
	// 테스트 시나리오 제어를 위한 플래그들
	ShouldFailCreateLB       bool
	ShouldFailDeleteLB       bool
	ShouldFailCreateTG       bool
	ShouldFailDeleteTG       bool
	ShouldFailCreateListener bool
	ShouldFailAddTarget      bool
	ShouldFailGetServers     bool

	// 반환할 데이터들
	LoadBalancers []vloadbalancer.LoadBalancerInstance
	TargetGroups  []vloadbalancer.TargetGroup
	Servers       []vserver.ServerInstance

	// 호출 추적
	CreateLBCalled       int
	DeleteLBCalled       int
	CreateTGCalled       int
	DeleteTGCalled       int
	CreateListenerCalled int
	AddTargetCalled      int
	GetServersCalled     int
}

// NewMockClient는 새로운 모킹 클라이언트를 생성합니다.
func NewMockClient() *MockClient {
	return &MockClient{
		LoadBalancers: []vloadbalancer.LoadBalancerInstance{},
		TargetGroups:  []vloadbalancer.TargetGroup{},
		Servers:       []vserver.ServerInstance{},
	}
}

func (m *MockClient) CreateLoadBalancerInstance(req *vloadbalancer.CreateLoadBalancerInstanceRequest) (*vloadbalancer.CreateLoadBalancerInstanceResponse, error) {
	m.CreateLBCalled++

	if m.ShouldFailCreateLB {
		return nil, fmt.Errorf("mock error: failed to create load balancer")
	}

	// 새로운 로드밸런서 생성
	lb := &vloadbalancer.LoadBalancerInstance{
		LoadBalancerInstanceNo:         ncloud.String("lb-12345"),
		LoadBalancerName:               req.LoadBalancerName,
		LoadBalancerDescription:        req.LoadBalancerDescription,
		LoadBalancerInstanceStatusName: ncloud.String("CREATING"),
		LoadBalancerInstanceStatus: &vloadbalancer.CommonCode{
			Code:     ncloud.String("INIT"),
			CodeName: ncloud.String("INIT"),
		},
		CreateDate: ncloud.String("2025-09-26T17:00:00+0900"),
	}

	m.LoadBalancers = append(m.LoadBalancers, *lb)

	return &vloadbalancer.CreateLoadBalancerInstanceResponse{
		LoadBalancerInstanceList: []*vloadbalancer.LoadBalancerInstance{lb},
	}, nil
}

func (m *MockClient) GetLoadBalancerInstanceList(req *vloadbalancer.GetLoadBalancerInstanceListRequest) (*vloadbalancer.GetLoadBalancerInstanceListResponse, error) {
	var lbList []*vloadbalancer.LoadBalancerInstance

	for i := range m.LoadBalancers {
		lbList = append(lbList, &m.LoadBalancers[i])
	}

	return &vloadbalancer.GetLoadBalancerInstanceListResponse{
		LoadBalancerInstanceList: lbList,
	}, nil
}

func (m *MockClient) DeleteLoadBalancerInstances(req *vloadbalancer.DeleteLoadBalancerInstancesRequest) (*vloadbalancer.DeleteLoadBalancerInstancesResponse, error) {
	m.DeleteLBCalled++

	if m.ShouldFailDeleteLB {
		return nil, fmt.Errorf("mock error: failed to delete load balancer")
	}

	// 로드밸런서 삭제 (실제로는 상태만 변경)
	for i := range m.LoadBalancers {
		if req.LoadBalancerInstanceNoList != nil && len(req.LoadBalancerInstanceNoList) > 0 {
			if *m.LoadBalancers[i].LoadBalancerInstanceNo == *req.LoadBalancerInstanceNoList[0] {
				m.LoadBalancers[i].LoadBalancerInstanceStatusName = ncloud.String("TERMINATING")
			}
		}
	}

	return &vloadbalancer.DeleteLoadBalancerInstancesResponse{
		LoadBalancerInstanceList: []*vloadbalancer.LoadBalancerInstance{},
	}, nil
}

func (m *MockClient) CreateTargetGroup(req *vloadbalancer.CreateTargetGroupRequest) (*vloadbalancer.CreateTargetGroupResponse, error) {
	m.CreateTGCalled++

	if m.ShouldFailCreateTG {
		return nil, fmt.Errorf("mock error: failed to create target group")
	}

	tg := &vloadbalancer.TargetGroup{
		TargetGroupNo:          ncloud.String("tg-12345"),
		TargetGroupName:        req.TargetGroupName,
		TargetGroupDescription: req.TargetGroupDescription,
		VpcNo:                  req.VpcNo,
		TargetGroupProtocolType: &vloadbalancer.CommonCode{
			Code:     req.TargetGroupProtocolTypeCode,
			CodeName: req.TargetGroupProtocolTypeCode,
		},
		TargetGroupPort: req.TargetGroupPort,
		CreateDate:      ncloud.String("2025-09-26T17:00:00+0900"),
	}

	m.TargetGroups = append(m.TargetGroups, *tg)

	return &vloadbalancer.CreateTargetGroupResponse{
		TargetGroupList: []*vloadbalancer.TargetGroup{tg},
	}, nil
}

func (m *MockClient) DeleteTargetGroups(req *vloadbalancer.DeleteTargetGroupsRequest) (*vloadbalancer.DeleteTargetGroupsResponse, error) {
	m.DeleteTGCalled++

	if m.ShouldFailDeleteTG {
		return nil, fmt.Errorf("1200059: Target group in use")
	}

	// 타겟 그룹 삭제
	if req.TargetGroupNoList != nil {
		for _, tgNo := range req.TargetGroupNoList {
			for i := len(m.TargetGroups) - 1; i >= 0; i-- {
				if *m.TargetGroups[i].TargetGroupNo == *tgNo {
					m.TargetGroups = append(m.TargetGroups[:i], m.TargetGroups[i+1:]...)
					break
				}
			}
		}
	}

	return &vloadbalancer.DeleteTargetGroupsResponse{}, nil
}

func (m *MockClient) GetTargetGroupList(req *vloadbalancer.GetTargetGroupListRequest) (*vloadbalancer.GetTargetGroupListResponse, error) {
	var tgList []*vloadbalancer.TargetGroup

	for i := range m.TargetGroups {
		tgList = append(tgList, &m.TargetGroups[i])
	}

	return &vloadbalancer.GetTargetGroupListResponse{
		TargetGroupList: tgList,
	}, nil
}

func (m *MockClient) CreateLoadBalancerListener(req *vloadbalancer.CreateLoadBalancerListenerRequest) (*vloadbalancer.CreateLoadBalancerListenerResponse, error) {
	m.CreateListenerCalled++

	if m.ShouldFailCreateListener {
		return nil, fmt.Errorf("mock error: failed to create listener")
	}

	listener := &vloadbalancer.LoadBalancerListener{
		LoadBalancerListenerNo: ncloud.String("listener-12345"),
		ProtocolType: &vloadbalancer.CommonCode{
			Code:     req.ProtocolTypeCode,
			CodeName: req.ProtocolTypeCode,
		},
		Port: req.Port,
	}

	return &vloadbalancer.CreateLoadBalancerListenerResponse{
		LoadBalancerListenerList: []*vloadbalancer.LoadBalancerListener{listener},
	}, nil
}

func (m *MockClient) AddTarget(req *vloadbalancer.AddTargetRequest) (*vloadbalancer.AddTargetResponse, error) {
	m.AddTargetCalled++

	if m.ShouldFailAddTarget {
		return nil, fmt.Errorf("mock error: failed to add target")
	}

	return &vloadbalancer.AddTargetResponse{}, nil
}

func (m *MockClient) GetServerInstanceList(req *vserver.GetServerInstanceListRequest) (*vserver.GetServerInstanceListResponse, error) {
	m.GetServersCalled++

	if m.ShouldFailGetServers {
		return nil, fmt.Errorf("mock error: failed to get servers")
	}

	var serverList []*vserver.ServerInstance

	for i := range m.Servers {
		serverList = append(serverList, &m.Servers[i])
	}

	return &vserver.GetServerInstanceListResponse{
		ServerInstanceList: serverList,
	}, nil
}

// 테스트 헬퍼 메서드들
func (m *MockClient) AddMockLoadBalancer(lbID, lbName, status string) {
	lb := vloadbalancer.LoadBalancerInstance{
		LoadBalancerInstanceNo:         ncloud.String(lbID),
		LoadBalancerName:               ncloud.String(lbName),
		LoadBalancerInstanceStatusName: ncloud.String(status),
		LoadBalancerInstanceStatus: &vloadbalancer.CommonCode{
			Code:     ncloud.String("RUN"),
			CodeName: ncloud.String("RUN"),
		},
		CreateDate: ncloud.String("2025-09-26T17:00:00+0900"),
	}
	m.LoadBalancers = append(m.LoadBalancers, lb)
}

func (m *MockClient) AddMockTargetGroup(tgID, tgName string, port int32) {
	tg := vloadbalancer.TargetGroup{
		TargetGroupNo:   ncloud.String(tgID),
		TargetGroupName: ncloud.String(tgName),
		TargetGroupPort: ncloud.Int32(port),
		CreateDate:      ncloud.String("2025-09-26T17:00:00+0900"),
	}
	m.TargetGroups = append(m.TargetGroups, tg)
}

func (m *MockClient) AddMockServer(serverID, serverName, privateIP string) {
	server := vserver.ServerInstance{
		ServerInstanceNo: ncloud.String(serverID),
		ServerName:       ncloud.String(serverName),
		// PrivateIp 필드가 없으므로 기본 구조만 사용
	}
	m.Servers = append(m.Servers, server)
}

func (m *MockClient) Reset() {
	m.ShouldFailCreateLB = false
	m.ShouldFailDeleteLB = false
	m.ShouldFailCreateTG = false
	m.ShouldFailDeleteTG = false
	m.ShouldFailCreateListener = false
	m.ShouldFailAddTarget = false
	m.ShouldFailGetServers = false

	m.LoadBalancers = []vloadbalancer.LoadBalancerInstance{}
	m.TargetGroups = []vloadbalancer.TargetGroup{}
	m.Servers = []vserver.ServerInstance{}

	m.CreateLBCalled = 0
	m.DeleteLBCalled = 0
	m.CreateTGCalled = 0
	m.DeleteTGCalled = 0
	m.CreateListenerCalled = 0
	m.AddTargetCalled = 0
	m.GetServersCalled = 0
}
