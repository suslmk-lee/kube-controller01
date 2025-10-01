#!/bin/bash

set -e

echo "🧪 Running coverage tests..."
go test ./internal/controller/... -coverprofile=cover.out -covermode=atomic

echo ""
echo "📊 Generating filtered coverage report..."

# interfaces.go의 API 래퍼 함수들을 제외한 커버리지 파일 생성
grep -v "interfaces.go" cover.out > cover_filtered.out

# 전체 커버리지 계산
echo ""
echo "======================================"
echo "📈 Full Coverage Report (All Code)"
echo "======================================"
go tool cover -func=cover.out | tail -1

# 필터링된 커버리지 계산
echo ""
echo "======================================"
echo "✅ Filtered Coverage Report"
echo "   (Excluding API Wrapper Functions)"
echo "======================================"
go tool cover -func=cover_filtered.out | tail -1

# 상세 함수별 커버리지 (0% 제외)
echo ""
echo "======================================"
echo "📋 Coverage by Function (Non-Zero)"
echo "======================================"
go tool cover -func=cover_filtered.out | grep -v "0.0%" | grep -v "total:" | sort -k3 -rn | head -20

# 테스트 가능한 함수들의 평균 커버리지 계산
echo ""
echo "======================================"
echo "🎯 High Coverage Functions (>50%)"
echo "======================================"
go tool cover -func=cover_filtered.out | grep -v "0.0%" | grep -v "total:" | awk '{
    if ($3 != "0.0%") {
        coverage = substr($3, 1, length($3)-1)
        if (coverage > 50) {
            print $0
        }
    }
}' | sort -k3 -rn

# 커버리지가 낮은 함수들
echo ""
echo "======================================"
echo "⚠️  Low Coverage Functions (1-50%)"
echo "======================================"
go tool cover -func=cover_filtered.out | grep -v "0.0%" | grep -v "total:" | awk '{
    if ($3 != "0.0%") {
        coverage = substr($3, 1, length($3)-1)
        if (coverage > 0 && coverage <= 50) {
            print $0
        }
    }
}' | sort -k3 -rn

# 테스트되지 않은 함수들 (참고용)
echo ""
echo "======================================"
echo "❌ Untested Functions (0%)"
echo "======================================"
go tool cover -func=cover_filtered.out | grep "0.0%" | head -10
echo "... (showing first 10 of untested functions)"

# HTML 리포트 생성
echo ""
echo "======================================"
echo "📄 Generating HTML Report"
echo "======================================"
go tool cover -html=cover_filtered.out -o coverage_filtered.html
echo "✅ HTML report generated: coverage_filtered.html"

echo ""
echo "======================================"
echo "📊 Summary"
echo "======================================"
TOTAL_COVERAGE=$(go tool cover -func=cover.out | tail -1 | awk '{print $3}')
FILTERED_COVERAGE=$(go tool cover -func=cover_filtered.out | tail -1 | awk '{print $3}')
TESTABLE_FUNCS=$(go tool cover -func=cover_filtered.out | grep -v "0.0%" | grep -v "total:" | wc -l | tr -d ' ')
UNTESTED_FUNCS=$(go tool cover -func=cover_filtered.out | grep "0.0%" | wc -l | tr -d ' ')

echo "Total Coverage (All Code):        $TOTAL_COVERAGE"
echo "Filtered Coverage (Excl. API):    $FILTERED_COVERAGE"
echo "Testable Functions Covered:       $TESTABLE_FUNCS"
echo "Untested Functions:               $UNTESTED_FUNCS"
echo ""
echo "✅ Coverage report complete!"
echo "   View detailed report: open coverage_filtered.html"
