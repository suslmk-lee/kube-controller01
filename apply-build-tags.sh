#!/bin/bash

# 빌드 태그를 테스트 파일에 자동으로 추가하는 스크립트

set -e

echo "🏷️  빌드 태그 적용 스크립트"
echo "================================"

# 제외할 파일 목록
EXCLUDE_FILES=(
    "internal/controller/simple_coverage_test.go"
    "internal/controller/interface_coverage_test.go"
    "internal/controller/coverage_improvement_test.go"
    "internal/controller/utility_coverage_test.go"
    "internal/controller/mock_client_test.go"
)

BUILD_TAG="//go:build coverage_extra
// +build coverage_extra

"

# 각 파일에 빌드 태그 추가
for file in "${EXCLUDE_FILES[@]}"; do
    if [ -f "$file" ]; then
        echo "📝 Processing: $file"
        
        # 이미 빌드 태그가 있는지 확인
        if grep -q "//go:build" "$file"; then
            echo "   ⚠️  빌드 태그가 이미 존재합니다. 건너뜁니다."
            continue
        fi
        
        # 임시 파일 생성
        temp_file="${file}.tmp"
        
        # 빌드 태그 추가
        echo "$BUILD_TAG" > "$temp_file"
        cat "$file" >> "$temp_file"
        
        # 원본 파일 교체
        mv "$temp_file" "$file"
        
        echo "   ✅ 빌드 태그 추가 완료"
    else
        echo "   ❌ 파일을 찾을 수 없습니다: $file"
    fi
done

echo ""
echo "================================"
echo "✅ 빌드 태그 적용 완료!"
echo ""
echo "📊 테스트 실행 방법:"
echo "  - 핵심 테스트만: go test ./internal/controller/... -coverprofile=cover.out"
echo "  - 전체 테스트:   go test -tags=coverage_extra ./internal/controller/... -coverprofile=cover.out"
echo ""
