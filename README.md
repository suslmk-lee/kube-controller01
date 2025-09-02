# kebe-controller01

K-PaaS 환경에서 Kubernetes Service와 네이버 클라우드 플랫폼(NCP) 로드밸런서를 자동으로 연동하는 컨트롤러입니다.

## Description

이 컨트롤러는 Kubernetes의 LoadBalancer 타입 Service가 생성될 때 자동으로 네이버 클라우드의 Network Proxy 로드밸런서를 생성하고, 실제 External IP 또는 도메인을 Service 상태에 반영합니다. 

### 주요 기능

- **자동 로드밸런서 생성**: LoadBalancer 타입 Service 감지 시 NCP 로드밸런서 자동 생성
- **실제 External IP 획득**: 네이버 클라우드 API를 통해 실제 공인 IP 또는 도메인 획득
- **타겟 그룹 관리**: 각 포트별 타겟 그룹 및 리스너 자동 구성
- **안전한 리소스 정리**: Finalizer를 통한 Service 삭제 시 관련 NCP 리소스 자동 삭제
- **상태 모니터링**: 로드밸런서 상태 실시간 추적 및 재시도 메커니즘

## Getting Started

### Prerequisites
- go version v1.23.0+
- docker version 17.03+
- kubectl version v1.11.3+
- Access to a Kubernetes v1.11.3+ cluster
- 네이버 클라우드 플랫폼 계정 및 API 키
- VPC 환경 (VPC No, Subnet No 필요)

### Environment Variables

컨트롤러 실행 전 다음 환경 변수를 설정해야 합니다:

```bash
export NAVER_CLOUD_API_KEY=your_api_key
export NAVER_CLOUD_API_SECRET=your_api_secret
export NAVER_CLOUD_VPC_NO=your_vpc_no
export NAVER_CLOUD_SUBNET_NO=your_subnet_no
export NAVER_CLOUD_REGION=KR  # 선택사항, 기본값: KR
```

### To Deploy on the cluster
**Build and push your image to the location specified by `IMG`:**

```sh
make docker-build docker-push IMG=<some-registry>/kebe-controller01:tag
```

**NOTE:** This image ought to be published in the personal registry you specified.
And it is required to have access to pull the image from the working environment.
Make sure you have the proper permission to the registry if the above commands don’t work.

**Install the CRDs into the cluster:**

```sh
make install
```

**Deploy the Manager to the cluster with the image specified by `IMG`:**

```sh
make deploy IMG=<some-registry>/kebe-controller01:tag
```

> **NOTE**: If you encounter RBAC errors, you may need to grant yourself cluster-admin
privileges or be logged in as admin.

**Create instances of your solution**
You can apply the samples (examples) from the config/sample:

```sh
kubectl apply -k config/samples/
```

**Test LoadBalancer Service**
테스트용 LoadBalancer Service를 배포하여 External IP 획득을 확인할 수 있습니다:

```sh
kubectl apply -f config/samples/test-loadbalancer-service.yaml
kubectl get svc test-loadbalancer -w
```

>**NOTE**: External IP가 할당되기까지 1-3분 정도 소요될 수 있습니다.

### To Uninstall
**Delete the instances (CRs) from the cluster:**

```sh
kubectl delete -k config/samples/
```

**Delete the APIs(CRDs) from the cluster:**

```sh
make uninstall
```

**UnDeploy the controller from the cluster:**

```sh
make undeploy
```

## Project Distribution

Following the options to release and provide this solution to the users.

### By providing a bundle with all YAML files

1. Build the installer for the image built and published in the registry:

```sh
make build-installer IMG=<some-registry>/kebe-controller01:tag
```

**NOTE:** The makefile target mentioned above generates an 'install.yaml'
file in the dist directory. This file contains all the resources built
with Kustomize, which are necessary to install this project without its
dependencies.

2. Using the installer

Users can just run 'kubectl apply -f <URL for YAML BUNDLE>' to install
the project, i.e.:

```sh
kubectl apply -f https://raw.githubusercontent.com/<org>/kebe-controller01/<tag or branch>/dist/install.yaml
```

### By providing a Helm Chart

1. Build the chart using the optional helm plugin

```sh
kubebuilder edit --plugins=helm/v1-alpha
```

2. See that a chart was generated under 'dist/chart', and users
can obtain this solution from there.

**NOTE:** If you change the project, you need to update the Helm Chart
using the same command above to sync the latest changes. Furthermore,
if you create webhooks, you need to use the above command with
the '--force' flag and manually ensure that any custom configuration
previously added to 'dist/chart/values.yaml' or 'dist/chart/manager/manager.yaml'
is manually re-applied afterwards.

## Contributing
// TODO(user): Add detailed information on how you would like others to contribute to this project

**NOTE:** Run `make help` for more information on all potential `make` targets

More information can be found via the [Kubebuilder Documentation](https://book.kubebuilder.io/introduction.html)

## License

Copyright 2025.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.

