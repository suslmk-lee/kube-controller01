#!/bin/bash

# Naver Cloud LoadBalancer Controller 빠른 배포 스크립트
# 
# 사용법:
# ./deploy/quick-deploy.sh
# 
# 환경 변수로 설정 가능:
# NAVER_CLOUD_API_KEY=your_key NAVER_CLOUD_API_SECRET=your_secret ./deploy/quick-deploy.sh

set -e

echo "=== Naver Cloud LoadBalancer Controller 배포 ==="

# 색상 정의
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# 함수 정의
log_info() {
    echo -e "${BLUE}ℹ️  $1${NC}"
}

log_success() {
    echo -e "${GREEN}✅ $1${NC}"
}

log_warning() {
    echo -e "${YELLOW}⚠️  $1${NC}"
}

log_error() {
    echo -e "${RED}❌ $1${NC}"
}

# 1. 사전 요구사항 확인
log_info "사전 요구사항 확인 중..."

# kubectl 확인
if ! command -v kubectl &> /dev/null; then
    log_error "kubectl이 설치되지 않았습니다."
    exit 1
fi

# 클러스터 연결 확인
if ! kubectl cluster-info &> /dev/null; then
    log_error "Kubernetes 클러스터에 연결할 수 없습니다."
    exit 1
fi

log_success "kubectl 및 클러스터 연결 확인됨"

# 2. 환경 변수 확인
log_info "네이버 클라우드 환경 변수 확인 중..."

required_vars=("NAVER_CLOUD_API_KEY" "NAVER_CLOUD_API_SECRET" "NAVER_CLOUD_VPC_NO" "NAVER_CLOUD_SUBNET_NO")
missing_vars=()

for var in "${required_vars[@]}"; do
    if [[ -z "${!var}" ]]; then
        missing_vars+=("$var")
    fi
done

if [[ ${#missing_vars[@]} -gt 0 ]]; then
    log_error "다음 환경 변수가 설정되지 않았습니다:"
    for var in "${missing_vars[@]}"; do
        echo "  - $var"
    done
    echo ""
    echo "환경 변수를 설정하고 다시 실행하세요:"
    echo "  export NAVER_CLOUD_API_KEY=your_api_key"
    echo "  export NAVER_CLOUD_API_SECRET=your_api_secret"
    echo "  export NAVER_CLOUD_VPC_NO=your_vpc_no"
    echo "  export NAVER_CLOUD_SUBNET_NO=your_subnet_no"
    echo "  export NAVER_CLOUD_REGION=KR  # 선택사항"
    exit 1
fi

# 기본값 설정
if [[ -z "$NAVER_CLOUD_REGION" ]]; then
    export NAVER_CLOUD_REGION="KR"
fi

log_success "환경 변수 확인 완료"

# 3. 네임스페이스 생성
log_info "네임스페이스 생성 중..."
kubectl create namespace k-paas-system --dry-run=client -o yaml | kubectl apply -f -
log_success "네임스페이스 생성 완료"

# 4. Secret 생성
log_info "네이버 클라우드 인증 정보 Secret 생성 중..."
kubectl create secret generic naver-cloud-credentials \
    --from-literal=NAVER_CLOUD_API_KEY="$NAVER_CLOUD_API_KEY" \
    --from-literal=NAVER_CLOUD_API_SECRET="$NAVER_CLOUD_API_SECRET" \
    --from-literal=NAVER_CLOUD_REGION="$NAVER_CLOUD_REGION" \
    --from-literal=NAVER_CLOUD_VPC_NO="$NAVER_CLOUD_VPC_NO" \
    --from-literal=NAVER_CLOUD_SUBNET_NO="$NAVER_CLOUD_SUBNET_NO" \
    --namespace=k-paas-system \
    --dry-run=client -o yaml | kubectl apply -f -
log_success "Secret 생성 완료"

# 5. 컨트롤러 배포
log_info "컨트롤러 배포 중..."
kubectl apply -f "$(dirname "$0")/kebe-controller-complete.yaml"
log_success "컨트롤러 배포 완료"

# 6. 배포 상태 확인
log_info "배포 상태 확인 중..."
kubectl wait --for=condition=available --timeout=300s deployment/controller-manager -n k-paas-system

if [[ $? -eq 0 ]]; then
    log_success "컨트롤러가 성공적으로 배포되었습니다!"
else
    log_warning "컨트롤러 배포가 완료되지 않았습니다. 상태를 확인하세요."
fi

# 7. 상태 정보 출력
echo ""
log_info "배포 상태 정보:"
echo ""
echo "📋 Pod 상태:"
kubectl get pods -n k-paas-system -o wide

echo ""
echo "📋 서비스 상태:"
kubectl get svc -n k-paas-system

echo ""
echo "📋 최근 이벤트:"
kubectl get events -n k-paas-system --sort-by='.lastTimestamp' | tail -5

# 8. 다음 단계 안내
echo ""
log_info "다음 단계:"
echo "1. 컨트롤러 로그 확인:"
echo "   kubectl logs -n k-paas-system deployment/controller-manager -f"
echo ""
echo "2. 테스트 LoadBalancer 서비스 배포:"
echo "   kubectl apply -f $(dirname "$0")/test-loadbalancer-service.yaml"
echo ""
echo "3. LoadBalancer 서비스 상태 확인:"
echo "   kubectl get svc test-nginx-lb -w"
echo ""
echo "4. 문제 발생 시 디버깅:"
echo "   ./scripts/debug-loadbalancer.sh"

log_success "배포 완료! 🎉"