# Naver Cloud LoadBalancer Controller 배포 가이드

## 📋 배포 개요

이 디렉토리는 네이버 클라우드 플랫폼 Kubernetes LoadBalancer 컨트롤러를 Kubernetes 클러스터에 배포하기 위한 YAML 파일들을 포함합니다. NHN Controller 형태와 호환되는 구조로 설계되었습니다.

## 🚀 빠른 시작

### 1. 사전 준비사항

- Kubernetes 클러스터 (v1.20+)
- kubectl 설치 및 클러스터 접근 권한
- 네이버 클라우드 플랫폼 API 키 및 시크릿
- VPC 및 서브넷 정보

### 2. 네이버 클라우드 인증 정보 설정

```bash
# 1. 템플릿 파일 복사
cp deploy/naver-cloud-credentials-template.yaml deploy/naver-cloud-credentials.yaml

# 2. 실제 값으로 수정 (에디터로 열어서 수정)
vi deploy/naver-cloud-credentials.yaml

# 3. Secret 배포
kubectl apply -f deploy/naver-cloud-credentials.yaml

# 4. 보안을 위해 로컬 파일 삭제
rm deploy/naver-cloud-credentials.yaml
```

### 3. 컨트롤러 배포

```bash
# 컨트롤러 배포
kubectl apply -f deploy/kebe-controller-complete.yaml

# 배포 상태 확인
kubectl get pods -n k-paas-system
kubectl logs -n k-paas-system deployment/controller-manager -f
```

### 4. 테스트 서비스 배포

```bash
# 테스트 LoadBalancer 서비스 배포
kubectl apply -f deploy/test-loadbalancer-service.yaml

# External IP 할당 대기
kubectl get svc test-nginx-lb -w

# 외부 접근 테스트
curl http://<EXTERNAL-IP>
```

## 📁 파일 구조

```
deploy/
├── README.md                           # 이 파일
├── kebe-controller-complete.yaml       # 컨트롤러 완전 배포 YAML
├── naver-cloud-credentials-template.yaml # 인증 정보 템플릿
└── test-loadbalancer-service.yaml      # 테스트 서비스
```

## 🔧 상세 배포 가이드

### 컨트롤러 구성 요소

`kebe-controller-complete.yaml`에는 다음 리소스들이 포함되어 있습니다:

- **Namespace**: `k-paas-system`
- **ServiceAccount**: 컨트롤러 실행용 서비스 계정
- **ClusterRole/ClusterRoleBinding**: 필요한 권한 설정
- **Secret**: 네이버 클라우드 인증 정보
- **Deployment**: 컨트롤러 Pod
- **Service**: 메트릭 서비스

### 환경 변수 설정

컨트롤러는 다음 환경 변수를 필요로 합니다:

| 변수명 | 설명 | 예시 |
|--------|------|------|
| `NAVER_CLOUD_API_KEY` | 네이버 클라우드 API 키 | `F4054E1B268386877BC3` |
| `NAVER_CLOUD_API_SECRET` | 네이버 클라우드 API 시크릿 | `41CE79571CD59F7B4A922B6A21786F24EAF4DE71` |
| `NAVER_CLOUD_REGION` | 리전 코드 | `KR` |
| `NAVER_CLOUD_VPC_NO` | VPC 번호 | `5123647` |
| `NAVER_CLOUD_SUBNET_NO` | 서브넷 번호 | `46949` |

### 네이버 클라우드 API 키 권한

API 키는 다음 권한을 가져야 합니다:

- **VPC**: VPC 리소스 조회 및 관리
- **LoadBalancer**: 로드밸런서 생성, 수정, 삭제
- **Server**: 서버 인스턴스 조회
- **NetworkInterface**: 네트워크 인터페이스 조회

## 🔍 문제 해결

### 1. 컨트롤러 상태 확인

```bash
# Pod 상태 확인
kubectl get pods -n k-paas-system

# 로그 확인
kubectl logs -n k-paas-system deployment/controller-manager -f

# 이벤트 확인
kubectl get events -n k-paas-system --sort-by='.lastTimestamp'
```

### 2. LoadBalancer 서비스 문제 해결

```bash
# 서비스 상태 확인
kubectl get svc -o wide

# 서비스 상세 정보
kubectl describe svc <service-name>

# 컨트롤러 로그에서 해당 서비스 관련 로그 확인
kubectl logs -n k-paas-system deployment/controller-manager | grep <service-name>
```

### 3. 디버깅 도구 사용

컨트롤러와 함께 제공되는 디버깅 스크립트를 사용할 수 있습니다:

```bash
# 종합 상태 확인
./scripts/debug-loadbalancer.sh

# 타겟 그룹 상태 확인
./scripts/check-target-group-status.sh

# 통합 테스트
./scripts/test-external-ip.sh
```

## 🔒 보안 고려사항

### 1. Secret 관리

- 프로덕션 환경에서는 Secret을 별도로 관리하세요
- External Secrets Operator 또는 Sealed Secrets 사용 권장
- API 키는 최소 권한 원칙 적용

### 2. 네트워크 보안

- 컨트롤러는 control-plane 노드에서만 실행되도록 설정
- 필요한 경우 NetworkPolicy 적용
- 메트릭 엔드포인트 접근 제한

### 3. RBAC

- 최소 권한으로 ClusterRole 설정
- ServiceAccount 분리 고려
- 정기적인 권한 검토

## 📊 모니터링

### 메트릭 수집

컨트롤러는 다음 메트릭을 제공합니다:

- 처리된 LoadBalancer 서비스 수
- API 호출 성공/실패 횟수
- 타겟 그룹 등록 상태
- 응답 시간 메트릭

### Prometheus 연동

```yaml
# ServiceMonitor 예시 (Prometheus Operator 사용 시)
apiVersion: monitoring.coreos.com/v1
kind: ServiceMonitor
metadata:
  name: kebe-controller01-metrics
  namespace: k-paas-system
spec:
  selector:
    matchLabels:
      control-plane: controller-manager
  endpoints:
  - port: https
    path: /metrics
    scheme: https
    tlsConfig:
      insecureSkipVerify: true
```

## 🔄 업그레이드

### 컨트롤러 업그레이드

```bash
# 새 이미지로 업데이트
kubectl set image deployment/controller-manager \
  manager=registry.k-paas.org/kpaas/naver-controller:v1.1.0 \
  -n k-paas-system

# 롤아웃 상태 확인
kubectl rollout status deployment/controller-manager -n k-paas-system
```

### 설정 변경

```bash
# Secret 업데이트
kubectl create secret generic naver-cloud-credentials \
  --from-literal=NAVER_CLOUD_API_KEY=new_key \
  --from-literal=NAVER_CLOUD_API_SECRET=new_secret \
  --dry-run=client -o yaml | kubectl apply -f -

# 컨트롤러 재시작
kubectl rollout restart deployment/controller-manager -n k-paas-system
```

## 🆘 지원

문제가 발생하거나 도움이 필요한 경우:

1. **로그 확인**: 컨트롤러 로그에서 상세한 에러 메시지 확인
2. **디버깅 도구**: 제공된 스크립트로 상태 분석
3. **네이버 클라우드 콘솔**: 리소스 상태 직접 확인
4. **문서 참조**: troubleshooting 가이드 참조

## 📚 추가 자료

- [네이버 클라우드 플랫폼 API 문서](https://ncloud.apigw.ntruss.com/docs/)
- [Kubernetes LoadBalancer 서비스](https://kubernetes.io/docs/concepts/services-networking/service/#loadbalancer)
- [컨트롤러 개발 문서](../docs/)