/*
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
*/

package controller

import (
	"context"
	"fmt"
	"time"

	// 실제 구현시에는 아래 import의 주석을 해제하세요
	// "github.com/NaverCloudPlatform/ncloud-sdk-go-v2/ncloud"
	// "github.com/NaverCloudPlatform/ncloud-sdk-go-v2/services/vloadbalancer"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

// ServiceReconciler reconciles a Service object
type ServiceReconciler struct {
	client.Client
	Scheme *runtime.Scheme
	// Naver Cloud API 클라이언트 설정
	NaverCloudConfig NaverCloudConfig
}

// NaverCloudConfig 구조체는 Naver Cloud API 접근을 위한 설정을 담고 있습니다
type NaverCloudConfig struct {
	APIKey    string
	APISecret string
	Region    string
	// VPC 환경을 위한 설정
	VpcNo    string // VPC 번호
	SubnetNo string // 서브넷 번호
}

// LoadBalancerStatus는 Naver Cloud 로드 밸런서의 상태를 추적합니다
type LoadBalancerStatus struct {
	ProvisioningStatus string
	LBID               string // Naver Cloud에서 사용하는 로드밸런서 인스턴스 번호
	ExternalIP         string // 로드밸런서의 외부 IP 주소
}

// +kubebuilder:rbac:groups=core,resources=services,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=services/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=core,resources=services/finalizers,verbs=update
// +kubebuilder:rbac:groups=core,resources=events,verbs=create;patch

// Reconcile는 쿠버네티스 조정 루프의 일부로, 클러스터의 현재 상태를 원하는 상태에 가깝게 이동시키는 것을 목표로 합니다.
// Service 객체가 지정한 상태와 실제 클러스터 상태를 비교하고,
// 클러스터 상태가 사용자가 지정한 상태를 반영하도록 작업을 수행합니다.
func (r *ServiceReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx).WithValues("service", req.NamespacedName)
	logger.Info("서비스 조정 시작")

	// 서비스 객체 가져오기
	var service corev1.Service
	if err := r.Get(ctx, req.NamespacedName, &service); err != nil {
		if errors.IsNotFound(err) {
			// 서비스가 삭제된 경우, 연결된 Naver Cloud LB도 삭제해야 합니다.
			// 실제 구현에서는 finalizer를 사용하여 처리합니다.
			logger.Info("서비스가 이미 삭제됨")
			return ctrl.Result{}, nil
		}
		logger.Error(err, "서비스 가져오기 실패")
		return ctrl.Result{}, err
	}

	// 서비스가 LoadBalancer 타입인지 확인
	if service.Spec.Type != corev1.ServiceTypeLoadBalancer {
		logger.Info("LoadBalancer 타입이 아닌 서비스 무시")
		return ctrl.Result{}, nil
	}

	// Finalizer 처리
	naverLBFinalizer := "naver.k-paas.org/lb-finalizer"

	// 서비스가 삭제 중인지 확인
	if !service.ObjectMeta.DeletionTimestamp.IsZero() {
		// 삭제 중이고 finalizer가 있는 경우
		if containsString(service.Finalizers, naverLBFinalizer) {
			// Naver Cloud LB 삭제 로직 실행
			if err := r.deleteNaverCloudLB(ctx, &service); err != nil {
				logger.Error(err, "Naver Cloud LB 삭제 실패")
				return ctrl.Result{RequeueAfter: 30 * time.Second}, err
			}

			// Finalizer 제거
			service.Finalizers = removeString(service.Finalizers, naverLBFinalizer)
			if err := r.Update(ctx, &service); err != nil {
				logger.Error(err, "Finalizer 제거 실패")
				return ctrl.Result{}, err
			}
		}
		return ctrl.Result{}, nil
	}

	// Finalizer 추가 (아직 없는 경우)
	if !containsString(service.Finalizers, naverLBFinalizer) {
		service.Finalizers = append(service.Finalizers, naverLBFinalizer)
		if err := r.Update(ctx, &service); err != nil {
			logger.Error(err, "Finalizer 추가 실패")
			return ctrl.Result{}, err
		}
		// 업데이트 후 즉시 반환하여 재조정을 트리거합니다
		return ctrl.Result{}, nil
	}

	// Naver Cloud LB 생성 또는 업데이트 로직
	lbStatus, err := r.reconcileNaverCloudLB(ctx, &service)
	if err != nil {
		logger.Error(err, "Naver Cloud LB 조정 실패")
		return ctrl.Result{RequeueAfter: 30 * time.Second}, err
	}

	// 로드 밸런서가 아직 생성 중인 경우 재시도
	if lbStatus.ProvisioningStatus == "PENDING" {
		logger.Info("Naver Cloud LB가 아직 프로비저닝 중, 재시도 예정")
		return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
	}

	// 서비스 상태 업데이트 (External IP 설정)
	if lbStatus.ExternalIP != "" && (len(service.Status.LoadBalancer.Ingress) == 0 || service.Status.LoadBalancer.Ingress[0].IP != lbStatus.ExternalIP) {
		service.Status.LoadBalancer.Ingress = []corev1.LoadBalancerIngress{
			{IP: lbStatus.ExternalIP},
		}

		if err := r.Status().Update(ctx, &service); err != nil {
			logger.Error(err, "서비스 상태 업데이트 실패")
			return ctrl.Result{}, err
		}
	}

	logger.Info("서비스 조정 완료", "external-ip", lbStatus.ExternalIP)
	return ctrl.Result{}, nil
}

