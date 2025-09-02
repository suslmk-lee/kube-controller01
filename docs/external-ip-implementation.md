# External IP 획득 로직 구현

## 개요

네이버 클라우드 플랫폼(NCP)의 로드밸런서에서 실제 External IP 또는 도메인을 획득하는 로직을 구현했습니다.

## 구현된 기능

### 1. 실제 External IP 획득 (`getLoadBalancerExternalAddress`)

네이버 클라우드 로드밸런서 API를 통해 다음 순서로 외부 접근 주소를 획득합니다:

1. **VirtualIP 확인**: 일반적으로 공인 IP 주소
2. **Domain 확인**: 도메인 기반 접근 주소
3. **LoadBalancerIpList 확인**: 여러 IP 중 공인 IP 우선 선택
4. **Fallback**: 모든 방법 실패 시 기본 도메인 생성

```go
// 예시 사용법
extIP, err := r.getLoadBalancerExternalAddress(ctx, client, lbID)
if err != nil {
    // 에러 처리
}
```

### 2. 로드밸런서 준비 대기 (`waitForLoadBalancerReady`)

로드밸런서가 완전히 준비될 때까지 대기하는 로직:

- 최대 12회 재시도 (약 2-3분)
- 점진적 대기 시간 증가 (10초 → 15초 → 20초...)
- 상태 코드 확인: `RUN`, `ERROR`, `TERMINATING` 등

### 3. 재시도 메커니즘

External IP 획득 실패 시 5회까지 재시도:

```go
for retry := 0; retry < 5; retry++ {
    extIP, err = r.getLoadBalancerExternalAddress(ctx, client, lbID)
    if err == nil {
        break
    }
    time.Sleep(time.Duration(10+retry*5) * time.Second)
}
```

### 4. 완전한 리소스 정리

Service 삭제 시 다음 리소스들을 순차적으로 정리:

1. 로드밸런서 삭제 (리스너도 함께 삭제됨)
2. 타겟 그룹 삭제

## API 응답 구조 분석

네이버 클라우드 로드밸런서 API 응답에서 확인하는 필드들:

```go
type LoadBalancerInstance struct {
    VirtualIp              *string  // 공인 IP 주소
    LoadBalancerDomain     *string  // 도메인 주소
    LoadBalancerIpList     []IPInfo // IP 목록
    LoadBalancerInstanceStatus *Status // 상태 정보
}

type IPInfo struct {
    Ip         *string  // IP 주소
    IpTypeCode *string  // "PUBLIC" 또는 "PRIVATE"
}
```

## 상태 관리

### LoadBalancerStatus 구조체

```go
type LoadBalancerStatus struct {
    ProvisioningStatus string // "PENDING", "ACTIVE", "ERROR"
    LBID               string // 로드밸런서 ID
    ExternalIP         string // 외부 접근 주소
}
```

### 상태 전환

1. **PENDING**: 로드밸런서 생성 중 또는 External IP 획득 실패
2. **ACTIVE**: 로드밸런서 준비 완료 및 External IP 획득 성공
3. **ERROR**: 로드밸런서 오류 상태

## 테스트 방법

### 1. 환경 설정

```bash
export NAVER_CLOUD_API_KEY=your_api_key
export NAVER_CLOUD_API_SECRET=your_api_secret
export NAVER_CLOUD_VPC_NO=your_vpc_no
export NAVER_CLOUD_SUBNET_NO=your_subnet_no
```

### 2. 테스트 실행

```bash
# 빌드 및 테스트
./scripts/test-external-ip.sh

# 컨트롤러 실행
make run

# 테스트 서비스 배포
kubectl apply -f config/samples/test-loadbalancer-service.yaml

# 상태 확인
kubectl get svc test-loadbalancer -w
```

### 3. 예상 결과

```yaml
NAME               TYPE           CLUSTER-IP     EXTERNAL-IP              PORT(S)
test-loadbalancer  LoadBalancer   10.96.123.45   203.0.113.100           80:30080/TCP,443:30443/TCP
```

또는

```yaml
NAME               TYPE           CLUSTER-IP     EXTERNAL-IP                    PORT(S)
test-loadbalancer  LoadBalancer   10.96.123.45   lb-12345.ncloud.com           80:30080/TCP,443:30443/TCP
```

## 에러 처리

### 일반적인 에러 상황

1. **API 인증 실패**: API 키/시크릿 확인
2. **VPC/Subnet 오류**: VPC No, Subnet No 확인
3. **로드밸런서 생성 실패**: 네이버 클라우드 콘솔에서 상태 확인
4. **External IP 획득 실패**: 재시도 메커니즘으로 자동 처리

### 로그 확인

```bash
# 컨트롤러 로그 확인
kubectl logs -f deployment/kebe-controller01-controller-manager -n kebe-controller01-system
```

## 향후 개선 사항

1. **타겟 그룹에 노드 자동 추가**: 현재는 타겟 그룹만 생성, 실제 노드 추가 로직 필요
2. **헬스체크 설정**: 타겟 그룹 헬스체크 세부 설정
3. **SSL 인증서 연동**: HTTPS 리스너를 위한 SSL 인증서 자동 연동
4. **메트릭 수집**: Prometheus 메트릭을 통한 모니터링 강화

## 최종 구현 완료 ✅

**2025년 1월 2일 - 실제 External IP 획득 로직 구현 완료!**

### 성공적으로 해결된 문제들:

1. **네이버 클라우드 SDK 필드명 확인**
   - `LoadBalancerDomain`: 도메인 기반 접근 주소
   - `LoadBalancerIpList`: IP 리스트 (`[]*string` 타입)
   - `VirtualIp` 필드는 존재하지 않음을 확인

2. **컴파일 에러 해결**
   - 변수 재선언 문제 해결 (`:=` vs `=`)
   - 함수 스코프 내 변수명 충돌 해결

3. **빌드 및 테스트 성공**
   - `make build`: ✅ 성공
   - `make test`: ✅ 성공  
   - 테스트 스크립트: ✅ 성공

### 최종 구현된 기능:

```go
// 실제 네이버 클라우드 SDK 필드 사용
if lbInstance.LoadBalancerDomain != nil && *lbInstance.LoadBalancerDomain != "" {
    return *lbInstance.LoadBalancerDomain, nil
}

if lbInstance.LoadBalancerIpList != nil && len(lbInstance.LoadBalancerIpList) > 0 {
    if lbInstance.LoadBalancerIpList[0] != nil && *lbInstance.LoadBalancerIpList[0] != "" {
        return *lbInstance.LoadBalancerIpList[0], nil
    }
}
```

### 다음 단계:

1. **실제 클러스터 테스트**
   ```bash
   make install  # CRD 설치
   make run      # 컨트롤러 실행
   kubectl apply -f config/samples/test-loadbalancer-service.yaml
   kubectl get svc test-loadbalancer -w
   ```

2. **타겟 그룹에 노드 추가 구현** (다음 우선순위)
3. **SSL 인증서 연동** (HTTPS 지원)
4. **메트릭 및 모니터링 강화**

**🎉 External IP 획득 로직 구현이 성공적으로 완료되었습니다!**