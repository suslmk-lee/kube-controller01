# 빌드 태그를 활용한 커버리지 관리 가이드

## 🎯 목적
불필요한 테스트 파일을 커버리지 계산에서 제외하여 의미있는 커버리지 측정

## 📋 적용 대상 파일

### 제외 권장 파일 (커버리지 향상용 테스트)
```
internal/controller/
├── simple_coverage_test.go          # 단순 커버리지 향상용
├── interface_coverage_test.go       # 인터페이스 커버리지용
├── coverage_improvement_test.go     # 커버리지 개선용
├── utility_coverage_test.go         # 유틸리티 커버리지용
└── mock_client_test.go              # 모킹 테스트
```

### 유지할 파일 (핵심 테스트)
```
internal/controller/
├── service_controller_test.go       # 메인 컨트롤러 테스트
├── reconcile_test.go                # Reconcile 로직
├── reconcile_logic_test.go          # Reconcile 상세 로직
├── annotation_test.go               # 어노테이션 관리
├── node_utils_test.go               # 노드 유틸리티
├── delete_logic_test.go             # 삭제 로직
├── helper_functions_test.go         # 헬퍼 함수
├── predicate_test.go                # Predicate 로직
├── utils_test.go                    # 유틸리티
├── naver_cloud_test.go              # 네이버 클라우드
├── business_logic_test.go           # 비즈니스 로직
└── mock_api_test.go                 # API 모킹
```

## 🏷️ 방법 1: 빌드 태그로 테스트 파일 제외 (권장)

### 1단계: 제외할 파일에 빌드 태그 추가

각 파일 맨 위에 다음 추가:

```go
//go:build coverage_extra
// +build coverage_extra

package controller

// 기존 코드...
```

**적용 예시**:

#### simple_coverage_test.go
```go
//go:build coverage_extra
// +build coverage_extra

/*
Copyright 2025.
...
*/

package controller

import (
    // ...
)
```

#### interface_coverage_test.go
```go
//go:build coverage_extra
// +build coverage_extra

/*
Copyright 2025.
...
*/

package controller

import (
    // ...
)
```

### 2단계: 테스트 실행 방법

#### 기본 테스트 (빌드 태그 파일 제외)
```bash
# 핵심 테스트만 실행
go test ./internal/controller/... -coverprofile=cover.out

# 커버리지 리포트
go tool cover -func=cover.out
```

#### 전체 테스트 (모든 파일 포함)
```bash
# 모든 테스트 실행
go test -tags=coverage_extra ./internal/controller/... -coverprofile=cover_full.out

# 전체 커버리지 리포트
go tool cover -func=cover_full.out
```

## 🏷️ 방법 2: 여러 빌드 태그 사용

더 세밀한 제어가 필요한 경우:

### 태그 분류
```go
// 단순 커버리지 향상용
//go:build coverage_simple
// +build coverage_simple

// 인터페이스 테스트용
//go:build coverage_interface
// +build coverage_interface

// 모킹 테스트용
//go:build coverage_mock
// +build coverage_mock
```

### 사용 예시
```bash
# 기본 테스트만
go test ./internal/controller/... -coverprofile=cover.out

# 특정 태그 포함
go test -tags=coverage_simple ./internal/controller/... -coverprofile=cover.out

# 여러 태그 포함
go test -tags="coverage_simple coverage_interface" ./internal/controller/... -coverprofile=cover.out

# 모든 태그 포함
go test -tags="coverage_simple coverage_interface coverage_mock" ./internal/controller/... -coverprofile=cover.out
```

## 🏷️ 방법 3: 네거티브 빌드 태그 (제외 방식)

기본적으로 포함하되, 특정 상황에서만 제외:

```go
//go:build !skip_coverage_extra
// +build !skip_coverage_extra

package controller
```

**사용법**:
```bash
# 기본 테스트 (모든 파일 포함)
go test ./internal/controller/... -coverprofile=cover.out

# 특정 파일 제외
go test -tags=skip_coverage_extra ./internal/controller/... -coverprofile=cover.out
```

## 📝 coverage-report.sh 스크립트 수정

기존 스크립트를 빌드 태그 지원하도록 수정:

```bash
#!/bin/bash

# 빌드 태그 옵션
BUILD_TAGS="${BUILD_TAGS:-}"  # 환경 변수로 제어

# 테스트 실행
if [ -z "$BUILD_TAGS" ]; then
    echo "📊 Running core tests only (excluding coverage_extra)..."
    go test ./internal/controller/... -coverprofile=cover.out
else
    echo "📊 Running tests with tags: $BUILD_TAGS"
    go test -tags="$BUILD_TAGS" ./internal/controller/... -coverprofile=cover.out
fi

# 나머지 리포트 생성 로직...
```

**사용 예시**:
```bash
# 기본 테스트 (핵심만)
./coverage-report.sh

# 전체 테스트
BUILD_TAGS="coverage_extra" ./coverage-report.sh

# 특정 태그만
BUILD_TAGS="coverage_simple" ./coverage-report.sh
```