// reconcileNaverCloudLB는 Naver Cloud 로드 밸런서를 생성하거나 업데이트합니다
func (r *ServiceReconciler) reconcileNaverCloudLB(ctx context.Context, service *corev1.Service) (LoadBalancerStatus, error) {
	logger := log.FromContext(ctx).WithValues("service", types.NamespacedName{Namespace: service.Namespace, Name: service.Name})
	logger.Info("Naver Cloud LB 조정 시작")

	// 실제 환경에서는 아래 코드를 사용하여 Naver Cloud API에 접근합니다
	/*
		// Naver Cloud API 접근을 위한 인증 정보 설정
		apiKeys := &ncloud.APIKey{
			AccessKey: r.NaverCloudConfig.APIKey,
			SecretKey: r.NaverCloudConfig.APISecret,
		}

		// Naver Cloud VLoadBalancer API 클라이언트 생성
		config := vloadbalancer.NewConfiguration(apiKeys)
		client := vloadbalancer.NewAPIClient(config)
	*/

	// 실제 구현시에는 위 코드의 주석을 해제하세요

	// 로드 밸런서 ID가 서비스 어노테이션에 있는지 확인
	lbID, exists := service.Annotations["naver.k-paas.org/lb-id"]

	// 서비스에서 포트 정보 가져오기
	var ports []int32
	var protocols []string
	for _, port := range service.Spec.Ports {
		ports = append(ports, port.Port)

		// 프로토콜 설정 (TCP, UDP)
		proto := string(port.Protocol)
		if proto == "" {
			proto = "TCP" // 기본값으로 TCP 사용
		}
		protocols = append(protocols, proto)
	}

	if !exists {
		// 새 로드 밸런서 생성
		// 실제 구현시에는 아래 변수를 사용합니다
		// lbName := fmt.Sprintf("k8s-lb-%s-%s", service.Namespace, service.Name)

		// 실제 환경에서는 아래 코드를 사용합니다
		/*
			// 로드밸런서 생성 요청 구성 (실제 API 문서 참고하여 필요한 필드 수정 필요)
			req := vloadbalancer.CreateLoadBalancerInstanceRequest{
				RegionCode:               ncloud.String(r.NaverCloudConfig.Region),
				LoadBalancerName:          ncloud.String(lbName),
				LoadBalancerDescription:   ncloud.String(fmt.Sprintf("Auto-created by K-PaaS controller for service %s/%s", service.Namespace, service.Name)),
				VpcNo:                      ncloud.String(r.NaverCloudConfig.VpcNo),
				LoadBalancerTypeCode:      ncloud.String("APPLICATION"), // 애플리케이션 로드밸런서 사용
			}
		*/

		// 실제 환경에서는 여기서 Naver Cloud API를 호출하여 로드밸런서 생성
		// 아래는 실제 호출 코드 예시입니다
		/*
			resp, err := client.V2Api.CreateLoadBalancerInstance(&req)
			if err != nil {
				return LoadBalancerStatus{}, fmt.Errorf("로드밸런서 생성 실패: %w", err)
			}

			// 응답 확인
			if resp == nil || len(resp.LoadBalancerInstanceList) == 0 {
				return LoadBalancerStatus{}, fmt.Errorf("로드밸런서 생성 응답이 올바르지 않음")
			}

			// 생성된 로드밸런서 정보 가져오기
			lbInstance := resp.LoadBalancerInstanceList[0]
			lbID = *lbInstance.LoadBalancerInstanceNo

			// IP 가져오기 - 실제 API에 맞게 수정 필요
			var extIP string
			if lbInstance.IpList != nil && len(lbInstance.IpList) > 0 {
				extIP = *lbInstance.IpList[0].Ip
			}
		*/

		// 시뮬레이션 모드에서는 가상 데이터 생성
		lbID = fmt.Sprintf("lb-instance-%s-%s", service.Namespace, service.Name)
		// 가상의 외부 IP 주소 생성
		extIP := fmt.Sprintf("203.0.113.%d", time.Now().Second())

		// 서비스 어노테이션 업데이트
		if service.Annotations == nil {
			service.Annotations = make(map[string]string)
		}
		service.Annotations["naver.k-paas.org/lb-id"] = lbID
		if err := r.Update(ctx, service); err != nil {
			return LoadBalancerStatus{}, fmt.Errorf("서비스 어노테이션 업데이트 실패: %w", err)
		}

		logger.Info("새 Naver Cloud LB 생성됨", "lb-id", lbID, "external-ip", extIP)

		return LoadBalancerStatus{
			ProvisioningStatus: "ACTIVE",
			LBID:               lbID,
			ExternalIP:         extIP, // 새로 선언된 extIP 변수 사용
		}, nil
	}

	// 기존 로드 밸런서 업데이트 로직
	logger.Info("기존 Naver Cloud LB 업데이트", "lb-id", lbID)

	// 실제 환경에서는 여기서 로드밸런서 상태를 확인하고 필요한 경우 업데이트
	// 아래 코드는 시뮬레이션을 위한 코드입니다
	// 가상의 외부 IP 주소 생성
	extIP := fmt.Sprintf("203.0.113.%d", time.Now().Second())
	return LoadBalancerStatus{
		ProvisioningStatus: "ACTIVE",
		LBID:               lbID,
		ExternalIP:         extIP,
	}, nil
}

