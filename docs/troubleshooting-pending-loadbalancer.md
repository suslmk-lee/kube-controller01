# LoadBalancer Pending 상태 문제 해결 가이드

## 문제 상황
LoadBalancer 타입 Service가 pending 상태에서 변화하지 않는 경우

## 진단 단계

### 1. 기본 상태 확인
```bash
# 디버깅 스크립트 실행
./scripts/debug-loadbalancer.sh

# 서비스 상태 확인
kubectl get svc -o wide
kubectl describe svc <service-name>
```

### 2. 네이버 클라우드 API 연결 테스트
```bash
# API 연결 테스트
./scripts/test-naver-api.sh
```

### 3. 컨트롤러 로그 확인
```bash
# 로컬 실행 시
make run

# 클러스터 배포 시
kubectl logs -n kebe-controller01-system deployment/kebe-controller01-controller-manager -f
```

## 일반적인 원인과 해결책

### 1. 환경 변수 미설정
**증상**: 컨트롤러가 시작되지 않거나 API 호출 실패

**해결책**:
```bash
export NAVER_CLOUD_API_KEY=your_api_key
export NAVER_CLOUD_API_SECRET=your_api_secret
export NAVER_CLOUD_VPC_NO=your_vpc_no
export NAVER_CLOUD_SUBNET_NO=your_subnet_no
export NAVER_CLOUD_REGION=KR  # 선택사항
```

### 2. API 권한 부족
**증상**: "403 Forbidden" 또는 권한 관련 에러

**해결책**:
- 네이버 클라우드 콘솔에서 API 키 권한 확인
- VPC LoadBalancer 서비스 권한 확인
- Sub Account인 경우 관리자에게 권한 요청

### 3. VPC/Subnet 설정 오류
**증상**: "Invalid VPC" 또는 "Invalid Subnet" 에러

**해결책**:
```bash
# VPC 목록 확인 (네이버 클라우드 콘솔)
# 올바른 VPC No와 Subnet No 확인
```

### 4. 로드밸런서 생성 실패
**증상**: 로그에서 "로드밸런서 생성 실패" 메시지

**해결책**:
- 네이버 클라우드 콘솔에서 로드밸런서 생성 상태 확인
- 리소스 할당량 확인
- 네트워크 설정 확인

### 5. External IP 획득 실패
**증상**: 로드밸런서는 생성되었지만 External IP가 할당되지 않음

**해결책**:
- 로드밸런서가 완전히 준비될 때까지 대기 (최대 5분)
- 네이버 클라우드 콘솔에서 로드밸런서 상태 확인

## 로그 분석 가이드

### 정상적인 로그 패턴
```
INFO    Naver Cloud LB 조정 시작
INFO    타겟 그룹 생성 성공
INFO    네이버 클라우드 NetworkProxy LB 생성 완료
INFO    로드밸런서 상태 확인    status-code=RUN
INFO    로드밸런서 Domain 획득  domain=lb-12345.ncloud.com
INFO    서비스 상태 업데이트 성공
```

### 문제가 있는 로그 패턴
```
ERROR   Naver Cloud LB 조정 실패
ERROR   타겟 그룹 생성 실패
ERROR   로드밸런서 생성 실패
INFO    로드밸런서가 아직 준비되지 않음    status=CREATING
```

## 수동 확인 방법

### 1. 네이버 클라우드 콘솔 확인
1. VPC > Load Balancer 메뉴 접속
2. 생성된 로드밸런서 상태 확인
3. 타겟 그룹 설정 확인
4. 리스너 설정 확인

### 2. kubectl 명령어로 상세 확인
```bash
# 서비스 어노테이션 확인
kubectl get svc <service-name> -o yaml | grep annotations -A 10

# 이벤트 확인
kubectl get events --field-selector involvedObject.name=<service-name>

# 컨트롤러 상태 확인
kubectl get pods -n kebe-controller01-system
```

## 강제 재시도 방법

### 1. 서비스 재생성
```bash
kubectl delete svc <service-name>
kubectl apply -f <service-yaml>
```

### 2. 컨트롤러 재시작
```bash
# 로컬 실행 시
Ctrl+C 후 make run

# 클러스터 배포 시
kubectl rollout restart deployment/kebe-controller01-controller-manager -n kebe-controller01-system
```

### 3. 어노테이션 제거 (강제 재생성)
```bash
kubectl annotate svc <service-name> naver.k-paas.org/lb-id-
kubectl annotate svc <service-name> naver.k-paas.org/target-groups-
```

## 예방 조치

1. **환경 변수 검증**: 컨트롤러 시작 전 모든 필수 환경 변수 확인
2. **API 권한 테스트**: 정기적으로 API 연결 테스트 실행
3. **리소스 모니터링**: 네이버 클라우드 리소스 사용량 모니터링
4. **로그 모니터링**: 컨트롤러 로그 정기 확인

## 추가 도구

- `./scripts/debug-loadbalancer.sh`: 종합 상태 확인
- `./scripts/test-naver-api.sh`: API 연결 테스트
- `./scripts/test-external-ip.sh`: 전체 기능 테스트