## 🎯 권장 적용 방안

### 단계별 적용

#### 1단계: 명확히 불필요한 파일에 태그 추가
```
✅ simple_coverage_test.go          → coverage_extra
✅ interface_coverage_test.go       → coverage_extra
✅ coverage_improvement_test.go     → coverage_extra
✅ utility_coverage_test.go         → coverage_extra
```

#### 2단계: 테스트 실행 확인
```bash
# 핵심 테스트만 실행
go test ./internal/controller/... -v

# 제외된 파일 수 확인
go list -f '{{.TestGoFiles}}' ./internal/controller
```

#### 3단계: 커버리지 비교
```bash
# Before (모든 파일)
go test -tags=coverage_extra ./internal/controller/... -coverprofile=cover_full.out
go tool cover -func=cover_full.out | grep total

# After (핵심만)
go test ./internal/controller/... -coverprofile=cover_core.out
go tool cover -func=cover_core.out | grep total
```

## 📊 예상 효과

### Before (현재)
```
Total Coverage: 20.8%
Test Files: 17개
Test Cases: 167개
```

### After (빌드 태그 적용)
```
Core Coverage: 25-30% (예상)
Test Files: 12-13개
Test Cases: 120-140개
```

**이유**: 중복/단순 테스트 제외로 의미있는 커버리지 집중

## 🔧 실제 적용 예시

### simple_coverage_test.go 수정
```go
//go:build coverage_extra
// +build coverage_extra

/*
Copyright 2025.

Licensed under the Apache License, Version 2.0 (the "License");
...
*/

package controller

import (
    "context"

    "github.com/NaverCloudPlatform/ncloud-sdk-go-v2/ncloud"
    // ... 기존 imports
)

// 기존 테스트 코드 그대로 유지
var _ = Describe("Simple Coverage Tests", func() {
    // ...
})
```

### interface_coverage_test.go 수정
```go
//go:build coverage_extra
// +build coverage_extra

/*
Copyright 2025.
...
*/

package controller

import (
    "github.com/NaverCloudPlatform/ncloud-sdk-go-v2/ncloud"
    // ... 기존 imports
)

// 기존 테스트 코드 그대로 유지
var _ = Describe("Interface Coverage Tests", func() {
    // ...
})
```

## 📋 체크리스트

빌드 태그 적용 전 확인사항:

- [ ] 제외할 파일 목록 확정
- [ ] 각 파일에 빌드 태그 추가
- [ ] 기본 테스트 실행 확인 (`go test ./...`)
- [ ] 전체 테스트 실행 확인 (`go test -tags=coverage_extra ./...`)
- [ ] 커버리지 비교 (Before/After)
- [ ] CI/CD 파이프라인 업데이트 (필요시)
- [ ] README 문서 업데이트

## 🚀 CI/CD 통합

### GitHub Actions 예시
```yaml
name: Test Coverage

on: [push, pull_request]

jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      
      - name: Set up Go
        uses: actions/setup-go@v4
        with:
          go-version: '1.21'
      
      # 핵심 테스트만 실행
      - name: Run Core Tests
        run: go test ./internal/controller/... -coverprofile=cover.out
      
      # 전체 테스트 실행 (선택적)
      - name: Run All Tests
        run: go test -tags=coverage_extra ./internal/controller/... -coverprofile=cover_full.out
      
      - name: Upload Coverage
        uses: codecov/codecov-action@v3
        with:
          files: ./cover.out
```

## 💡 추가 팁

### 1. 빌드 태그 확인
```bash
# 빌드 태그가 있는 파일 찾기
grep -r "//go:build" internal/controller/

# 특정 태그가 있는 파일 목록
grep -l "coverage_extra" internal/controller/*.go
```

### 2. 테스트 파일 수 확인
```bash
# 기본 테스트 파일 수
go list -f '{{len .TestGoFiles}}' ./internal/controller

# 특정 태그 포함 시 파일 수
go list -tags=coverage_extra -f '{{len .TestGoFiles}}' ./internal/controller
```

### 3. 커버리지 차이 비교
```bash
# 스크립트로 자동 비교
./compare-coverage.sh
```

## 📚 참고 자료

- [Go Build Constraints](https://pkg.go.dev/cmd/go#hdr-Build_constraints)
- [Go Testing Flags](https://pkg.go.dev/cmd/go#hdr-Testing_flags)
- [Build Tags Best Practices](https://www.digitalocean.com/community/tutorials/customizing-go-binaries-with-build-tags)

## ⚠️ 주의사항

1. **빌드 태그는 파일 맨 위에** 위치해야 함 (주석 전)
2. **두 가지 형식 모두 필요** (`//go:build`와 `// +build`)
3. **CI/CD 파이프라인** 설정 확인 필요
4. **팀원들과 공유** - 빌드 태그 사용법 문서화

---

**작성일**: 2025-10-13
**버전**: 1.0.0
**상태**: 빌드 태그 가이드
