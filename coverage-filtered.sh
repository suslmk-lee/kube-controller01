#!/bin/bash

set -e

echo "🧪 테스트 실행 중..."
echo "================================"

# 1. 전체 커버리지 생성
go test ./internal/controller/... -coverprofile=coverage.out

echo ""
echo "📊 원본 커버리지 분석..."
echo "================================"

# 원본 커버리지 확인
ORIGINAL_COVERAGE=$(go tool cover -func=coverage.out | grep total | awk '{print $3}')
echo "원본 커버리지: $ORIGINAL_COVERAGE"

# 2. 0% 커버리지 함수들 제외 (API 의존 함수들)
echo ""
echo "🔧 0% 커버리지 함수 제외 중..."
echo "================================"

# 제외할 함수 패턴들
EXCLUDE_PATTERNS=(
    "getLoadBalancerExternalAddress"
    "waitForLoadBalancerReady"
    "SetupWithManager"
    "addNodesToTargetGroup"
    "getNaverCloudInstanceNo"
    "getNaverCloudInstanceNoByIP"
    "getInstanceNoByServerListFallback"
    "checkInstanceNetworkInterface"
    "checkTargetGroupStatus"
    "addTargetsWithRetry"
    "verifyTargetRegistration"
    "createListenersSequentially"
    "waitForLoadBalancerReadyForListener"
)

# 커버리지 파일 필터링
cp coverage.out coverage.filtered.out

for pattern in "${EXCLUDE_PATTERNS[@]}"; do
    echo "  - 제외: $pattern"
    grep -v "$pattern" coverage.filtered.out > coverage.filtered.tmp || true
    mv coverage.filtered.tmp coverage.filtered.out
done

echo ""
echo "📈 필터링된 커버리지 분석..."
echo "================================"

# 필터링된 커버리지 확인
FILTERED_COVERAGE=$(go tool cover -func=coverage.filtered.out | grep total | awk '{print $3}')
echo "필터링된 커버리지: $FILTERED_COVERAGE"

echo ""
echo "📊 커버리지 비교"
echo "================================"
echo "원본:       $ORIGINAL_COVERAGE"
echo "필터링 후:  $FILTERED_COVERAGE"

# HTML 리포트 생성
echo ""
echo "📄 HTML 리포트 생성 중..."
go tool cover -html=coverage.filtered.out -o coverage.filtered.html

echo ""
echo "✅ 완료!"
echo "================================"
echo "HTML 리포트: coverage.filtered.html"
echo ""
echo "사용법:"
echo "  ./coverage-filtered.sh"
echo "  open coverage.filtered.html"