// deleteNaverCloudLB는 Naver Cloud 로드 밸런서를 삭제합니다
func (r *ServiceReconciler) deleteNaverCloudLB(ctx context.Context, service *corev1.Service) error {
	logger := log.FromContext(ctx).WithValues("service", types.NamespacedName{Namespace: service.Namespace, Name: service.Name})

	// 로드 밸런서 ID가 서비스 어노테이션에 있는지 확인
	lbID, exists := service.Annotations["naver.k-paas.org/lb-id"]
	if !exists {
		// 로드 밸런서 ID가 없으면 이미 삭제되었거나 생성된 적이 없는 것으로 간주
		logger.Info("삭제할 Naver Cloud LB ID를 찾을 수 없음")
		return nil
	}

	// 실제 환경에서는 아래 코드를 사용하여 Naver Cloud API를 호출합니다
	/*
		// Naver Cloud API 접근을 위한 인증 정보 설정
		apiKeys := &ncloud.APIKey{
			AccessKey: r.NaverCloudConfig.APIKey,
			SecretKey: r.NaverCloudConfig.APISecret,
		}

		// Naver Cloud VLoadBalancer API 클라이언트 생성
		config := vloadbalancer.NewConfiguration(apiKeys)
		client := vloadbalancer.NewAPIClient(config)

		// 로드밸런서 삭제 요청 구성
		req := vloadbalancer.DeleteLoadBalancerInstancesRequest{
			RegionCode: ncloud.String(r.NaverCloudConfig.Region),
			LoadBalancerInstanceNoList: []*string{ncloud.String(lbID)},
		}

		// API 호출
		_, err := client.V2Api.DeleteLoadBalancerInstances(&req)
		if err != nil {
			logger.Error(err, "Naver Cloud LB 삭제 실패")
			return fmt.Errorf("로드밸런서 삭제 실패: %w", err)
		}
	*/

	// 실제 Naver Cloud API를 호출하여 LB를 삭제합니다
	logger.Info("Naver Cloud LB 삭제 성공", "lb-id", lbID)

	// 삭제 성공 가정 (시뮬레이션)
	return nil
}

// 문자열 배열에 특정 문자열이 포함되어 있는지 확인하는 헬퍼 함수
func containsString(slice []string, s string) bool {
	for _, item := range slice {
		if item == s {
			return true
		}
	}
	return false
}

// 문자열 배열에서 특정 문자열을 제거하는 헬퍼 함수
func removeString(slice []string, s string) []string {
	result := make([]string, 0, len(slice))
	for _, item := range slice {
		if item != s {
			result = append(result, item)
		}
	}
	return result
}

// SetupWithManager sets up the controller with the Manager.
func (r *ServiceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	// LoadBalancer 타입의 서비스만 감시하는 필터 추가
	isLoadBalancerService := predicate.NewPredicateFuncs(func(object client.Object) bool {
		service, ok := object.(*corev1.Service)
		if !ok {
			return false
		}
		return service.Spec.Type == corev1.ServiceTypeLoadBalancer
	})

	return ctrl.NewControllerManagedBy(mgr).
		For(&corev1.Service{}).
		WithEventFilter(isLoadBalancerService).
		Named("service").
		Complete(r)
}
