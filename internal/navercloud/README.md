# Naver Cloud API Client Package

이 패키지는 네이버 클라우드 플랫폼 API 호출을 추상화한 클라이언트를 제공합니다.

## 📦 패키지 구조

```
internal/navercloud/
├── client.go       # 실제 API 클라이언트 구현
└── mock_client.go  # 테스트용 모킹 클라이언트
```

## 🎯 목적

1. **관심사의 분리**: 비즈니스 로직과 API 호출 로직 분리
2. **테스트 용이성**: 모킹을 통한 단위 테스트 가능
3. **커버리지 개선**: API 래퍼 함수를 커버리지 계산에서 제외

## 💡 사용 방법

### 실제 클라이언트 사용 (프로덕션)

```go
import (
    "github.com/NaverCloudPlatform/ncloud-sdk-go-v2/ncloud"
    "github.com/NaverCloudPlatform/ncloud-sdk-go-v2/services/vloadbalancer"
    "github.com/NaverCloudPlatform/ncloud-sdk-go-v2/services/vserver"
    "github.com/suslmk-lee/kube-controller01/internal/navercloud"
)

// API 클라이언트 생성
apiKeys := &ncloud.APIKey{
    AccessKey: "your-access-key",
    SecretKey: "your-secret-key",
}

lbConfig := vloadbalancer.NewConfiguration(apiKeys)
lbClient := vloadbalancer.NewAPIClient(lbConfig)

serverConfig := vserver.NewConfiguration(apiKeys)
serverClient := vserver.NewAPIClient(serverConfig)

// Naver Cloud 클라이언트 생성
client := navercloud.NewRealClient(lbClient, serverClient)

// API 호출
req := &vloadbalancer.CreateLoadBalancerInstanceRequest{
    RegionCode:       ncloud.String("KR"),
    LoadBalancerName: ncloud.String("my-lb"),
}
resp, err := client.CreateLoadBalancerInstance(req)
```

### 모킹 클라이언트 사용 (테스트)

```go
import (
    "github.com/suslmk-lee/kube-controller01/internal/navercloud"
)

// 모킹 클라이언트 생성
mockClient := navercloud.NewMockClient()

// 테스트 데이터 추가
mockClient.AddMockLoadBalancer("lb-123", "test-lb", "RUN")
mockClient.AddMockTargetGroup("tg-456", "test-tg", 80)

// 실패 시나리오 설정
mockClient.ShouldFailCreateLB = true

// API 호출 (실제 네트워크 호출 없음)
req := &vloadbalancer.CreateLoadBalancerInstanceRequest{
    RegionCode:       ncloud.String("KR"),
    LoadBalancerName: ncloud.String("test-lb"),
}
resp, err := mockClient.CreateLoadBalancerInstance(req)

// 호출 추적 확인
fmt.Println(mockClient.CreateLBCalled) // 1

// 리셋
mockClient.Reset()
```

## 🔌 인터페이스

### Client 인터페이스

모든 네이버 클라우드 API 호출을 정의하는 인터페이스입니다:

```go
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
```

## 🧪 테스트

### MockClient 기능

- **실패 시나리오 시뮬레이션**: `ShouldFail*` 플래그
- **데이터 모킹**: `AddMock*` 메서드
- **호출 추적**: `*Called` 카운터
- **상태 리셋**: `Reset()` 메서드

### 테스트 예제

```go
func TestServiceReconciler(t *testing.T) {
    mockClient := navercloud.NewMockClient()
    
    reconciler := &ServiceReconciler{
        NaverClient: mockClient,
    }
    
    // 테스트 실행
    err := reconciler.CreateLoadBalancer(ctx, service)
    
    // 검증
    assert.NoError(t, err)
    assert.Equal(t, 1, mockClient.CreateLBCalled)
}
```

## 📊 커버리지

이 패키지는 커버리지 계산에서 제외됩니다:

```bash
# API 래퍼 함수를 제외한 커버리지 계산
./coverage-report.sh
```

API 래퍼 함수들은 단순히 SDK 메서드를 호출하는 역할만 하므로, 실제 비즈니스 로직 커버리지에 집중할 수 있습니다.

## 🔄 마이그레이션 가이드

기존 코드에서 이 패키지로 마이그레이션:

### Before (직접 API 호출)
```go
client := vloadbalancer.NewAPIClient(config)
resp, err := client.V2Api.CreateLoadBalancerInstance(&req)
```

### After (인터페이스 사용)
```go
naverClient := navercloud.NewRealClient(lbClient, serverClient)
resp, err := naverClient.CreateLoadBalancerInstance(&req)
```

## 📝 참고사항

- 이 패키지는 네이버 클라우드 SDK v2를 사용합니다
- 모든 API 호출은 에러 처리가 필요합니다
- 테스트 시에는 항상 `MockClient`를 사용하세요
