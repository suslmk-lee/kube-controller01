# Kubernetes 컨트롤러 배포 가이드

이 문서는 Naver Cloud 로드 밸런서와 연동되는 Kubernetes 컨트롤러를 배포하는 방법을 설명합니다.

## 1. 컨테이너 이미지 빌드 및 푸시

먼저 컨트롤러를 Docker 이미지로 빌드하고 레지스트리에 푸시해야 합니다.

```bash
# 이미지 이름과 태그 설정 (본인의 레지스트리 주소로 변경해주세요)
export IMG=<your-registry>/kube-controller01:latest

# Docker 이미지 빌드
make docker-build

# Docker 이미지 푸시
make docker-push
```

## 2. Naver Cloud API 인증 정보 설정

Naver Cloud API 인증 정보를 쿠버네티스 시크릿으로 생성합니다.

```bash
kubectl create namespace k-paas-system

kubectl create secret generic naver-cloud-credentials \
  --namespace k-paas-system \
  --from-literal=NAVER_CLOUD_API_KEY="your-api-key" \
  --from-literal=NAVER_CLOUD_API_SECRET="your-api-secret" \
  --from-literal=NAVER_CLOUD_REGION="KR" \
  --from-literal=NAVER_CLOUD_VPC_NO="your-vpc-no" \
  --from-literal=NAVER_CLOUD_SUBNET_NO="your-subnet-no"
```

## 3. VPC 및 서브넷 정보 설정

`config/manager/manager.yaml` 파일을 수정하여 환경 변수 설정을 추가합니다:

```bash
# config/manager/manager.yaml 파일 수정
vi config/manager/manager.yaml
```

파일 내의 컨테이너 `env` 섹션에 다음과 같이 환경 변수를 추가합니다:

```yaml
env:
  - name: NAVER_CLOUD_API_KEY
    valueFrom:
      secretKeyRef:
        name: naver-cloud-credentials
        key: NAVER_CLOUD_API_KEY
  - name: NAVER_CLOUD_API_SECRET
    valueFrom:
      secretKeyRef:
        name: naver-cloud-credentials
        key: NAVER_CLOUD_API_SECRET
  - name: NAVER_CLOUD_REGION
    valueFrom:
      secretKeyRef:
        name: naver-cloud-credentials
        key: NAVER_CLOUD_REGION
  - name: NAVER_CLOUD_VPC_NO
    valueFrom:
      secretKeyRef:
        name: naver-cloud-credentials
        key: NAVER_CLOUD_VPC_NO
  - name: NAVER_CLOUD_SUBNET_NO
    valueFrom:
      secretKeyRef:
        name: naver-cloud-credentials
        key: NAVER_CLOUD_SUBNET_NO
```

## 4. 배포 매니페스트 파일 수정

`config/default/kustomization.yaml` 파일을 열고, 이미지 이름과 태그를 설정합니다:

```yaml
images:
- name: controller
  newName: <your-registry>/kube-controller01
  newTag: latest
```

## 5. 코드에서 주석 처리된 부분 활성화

실제 Naver Cloud API를 사용하기 위해 서비스 컨트롤러 코드에서 주석 처리된 부분을 활성화해야 합니다:

1. `service_controller.go` 파일의 import 부분에서 ncloud SDK 관련 주석 제거
2. reconcileNaverCloudLB 및 deleteNaverCloudLB 함수 내의 API 호출 코드 주석 제거

## 6. 컨트롤러 배포

이제 컨트롤러를 쿠버네티스에 배포합니다:

```bash
# 필요한 CRD 적용
make install

# 컨트롤러 배포
make deploy
```

## 7. 배포 확인

컨트롤러가 정상적으로 배포되었는지 확인합니다:

```bash
# 파드 상태 확인
kubectl get pods -n k-paas-system

# 로그 확인
kubectl logs -l control-plane=controller-manager -n k-paas-system
```

## 8. 테스트

LoadBalancer 타입의 서비스를 생성하여 컨트롤러가 제대로 동작하는지 테스트합니다:

```bash
# 테스트용 서비스 생성
cat <<EOF | kubectl apply -f -
apiVersion: v1
kind: Service
metadata:
  name: test-service
spec:
  selector:
    app: test-app
  ports:
  - port: 80
    targetPort: 8080
  type: LoadBalancer
EOF

# 서비스 상태 확인
kubectl get service test-service

# 이벤트 확인
kubectl describe service test-service
```

서비스의 `EXTERNAL-IP` 필드에 Naver Cloud에서 할당된 IP가 표시되면 성공적으로 작동하는 것입니다.

## 문제 해결

컨트롤러에 문제가 있다면 로그를 확인하여 디버깅할 수 있습니다:

```bash
kubectl logs -l control-plane=controller-manager -n k-paas-system -f
```

필요한 경우 컨트롤러를 제거하고 다시 배포할 수 있습니다:

```bash
make undeploy
make deploy
```
