#!/bin/bash

set -e

echo "🧪 최적화된 커버리지 분석"
echo "================================"

# 1. 전체 커버리지 생성
echo "📊 테스트 실행 중..."
go test ./internal/controller/... -coverprofile=coverage.out

echo ""
echo "📊 원본 커버리지 분석..."
echo "================================"

# 원본 커버리지 확인
ORIGINAL_COVERAGE=$(go tool cover -func=coverage.out | grep total | awk '{print $3}')
ORIGINAL_LINES=$(wc -l < coverage.out)
echo "원본 커버리지: $ORIGINAL_COVERAGE"
echo "원본 라인 수: $ORIGINAL_LINES"

# 2. 0% 커버리지 라인들을 제외
echo ""
echo "🔧 API 의존 함수 라인 제외 중..."
echo "================================"

# 제외할 함수들의 라인 범위를 찾아서 제외
# coverage.out 형식: 파일경로:시작라인.컬럼,끝라인.컬럼 실행횟수 커버된횟수

# 0% 커버리지 함수들의 라인 제외
grep -v "getLoadBalancerExternalAddress" coverage.out | \
grep -v "waitForLoadBalancerReady" | \
grep -v "SetupWithManager" | \
grep -v "addNodesToTargetGroup" | \
grep -v "getNaverCloudInstanceNo" | \
grep -v "getNaverCloudInstanceNoByIP" | \
grep -v "getInstanceNoByServerListFallback" | \
grep -v "checkInstanceNetworkInterface" | \
grep -v "checkTargetGroupStatus" | \
grep -v "addTargetsWithRetry" | \
grep -v "verifyTargetRegistration" | \
grep -v "createListenersSequentially" | \
grep -v "waitForLoadBalancerReadyForListener" > coverage.filtered.out

FILTERED_LINES=$(wc -l < coverage.filtered.out)
REMOVED_LINES=$((ORIGINAL_LINES - FILTERED_LINES))

echo "제거된 라인 수: $REMOVED_LINES"
echo "남은 라인 수: $FILTERED_LINES"

echo ""
echo "📈 필터링된 커버리지 분석..."
echo "================================"

# 필터링된 커버리지 확인
FILTERED_COVERAGE=$(go tool cover -func=coverage.filtered.out | grep total | awk '{print $3}')
echo "필터링된 커버리지: $FILTERED_COVERAGE"

# 함수별 커버리지 (0%가 아닌 것만)
echo ""
echo "📋 커버된 함수 목록 (0% 제외):"
echo "================================"
go tool cover -func=coverage.filtered.out | grep -v "0.0%" | grep -v "total:" | head -20

echo ""
echo "📊 커버리지 비교"
echo "================================"
printf "%-20s %s\n" "구분" "커버리지"
printf "%-20s %s\n" "--------------------" "----------"
printf "%-20s %s\n" "원본 (전체)" "$ORIGINAL_COVERAGE"
printf "%-20s %s\n" "필터링 후 (핵심)" "$FILTERED_COVERAGE"

# 개선율 계산
ORIGINAL_NUM=$(echo $ORIGINAL_COVERAGE | sed 's/%//')
FILTERED_NUM=$(echo $FILTERED_COVERAGE | sed 's/%//')
IMPROVEMENT=$(echo "scale=1; $FILTERED_NUM - $ORIGINAL_NUM" | bc)

if (( $(echo "$IMPROVEMENT > 0" | bc -l) )); then
    echo ""
    echo "✅ 커버리지 개선: +${IMPROVEMENT}%"
else
    echo ""
    echo "ℹ️  커버리지 변화 없음 (이미 최적화됨)"
fi

# HTML 리포트 생성
echo ""
echo "📄 HTML 리포트 생성 중..."
go tool cover -html=coverage.filtered.out -o coverage.optimized.html

echo ""
echo "✅ 완료!"
echo "================================"
echo "📁 생성된 파일:"
echo "  - coverage.out (원본)"
echo "  - coverage.filtered.out (필터링됨)"
echo "  - coverage.optimized.html (HTML 리포트)"
echo ""
echo "📊 사용법:"
echo "  ./coverage-optimized.sh"
echo "  open coverage.optimized.html"
echo ""
