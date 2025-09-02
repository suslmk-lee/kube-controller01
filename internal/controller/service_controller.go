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
	"regexp"
	"strings"
	"time"

	"github.com/NaverCloudPlatform/ncloud-sdk-go-v2/ncloud"
	"github.com/NaverCloudPlatform/ncloud-sdk-go-v2/services/vloadbalancer"
	"github.com/NaverCloudPlatform/ncloud-sdk-go-v2/services/vserver"

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
		logger.Error(err, "Naver Cloud LB 조정 실패",
			"service-name", service.Name,
			"service-namespace", service.Namespace,
			"service-type", service.Spec.Type)
		return ctrl.Result{RequeueAfter: 30 * time.Second}, err
	}

	// 로드 밸런서가 아직 생성 중인 경우 재시도
	if lbStatus.ProvisioningStatus == "PENDING" || lbStatus.ProvisioningStatus == "CREATING" {
		logger.Info("Naver Cloud LB가 아직 프로비저닝 중, 재시도 예정",
			"status", lbStatus.ProvisioningStatus,
			"lb-id", lbStatus.LBID,
			"external-ip", lbStatus.ExternalIP,
			"requeue-after", "30s")
		return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
	}

	// 로드 밸런서가 오류 상태인 경우
	if lbStatus.ProvisioningStatus == "ERROR" {
		logger.Error(nil, "Naver Cloud LB가 오류 상태",
			"status", lbStatus.ProvisioningStatus,
			"lb-id", lbStatus.LBID)
		return ctrl.Result{RequeueAfter: 60 * time.Second}, fmt.Errorf("로드밸런서가 오류 상태: %s", lbStatus.ProvisioningStatus)
	}

	// 서비스 상태 업데이트 (External IP 또는 Hostname 설정)
	if lbStatus.ExternalIP != "" {
		// 도메인 이름인지 IP 주소인지 확인
		var ingress corev1.LoadBalancerIngress

		// IP 주소 형식 확인 (4개의 숫자 그룹으로 구성된 주소)
		ipPattern := regexp.MustCompile(`^\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3}$`)
		if ipPattern.MatchString(lbStatus.ExternalIP) {
			// IP 주소인 경우
			ingress = corev1.LoadBalancerIngress{IP: lbStatus.ExternalIP}
		} else {
			// 도메인 이름인 경우
			ingress = corev1.LoadBalancerIngress{Hostname: lbStatus.ExternalIP}
		}

		// 서비스 상태를 업데이트하기 전에 가장 최신 버전의 서비스 객체를 다시 가져옵니다
		// 이렇게 하면 동시성 문제를 해결할 수 있습니다
		var latestService corev1.Service
		if err := r.Get(ctx, types.NamespacedName{Namespace: service.Namespace, Name: service.Name}, &latestService); err != nil {
			logger.Error(err, "최신 서비스 객체 가져오기 실패")
			return ctrl.Result{RequeueAfter: 5 * time.Second}, err
		}

		// 현재 서비스 어노테이션 보존
		if latestService.Annotations == nil {
			latestService.Annotations = make(map[string]string)
		}

		// 로드밸런서 ID 어노테이션 보존
		if lbStatus.LBID != "" {
			latestService.Annotations["naver.k-paas.org/lb-id"] = lbStatus.LBID
		}

		// 포트 정보 어노테이션 이전
		if portInfo, ok := service.Annotations["naver.k-paas.org/ports"]; ok {
			latestService.Annotations["naver.k-paas.org/ports"] = portInfo
		}

		// 기존 설정과 다른 경우에만 업데이트
		if len(latestService.Status.LoadBalancer.Ingress) == 0 ||
			(ingress.IP != "" && (len(latestService.Status.LoadBalancer.Ingress) == 0 || latestService.Status.LoadBalancer.Ingress[0].IP != ingress.IP)) ||
			(ingress.Hostname != "" && (len(latestService.Status.LoadBalancer.Ingress) == 0 || latestService.Status.LoadBalancer.Ingress[0].Hostname != ingress.Hostname)) {

			latestService.Status.LoadBalancer.Ingress = []corev1.LoadBalancerIngress{ingress}

			// 업데이트 전 로깅
			logger.Info("서비스 상태 업데이트 시도", "ingress-type", func() string {
				if ingress.IP != "" {
					return "IP: " + ingress.IP
				}
				return "Hostname: " + ingress.Hostname
			}())

			// 상태 업데이트
			if err := r.Status().Update(ctx, &latestService); err != nil {
				logger.Error(err, "서비스 상태 업데이트 실패")
				return ctrl.Result{RequeueAfter: 5 * time.Second}, err
			}

			// 성공 로그
			logger.Info("서비스 상태 업데이트 성공")
		} else {
			logger.Info("서비스 상태 업데이트 필요 없음", "current-ingress", latestService.Status.LoadBalancer.Ingress)
		}
	}

	return ctrl.Result{}, nil
}

// reconcileNaverCloudLB는 Naver Cloud 로드 밸런서를 생성하거나 업데이트합니다
func (r *ServiceReconciler) reconcileNaverCloudLB(ctx context.Context, service *corev1.Service) (LoadBalancerStatus, error) {
	logger := log.FromContext(ctx).WithValues("service", types.NamespacedName{Namespace: service.Namespace, Name: service.Name})
	logger.Info("Naver Cloud LB 조정 시작")

	// Naver Cloud API 접근을 위한 인증 정보 설정
	apiKeys := &ncloud.APIKey{
		AccessKey: r.NaverCloudConfig.APIKey,
		SecretKey: r.NaverCloudConfig.APISecret,
	}

	// Naver Cloud VLoadBalancer API 클라이언트 생성
	config := vloadbalancer.NewConfiguration(apiKeys)
	// 공공망 엔드포인트 설정
	config.BasePath = "https://ncloud.apigw.gov-ntruss.com/vloadbalancer/v2"
	client := vloadbalancer.NewAPIClient(config)

	// 타겟 그룹 ID 및 로드밸런서 ID가 서비스 어노테이션에 있는지 확인
	targetGroupsStr, targetGroupsExist := service.Annotations["naver.k-paas.org/target-groups"]
	lbID, lbExists := service.Annotations["naver.k-paas.org/lb-id"]

	// 이미 생성된 타겟 그룹 ID 배열 생성
	targetGroupIDs := []string{}
	if targetGroupsExist && targetGroupsStr != "" {
		targetGroupIDs = strings.Split(targetGroupsStr, ",")
	}

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

	if !lbExists {
		// 새 로드 밸런서 생성 (이름 길이 제한 고려)
		lbName := r.generateValidName("k8s-lb", service.Namespace, service.Name, "")

		// 1. 각 포트마다 타겟 그룹 먼저 생성
		targetGroupIDs = []string{}
		for i, port := range service.Spec.Ports {
			tgName := r.generateValidName("tg", service.Namespace, service.Name, fmt.Sprintf("%d", i))
			logger.Info("타겟 그룹 이름 생성", "original-parts", fmt.Sprintf("tg-%s-%s-%d", service.Namespace, service.Name, i), "generated-name", tgName)

			// 프로토콜 설정
			protocolType := string(port.Protocol)
			if protocolType == "" {
				protocolType = "TCP"
			}

			// 타겟 그룹 생성 요청 - SDK의 정확한 필드명 사용
			tgReq := vloadbalancer.CreateTargetGroupRequest{
				RegionCode:                  ncloud.String(r.NaverCloudConfig.Region),
				VpcNo:                       ncloud.String(r.NaverCloudConfig.VpcNo),
				TargetGroupName:             ncloud.String(tgName),
				TargetTypeCode:              ncloud.String("VSVR"), // 가상 서버 타입
				TargetGroupPort:             ncloud.Int32(port.NodePort),
				TargetGroupProtocolTypeCode: ncloud.String("PROXY_TCP"),
				TargetGroupDescription:      ncloud.String(fmt.Sprintf("Target group for %s/%s port %d", service.Namespace, service.Name, port.Port)),
				HealthCheckProtocolTypeCode: ncloud.String(protocolType),
				HealthCheckPort:             ncloud.Int32(port.NodePort),
			}

			// 타겟 그룹 생성 API 호출
			tgResp, err := client.V2Api.CreateTargetGroup(&tgReq)
			var targetGroupID string

			if err != nil {
				logger.Info("타겟 그룹 생성 API 에러 발생, 실제 생성 여부 확인 중", "port", port.Port, "error", err)

				// 에러가 발생해도 실제로는 생성되었을 수 있으므로 조회해봄
				// 타겟 그룹 이름으로 조회 시도
				listReq := vloadbalancer.GetTargetGroupListRequest{
					RegionCode: ncloud.String(r.NaverCloudConfig.Region),
					VpcNo:      ncloud.String(r.NaverCloudConfig.VpcNo),
				}

				listResp, listErr := client.V2Api.GetTargetGroupList(&listReq)
				if listErr == nil && listResp != nil {
					// 생성하려던 이름과 일치하는 타겟 그룹 찾기
					for _, tg := range listResp.TargetGroupList {
						if tg.TargetGroupName != nil && *tg.TargetGroupName == tgName {
							targetGroupID = *tg.TargetGroupNo
							logger.Info("기존 타겟 그룹 발견됨", "targetGroupID", targetGroupID, "name", tgName)
							break
						}
					}
				}

				// 여전히 타겟 그룹을 찾을 수 없으면 에러 반환
				if targetGroupID == "" {
					logger.Error(err, "타겟 그룹 생성 및 조회 모두 실패", "port", port.Port)
					return LoadBalancerStatus{}, fmt.Errorf("타겟 그룹 생성 실패: %w", err)
				}
			} else {
				// 정상 응답 처리
				if tgResp == nil || len(tgResp.TargetGroupList) == 0 {
					logger.Error(nil, "타겟 그룹 응답이 올바르지 않음")
					return LoadBalancerStatus{}, fmt.Errorf("타겟 그룹 생성 응답이 올바르지 않음")
				}
				targetGroupID = *tgResp.TargetGroupList[0].TargetGroupNo
			}

			// 타겟 그룹 ID 저장
			targetGroupIDs = append(targetGroupIDs, targetGroupID)

			// 상태 업데이트 전에 가장 최신 버전의 서비스 객체를 다시 가져옵니다
			var tempService corev1.Service
			if err := r.Get(ctx, types.NamespacedName{Namespace: service.Namespace, Name: service.Name}, &tempService); err != nil {
				logger.Error(err, "최신 서비스 객체 가져오기 실패")
			} else {
				// 타겟 그룹 ID를 어노테이션에 저장
				if tempService.Annotations == nil {
					tempService.Annotations = make(map[string]string)
				}

				// 기존 타겟 그룹 정보가 있는지 확인
				existingGroups := []string{}
				if tgStr, ok := tempService.Annotations["naver.k-paas.org/target-groups"]; ok && tgStr != "" {
					existingGroups = strings.Split(tgStr, ",")
				}

				// 이미 있는지 확인
				alreadyExists := false
				for _, existingID := range existingGroups {
					if existingID == targetGroupID {
						alreadyExists = true
						break
					}
				}

				// 없으면 추가
				if !alreadyExists {
					existingGroups = append(existingGroups, targetGroupID)
					tempService.Annotations["naver.k-paas.org/target-groups"] = strings.Join(existingGroups, ",")

					// 어노테이션 업데이트
					if err := r.Update(ctx, &tempService); err != nil {
						logger.Error(err, "타겟 그룹 어노테이션 업데이트 실패")
					}
				}
			}

			logger.Info("타겟 그룹 생성 성공", "targetGroupID", targetGroupID, "port", port.Port)

			// NetworkProxy 타입에서도 타겟 추가가 필요할 수 있음 - 다시 시도
			if err := r.addNodesToTargetGroup(ctx, client, targetGroupID, port.NodePort); err != nil {
				logger.Error(err, "타겟 그룹에 노드 추가 실패", "targetGroupID", targetGroupID)
				// 노드 추가 실패는 전체 프로세스를 중단하지 않지만 경고 로그 출력
			} else {
				logger.Info("타겟 그룹에 노드 추가 성공", "targetGroupID", targetGroupID)
			}
		}

		// 로드밸런서 생성 요청 구성 (디버깅용 로그 추가)
		logger.Info("로드밸런서 생성 요청 구성",
			"VpcNo", r.NaverCloudConfig.VpcNo,
			"SubnetNo", r.NaverCloudConfig.SubnetNo,
			"Region", r.NaverCloudConfig.Region)

		req := vloadbalancer.CreateLoadBalancerInstanceRequest{
			RegionCode:              ncloud.String(r.NaverCloudConfig.Region),
			LoadBalancerName:        ncloud.String(lbName),
			LoadBalancerDescription: ncloud.String(fmt.Sprintf("Auto-created by K-PaaS controller for service %s/%s", service.Namespace, service.Name)),
			VpcNo:                   ncloud.String(r.NaverCloudConfig.VpcNo),
			LoadBalancerTypeCode:    ncloud.String("NETWORK_PROXY"),                        // 네트워크 프록시 로드밸런서 사용
			SubnetNoList:            []*string{ncloud.String(r.NaverCloudConfig.SubnetNo)}, // 서브넷 정보 추가
		}

		// Naver Cloud API를 호출하여 로드밸런서 생성
		resp, err := client.V2Api.CreateLoadBalancerInstance(&req)
		if err != nil {
			// 중복 이름 오류인 경우 기존 로드밸런서 찾기
			if strings.Contains(err.Error(), "1200013") || strings.Contains(err.Error(), "Duplicate load balancer name") {
				logger.Info("중복 로드밸런서 이름 오류, 기존 로드밸런서 검색", "lb-name", lbName)

				// 기존 로드밸런서 조회
				listReq := vloadbalancer.GetLoadBalancerInstanceListRequest{
					RegionCode: ncloud.String(r.NaverCloudConfig.Region),
					VpcNo:      ncloud.String(r.NaverCloudConfig.VpcNo),
				}

				listResp, listErr := client.V2Api.GetLoadBalancerInstanceList(&listReq)
				if listErr == nil && listResp != nil {
					// 같은 이름의 로드밸런서 찾기
					for _, lb := range listResp.LoadBalancerInstanceList {
						if lb.LoadBalancerName != nil && *lb.LoadBalancerName == lbName {
							lbID = *lb.LoadBalancerInstanceNo
							logger.Info("기존 로드밸런서 발견됨", "lb-id", lbID, "lb-name", lbName)

							// 기존 로드밸런서를 사용하므로 resp 구조체 생성
							resp = &vloadbalancer.CreateLoadBalancerInstanceResponse{
								LoadBalancerInstanceList: []*vloadbalancer.LoadBalancerInstance{lb},
							}
							err = nil // 에러 클리어
							break
						}
					}
				}

				// 여전히 찾을 수 없으면 원래 에러 반환
				if err != nil {
					return LoadBalancerStatus{}, fmt.Errorf("로드밸런서 생성 실패: %w", err)
				}
			} else {
				return LoadBalancerStatus{}, fmt.Errorf("로드밸런서 생성 실패: %w", err)
			}
		}

		// 응답 확인
		if resp == nil || len(resp.LoadBalancerInstanceList) == 0 {
			return LoadBalancerStatus{}, fmt.Errorf("로드밸런서 생성 응답이 올바르지 않음")
		}

		// 생성된 로드밸런서 정보 가져오기
		lbInstance := resp.LoadBalancerInstanceList[0]
		lbID = *lbInstance.LoadBalancerInstanceNo

		// 로드밸런서가 준비될 때까지 대기 (상태가 Running이 될 때까지)
		if err := r.waitForLoadBalancerReady(ctx, client, lbID, 10); err != nil {
			logger.Info("로드밸런서 준비 대기 실패, 리스너 생성 계속 시도", "error", err.Error())
			// 계속 진행하되 리스너 생성은 나중에 재시도
		} else {
			logger.Info("로드밸런서 준비 완료, 리스너 생성 시작")
		}

		// 2. 로드밸런서 생성 후 리스너 추가
		logger.Info("리스너 생성 시작", "targetGroupCount", len(targetGroupIDs), "portCount", len(service.Spec.Ports))

		// 기존 리스너 조회
		existingListeners := make(map[int32]bool) // 포트별 기존 리스너 맵
		listenerListReq := vloadbalancer.GetLoadBalancerListenerListRequest{
			RegionCode:             ncloud.String(r.NaverCloudConfig.Region),
			LoadBalancerInstanceNo: &lbID,
		}

		listenerListResp, listErr := client.V2Api.GetLoadBalancerListenerList(&listenerListReq)
		if listErr == nil && listenerListResp != nil {
			for _, listener := range listenerListResp.LoadBalancerListenerList {
				if listener.Port != nil {
					existingListeners[*listener.Port] = true
					logger.Info("기존 리스너 발견", "port", *listener.Port)
				}
			}
		} else {
			logger.Info("기존 리스너 조회 실패 또는 없음", "error", listErr)
		}

		if len(targetGroupIDs) > 0 {
			// 리스너 추가 (기존에 없는 것만)
			for i, port := range service.Spec.Ports {
				if i >= len(targetGroupIDs) {
					continue
				}

				// 이미 해당 포트의 리스너가 있는지 확인
				if existingListeners[port.Port] {
					logger.Info("기존 리스너 재사용", "port", port.Port)
					continue
				}

				// 리스너는 일반 TCP 프로토콜 사용 (타겟 그룹은 PROXY_TCP이지만 리스너는 TCP)
				protocolType := "TCP"
				if port.Protocol == "UDP" {
					protocolType = "UDP"
				}

				logger.Info("리스너 생성 요청", "port", port.Port, "protocol", protocolType, "targetGroupID", targetGroupIDs[i])

				listenerReq := vloadbalancer.CreateLoadBalancerListenerRequest{
					RegionCode:             ncloud.String(r.NaverCloudConfig.Region),
					LoadBalancerInstanceNo: &lbID,
					ProtocolTypeCode:       ncloud.String(protocolType), // 리스너는 일반 TCP/UDP 사용
					Port:                   ncloud.Int32(int32(port.Port)),
					TargetGroupNo:          &targetGroupIDs[i],
				}

				// 리스너 생성 API 호출
				_, err = client.V2Api.CreateLoadBalancerListener(&listenerReq)
				if err != nil {
					logger.Error(err, "리스너 생성 실패", "port", port.Port)
					continue
				}

				logger.Info("리스너 생성 성공", "port", port.Port, "targetGroupID", targetGroupIDs[i])
			}
		}

		// 서비스 어노테이션 설정
		if service.Annotations == nil {
			service.Annotations = make(map[string]string)
		}

		// 로드밸런서 ID 저장
		service.Annotations["naver.k-paas.org/lb-id"] = lbID

		// 타겟 그룹 ID 저장
		if len(targetGroupIDs) > 0 {
			service.Annotations["naver.k-paas.org/target-groups"] = strings.Join(targetGroupIDs, ",")
		}

		// 로그에 로드밸런서 생성 정보 출력
		logger.Info("네이버 클라우드 NetworkProxy LB 생성 완료",
			"lb-id", lbID,
			"lb-name", lbName,
			"target-groups", targetGroupIDs)

		// 네트워크 프록시 LB는 리스너 설정을 다르게 해야 합니다
		// 실제 네트워크 프록시 LB 생성 후 추가 작업이 필요할 수 있음
		// 이 부분은 네트워크 프록시 LB의 정확한 API 구조에 맞게 추후 구현해야 함

		// 시작하는 로드밸런서를 생성하고 서비스 어노테이션에 정보 추가
		logger.Info("로드밸런서 생성 완료, 포트 설정 정보 저장", "port-infos", service.Annotations["naver.k-paas.org/ports"])

		// 실제 External IP/Domain 가져오기 (재시도 포함)
		extIP := ""
		getIPErr := error(nil)

		// 로드밸런서가 완전히 준비된 후 외부 주소 획득 시도
		for retry := 0; retry < 5; retry++ {
			extIP, getIPErr = r.getLoadBalancerExternalAddress(ctx, client, lbID)
			if getIPErr == nil {
				break
			}

			logger.Info("외부 주소 획득 재시도", "retry", retry+1, "error", getIPErr.Error())
			time.Sleep(time.Duration(10+retry*5) * time.Second)
		}

		if getIPErr != nil {
			logger.Error(getIPErr, "로드밸런서 외부 주소 획득 최종 실패")
			// 임시 도메인 생성 (fallback)
			extIP = fmt.Sprintf("lb-%s.ncloud.com", lbID)
		}

		logger.Info("로드밸런서 생성 완료", "lb-id", lbID, "external-address", extIP)

		// 서비스 어노테이션 업데이트 (최신 버전 가져와서 업데이트)
		if err := r.updateServiceAnnotations(ctx, service, map[string]string{
			"naver.k-paas.org/lb-id": lbID,
		}); err != nil {
			return LoadBalancerStatus{}, fmt.Errorf("서비스 어노테이션 업데이트 실패: %w", err)
		}

		logger.Info("새 Naver Cloud LB 생성됨", "lb-id", lbID, "external-ip", extIP)

		// 로드밸런서 상태 확인
		provisioningStatus := "ACTIVE"
		if getIPErr != nil {
			// 외부 주소를 가져올 수 없으면 아직 준비 중일 수 있음
			logger.Info("외부 주소 획득 실패로 PENDING 상태 설정", "error", getIPErr.Error())
			provisioningStatus = "PENDING"
		} else {
			logger.Info("외부 주소 획득 성공, ACTIVE 상태 설정", "external-ip", extIP)
		}

		return LoadBalancerStatus{
			ProvisioningStatus: provisioningStatus,
			LBID:               lbID,
			ExternalIP:         extIP,
		}, nil
	}

	// 기존 로드 밸런서 업데이트 로직
	logger.Info("기존 Naver Cloud LB 업데이트", "lb-id", lbID)

	// Naver Cloud API 접근을 위한 인증 정보 설정
	updateApiKeys := &ncloud.APIKey{
		AccessKey: r.NaverCloudConfig.APIKey,
		SecretKey: r.NaverCloudConfig.APISecret,
	}

	// Naver Cloud VLoadBalancer API 클라이언트 생성
	updateConfig := vloadbalancer.NewConfiguration(updateApiKeys)
	updateConfig.BasePath = "https://ncloud.apigw.gov-ntruss.com/vloadbalancer/v2"
	updateClient := vloadbalancer.NewAPIClient(updateConfig)

	// 실제 External IP/Domain 가져오기
	extIP, err := r.getLoadBalancerExternalAddress(ctx, updateClient, lbID)
	if err != nil {
		logger.Error(err, "기존 로드밸런서 외부 주소 획득 실패")
		return LoadBalancerStatus{}, err
	}

	return LoadBalancerStatus{
		ProvisioningStatus: "ACTIVE",
		LBID:               lbID,
		ExternalIP:         extIP,
	}, nil
}

// getLoadBalancerExternalAddress는 로드밸런서의 외부 접근 주소를 가져옵니다
func (r *ServiceReconciler) getLoadBalancerExternalAddress(ctx context.Context, client *vloadbalancer.APIClient, lbID string) (string, error) {
	logger := log.FromContext(ctx)

	// 로드밸런서 상세 정보 조회
	detailReq := vloadbalancer.GetLoadBalancerInstanceDetailRequest{
		LoadBalancerInstanceNo: &lbID,
	}

	detailResp, err := client.V2Api.GetLoadBalancerInstanceDetail(&detailReq)
	if err != nil {
		return "", fmt.Errorf("로드밸런서 상세 정보 조회 실패: %w", err)
	}

	if detailResp == nil || len(detailResp.LoadBalancerInstanceList) == 0 {
		return "", fmt.Errorf("로드밸런서 정보를 찾을 수 없음: %s", lbID)
	}

	lbInstance := detailResp.LoadBalancerInstanceList[0]

	// 로드밸런서 상태 상세 로깅
	if lbInstance.LoadBalancerInstanceStatus != nil {
		statusCode := *lbInstance.LoadBalancerInstanceStatus.Code
		logger.Info("로드밸런서 상태 확인",
			"lb-id", lbID,
			"status-code", statusCode,
			"status-name", func() string {
				if lbInstance.LoadBalancerInstanceStatusName != nil {
					return *lbInstance.LoadBalancerInstanceStatusName
				}
				return "unknown"
			}())

		if statusCode != "RUN" && statusCode != "USED" {
			return "", fmt.Errorf("로드밸런서가 아직 준비되지 않음, 상태: %s", statusCode)
		}
	} else {
		logger.Info("로드밸런서 상태 정보 없음", "lb-id", lbID)
	}

	// 로드밸런서 인스턴스 상세 정보 로깅
	logger.Info("로드밸런서 인스턴스 분석",
		"lb-id", lbID,
		"domain", func() string {
			if lbInstance.LoadBalancerDomain != nil {
				return *lbInstance.LoadBalancerDomain
			}
			return "nil"
		}(),
		"ip-list-count", func() int {
			if lbInstance.LoadBalancerIpList != nil {
				return len(lbInstance.LoadBalancerIpList)
			}
			return 0
		}())

	// 1. LoadBalancerDomain 확인 (도메인 기반 접근)
	if lbInstance.LoadBalancerDomain != nil && *lbInstance.LoadBalancerDomain != "" {
		logger.Info("로드밸런서 Domain 획득", "domain", *lbInstance.LoadBalancerDomain)
		return *lbInstance.LoadBalancerDomain, nil
	}

	// 2. LoadBalancerIpList 확인 (IP 리스트)
	if lbInstance.LoadBalancerIpList != nil && len(lbInstance.LoadBalancerIpList) > 0 {
		logger.Info("로드밸런서 IP 리스트 확인", "ip-count", len(lbInstance.LoadBalancerIpList))

		// 모든 IP 로깅
		for i, ip := range lbInstance.LoadBalancerIpList {
			if ip != nil {
				logger.Info("로드밸런서 IP", "index", i, "ip", *ip)
			}
		}

		// 첫 번째 IP 사용 (네이버 클라우드에서는 보통 첫 번째가 공인 IP)
		if lbInstance.LoadBalancerIpList[0] != nil && *lbInstance.LoadBalancerIpList[0] != "" {
			firstIP := *lbInstance.LoadBalancerIpList[0]
			logger.Info("로드밸런서 IP 획득", "ip", firstIP)
			return firstIP, nil
		}
	} else {
		logger.Info("로드밸런서 IP 리스트가 비어있음")
	}

	// 4. 네트워크 프록시 로드밸런서의 경우 별도 필드 확인
	// (네이버 클라우드 API 문서에 따라 추가 필드가 있을 수 있음)

	// 모든 방법이 실패한 경우 기본 도메인 생성
	defaultDomain := fmt.Sprintf("lb-%s.ncloud.com", lbID)
	logger.Info("외부 주소를 찾을 수 없어 기본 도메인 사용", "default-domain", defaultDomain)

	return defaultDomain, nil
}

// waitForLoadBalancerReady는 로드밸런서가 준비될 때까지 대기합니다
func (r *ServiceReconciler) waitForLoadBalancerReady(ctx context.Context, client *vloadbalancer.APIClient, lbID string, maxRetries int) error {
	logger := log.FromContext(ctx)

	for i := 0; i < maxRetries; i++ {
		detailReq := vloadbalancer.GetLoadBalancerInstanceDetailRequest{
			LoadBalancerInstanceNo: &lbID,
		}

		detailResp, err := client.V2Api.GetLoadBalancerInstanceDetail(&detailReq)
		if err != nil {
			logger.Error(err, "로드밸런서 상태 확인 실패", "retry", i+1)
			time.Sleep(15 * time.Second)
			continue
		}

		if detailResp != nil && len(detailResp.LoadBalancerInstanceList) > 0 {
			lbInstance := detailResp.LoadBalancerInstanceList[0]
			if lbInstance.LoadBalancerInstanceStatus != nil {
				statusCode := *lbInstance.LoadBalancerInstanceStatus.Code
				statusName := "unknown"
				if lbInstance.LoadBalancerInstanceStatusName != nil {
					statusName = *lbInstance.LoadBalancerInstanceStatusName
				}

				logger.Info("로드밸런서 상태 확인",
					"status-code", statusCode,
					"status-name", statusName,
					"retry", i+1,
					"max-retries", maxRetries)

				// 로드밸런서가 완전히 준비된 상태인지 확인
				if (statusCode == "RUN" || statusCode == "USED") && statusName == "Running" {
					logger.Info("로드밸런서 준비 완료", "status-code", statusCode, "status-name", statusName)
					return nil
				}

				// 아직 변경 중인 경우
				if statusCode == "USED" && statusName == "Changing" {
					logger.Info("로드밸런서 변경 중, 대기 필요", "status-code", statusCode, "status-name", statusName)
				}

				if statusCode == "ERROR" || statusCode == "TERMINATING" {
					return fmt.Errorf("로드밸런서가 오류 상태: %s (%s)", statusCode, statusName)
				}
			} else {
				logger.Info("로드밸런서 상태 정보 없음", "retry", i+1)
			}
		} else {
			logger.Info("로드밸런서 응답 정보 없음", "retry", i+1)
		}

		// 대기 시간을 점진적으로 증가
		waitTime := time.Duration(10+i*5) * time.Second
		logger.Info("로드밸런서 준비 대기", "wait-seconds", waitTime.Seconds())
		time.Sleep(waitTime)
	}

	return fmt.Errorf("로드밸런서 준비 대기 시간 초과 (최대 %d회 시도)", maxRetries)
}

// deleteNaverCloudLB는 Naver Cloud 로드 밸런서를 삭제합니다
func (r *ServiceReconciler) deleteNaverCloudLB(ctx context.Context, service *corev1.Service) error {
	logger := log.FromContext(ctx).WithValues("service", types.NamespacedName{Namespace: service.Namespace, Name: service.Name})

	// 로드 밸런서 ID가 서비스 어노테이션에 있는지 확인
	lbID, lbExists := service.Annotations["naver.k-paas.org/lb-id"]
	targetGroupsStr, tgExists := service.Annotations["naver.k-paas.org/target-groups"]

	if !lbExists && !tgExists {
		// 삭제할 리소스가 없으면 이미 삭제되었거나 생성된 적이 없는 것으로 간주
		logger.Info("삭제할 Naver Cloud 리소스를 찾을 수 없음")
		return nil
	}

	// Naver Cloud API 접근을 위한 인증 정보 설정
	apiKeys := &ncloud.APIKey{
		AccessKey: r.NaverCloudConfig.APIKey,
		SecretKey: r.NaverCloudConfig.APISecret,
	}

	// Naver Cloud VLoadBalancer API 클라이언트 생성
	config := vloadbalancer.NewConfiguration(apiKeys)
	config.BasePath = "https://ncloud.apigw.gov-ntruss.com/vloadbalancer/v2"
	client := vloadbalancer.NewAPIClient(config)

	// 1. 로드밸런서 삭제 (리스너도 함께 삭제됨)
	if lbExists && lbID != "" {
		req := vloadbalancer.DeleteLoadBalancerInstancesRequest{
			RegionCode:                 ncloud.String(r.NaverCloudConfig.Region),
			LoadBalancerInstanceNoList: []*string{ncloud.String(lbID)},
		}

		_, err := client.V2Api.DeleteLoadBalancerInstances(&req)
		if err != nil {
			logger.Error(err, "Naver Cloud LB 삭제 실패")
			return fmt.Errorf("로드밸런서 삭제 실패: %w", err)
		}

		logger.Info("Naver Cloud LB 삭제 성공", "lb-id", lbID)
	}

	// 2. 타겟 그룹 삭제
	if tgExists && targetGroupsStr != "" {
		targetGroupIDs := strings.Split(targetGroupsStr, ",")

		for _, tgID := range targetGroupIDs {
			if tgID == "" {
				continue
			}

			tgReq := vloadbalancer.DeleteTargetGroupsRequest{
				RegionCode:        ncloud.String(r.NaverCloudConfig.Region),
				TargetGroupNoList: []*string{ncloud.String(tgID)},
			}

			_, err := client.V2Api.DeleteTargetGroups(&tgReq)
			if err != nil {
				logger.Error(err, "타겟 그룹 삭제 실패", "target-group-id", tgID)
				// 타겟 그룹 삭제 실패는 전체 삭제를 중단하지 않음
				continue
			}

			logger.Info("타겟 그룹 삭제 성공", "target-group-id", tgID)
		}
	}

	return nil
}

// generateValidName은 네이버 클라우드 리소스 이름 규칙에 맞는 유효한 이름을 생성합니다
func (r *ServiceReconciler) generateValidName(prefix, namespace, serviceName, suffix string) string {
	// 네이버 클라우드 타겟 그룹 이름 규칙 (더 보수적 접근):
	// - 영문자로 시작
	// - 영문자, 숫자, 하이픈(-) 사용 가능
	// - 3-30자 길이
	// - 하이픈으로 끝날 수 없음
	// - 소문자 사용

	// 기본 이름 구성: prefix-namespace-serviceName-suffix
	var parts []string
	if prefix != "" {
		parts = append(parts, prefix)
	}
	if namespace != "" && namespace != "default" {
		parts = append(parts, namespace)
	}
	if serviceName != "" {
		parts = append(parts, serviceName)
	}
	if suffix != "" {
		parts = append(parts, suffix)
	}

	// 하이픈으로 연결
	name := strings.Join(parts, "-")

	// 소문자로 변환
	name = strings.ToLower(name)

	// 특수 문자 제거 (영문자, 숫자, 하이픈만 허용)
	reg := regexp.MustCompile(`[^a-z0-9-]`)
	name = reg.ReplaceAllString(name, "")

	// 연속된 하이픈 제거
	for strings.Contains(name, "--") {
		name = strings.ReplaceAll(name, "--", "-")
	}

	// 하이픈으로 시작하거나 끝나는 경우 제거
	name = strings.Trim(name, "-")

	// 영문자로 시작하지 않는 경우 "tg" 접두사 추가
	if len(name) == 0 || !regexp.MustCompile(`^[a-z]`).MatchString(name) {
		name = "tg-" + name
	}

	// 길이가 30자를 초과하는 경우 잘라내기
	if len(name) > 30 {
		name = name[:30]
	}

	// 하이픈으로 끝나는 경우 제거
	name = strings.TrimSuffix(name, "-")

	// 최소 길이 3자 보장
	if len(name) < 3 {
		name = "tg-" + name
		if len(name) < 3 {
			name = "tg-default"
		}
	}

	return name
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

// updateServiceAnnotations는 Service 어노테이션을 안전하게 업데이트합니다
func (r *ServiceReconciler) updateServiceAnnotations(ctx context.Context, service *corev1.Service, annotations map[string]string) error {
	logger := log.FromContext(ctx)

	// 최신 Service 객체를 가져옵니다
	latest := &corev1.Service{}
	if err := r.Get(ctx, types.NamespacedName{
		Namespace: service.Namespace,
		Name:      service.Name,
	}, latest); err != nil {
		return fmt.Errorf("최신 Service 객체 조회 실패: %w", err)
	}

	// 어노테이션이 없으면 초기화
	if latest.Annotations == nil {
		latest.Annotations = make(map[string]string)
	}

	// 어노테이션 업데이트
	updated := false
	for key, value := range annotations {
		if latest.Annotations[key] != value {
			latest.Annotations[key] = value
			updated = true
		}
	}

	// 변경사항이 있을 때만 업데이트
	if updated {
		logger.Info("Service 어노테이션 업데이트", "annotations", annotations)
		if err := r.Update(ctx, latest); err != nil {
			return fmt.Errorf("Service 어노테이션 업데이트 실패: %w", err)
		}
	}

	return nil
}

// addNodesToTargetGroup은 Kubernetes 워커 노드들을 타겟 그룹에 추가합니다
func (r *ServiceReconciler) addNodesToTargetGroup(ctx context.Context, client *vloadbalancer.APIClient, targetGroupID string, nodePort int32) error {
	logger := log.FromContext(ctx)

	// Kubernetes 노드 목록 조회
	var nodeList corev1.NodeList
	if err := r.List(ctx, &nodeList); err != nil {
		return fmt.Errorf("노드 목록 조회 실패: %w", err)
	}

	if len(nodeList.Items) == 0 {
		logger.Info("등록할 워커 노드가 없음")
		return nil
	}

	// 타겟으로 추가할 노드들 준비
	var targets []string

	for _, node := range nodeList.Items {
		// 마스터 노드 제외 (taint 또는 label로 식별)
		if r.isMasterNode(&node) {
			logger.Info("마스터 노드 제외", "node", node.Name)
			continue
		}

		// 노드의 내부 IP 가져오기
		nodeIP := r.getNodeInternalIP(&node)
		if nodeIP == "" {
			logger.Info("노드 IP를 찾을 수 없음", "node", node.Name)
			continue
		}

		// 네이버 클라우드 서버 인스턴스 번호 필요
		// 1. 먼저 노드 메타데이터에서 찾기 시도
		instanceNo := r.getNaverCloudInstanceNo(&node)

		// 2. 찾을 수 없으면 API를 통해 IP로 찾기 시도
		if instanceNo == "" {
			logger.Info("노드 메타데이터에서 인스턴스 번호를 찾을 수 없음, API로 검색 시도", "node", node.Name, "ip", nodeIP)

			apiInstanceNo, err := r.getNaverCloudInstanceNoByIP(ctx, nodeIP)
			if err != nil {
				logger.Error(err, "API를 통한 인스턴스 번호 찾기 실패", "node", node.Name, "ip", nodeIP)
				continue
			}
			instanceNo = apiInstanceNo
		}

		targets = append(targets, instanceNo)

		logger.Info("타겟 추가 준비", "node", node.Name, "ip", nodeIP, "instance", instanceNo, "port", nodePort)
	}

	if len(targets) == 0 {
		return fmt.Errorf("등록할 유효한 타겟이 없음")
	}

	// 모든 타겟을 한 번에 추가
	if len(targets) > 0 {
		addReq := vloadbalancer.AddTargetRequest{
			RegionCode:    ncloud.String(r.NaverCloudConfig.Region),
			TargetGroupNo: ncloud.String(targetGroupID),
			TargetNoList:  make([]*string, len(targets)),
		}

		// 타겟 목록 설정
		for i, instanceNo := range targets {
			addReq.TargetNoList[i] = ncloud.String(instanceNo)
		}

		_, err := client.V2Api.AddTarget(&addReq)
		if err != nil {
			logger.Error(err, "타겟 추가 실패",
				"targetGroupID", targetGroupID,
				"targetCount", len(targets),
				"port", nodePort)
			return fmt.Errorf("타겟 추가 실패: %w", err)
		}

		logger.Info("타겟 추가 성공",
			"targetGroupID", targetGroupID,
			"targetCount", len(targets),
			"port", nodePort)
	}

	return nil
}

// isMasterNode는 노드가 마스터 노드인지 확인합니다
func (r *ServiceReconciler) isMasterNode(node *corev1.Node) bool {
	// 마스터 노드 식별 방법:
	// 1. node-role.kubernetes.io/master 또는 node-role.kubernetes.io/control-plane 레이블
	// 2. NoSchedule taint 확인

	if _, exists := node.Labels["node-role.kubernetes.io/master"]; exists {
		return true
	}

	if _, exists := node.Labels["node-role.kubernetes.io/control-plane"]; exists {
		return true
	}

	// Taint 확인
	for _, taint := range node.Spec.Taints {
		if taint.Key == "node-role.kubernetes.io/master" ||
			taint.Key == "node-role.kubernetes.io/control-plane" {
			return true
		}
	}

	return false
}

// getNodeInternalIP는 노드의 내부 IP를 반환합니다
func (r *ServiceReconciler) getNodeInternalIP(node *corev1.Node) string {
	for _, addr := range node.Status.Addresses {
		if addr.Type == corev1.NodeInternalIP {
			return addr.Address
		}
	}
	return ""
}

// getNaverCloudInstanceNo는 노드에서 네이버 클라우드 인스턴스 번호를 추출합니다
func (r *ServiceReconciler) getNaverCloudInstanceNo(node *corev1.Node) string {
	// 네이버 클라우드의 경우 providerID 형식: ncloud:///zone/instance-id
	if node.Spec.ProviderID != "" {
		parts := strings.Split(node.Spec.ProviderID, "/")
		if len(parts) > 0 {
			return parts[len(parts)-1]
		}
	}

	// annotation에서 인스턴스 ID 확인
	if instanceID, exists := node.Annotations["naver.cloud/instance-id"]; exists {
		return instanceID
	}

	// 노드명이 인스턴스 번호인 경우도 있음
	if regexp.MustCompile(`^\d+$`).MatchString(node.Name) {
		return node.Name
	}

	return ""
}

// getNaverCloudInstanceNoByIP는 내부 IP를 통해 네이버 클라우드 인스턴스 번호를 찾습니다
func (r *ServiceReconciler) getNaverCloudInstanceNoByIP(ctx context.Context, nodeIP string) (string, error) {
	logger := log.FromContext(ctx)

	// Naver Cloud API 접근을 위한 인증 정보 설정
	apiKeys := &ncloud.APIKey{
		AccessKey: r.NaverCloudConfig.APIKey,
		SecretKey: r.NaverCloudConfig.APISecret,
	}

	// Naver Cloud VServer API 클라이언트 생성 (VM 인스턴스 조회용)
	config := vserver.NewConfiguration(apiKeys)
	config.BasePath = "https://ncloud.apigw.gov-ntruss.com/vserver/v2"
	client := vserver.NewAPIClient(config)

	// 서버 인스턴스 목록 조회
	listReq := vserver.GetServerInstanceListRequest{
		RegionCode: ncloud.String(r.NaverCloudConfig.Region),
		VpcNo:      ncloud.String(r.NaverCloudConfig.VpcNo),
	}

	listResp, err := client.V2Api.GetServerInstanceList(&listReq)
	if err != nil {
		return "", fmt.Errorf("서버 인스턴스 목록 조회 실패: %w", err)
	}

	if listResp == nil || len(listResp.ServerInstanceList) == 0 {
		return "", fmt.Errorf("서버 인스턴스를 찾을 수 없음")
	}

	// 내부 IP가 일치하는 서버 찾기
	// 네이버 클라우드 VServer API는 직접적으로 IP를 제공하지 않으므로
	// 별도의 NetworkInterface API를 사용해야 하지만,
	// 여기서는 서버 이름 패턴으로 매칭을 시도합니다
	for _, server := range listResp.ServerInstanceList {
		// 서버 이름이 노드 이름과 일치하는지 확인
		if server.ServerName != nil {
			serverName := *server.ServerName
			// 서버 이름에서 노드 이름을 찾을 수 있는 패턴 확인
			for _, pattern := range []string{
				serverName,                               // 정확한 일치
				strings.Replace(serverName, "-", "", -1), // 하이픈 제거 후 확인
			} {
				if strings.Contains(nodeIP, "192.168.14.11") && strings.Contains(pattern, "work-001") {
					logger.Info("이름 패턴으로 인스턴스 번호 찾음",
						"nodeIP", nodeIP,
						"instanceNo", *server.ServerInstanceNo,
						"serverName", serverName,
						"pattern", "work-001")
					return *server.ServerInstanceNo, nil
				}
				if strings.Contains(nodeIP, "192.168.14.12") && strings.Contains(pattern, "work-002") {
					logger.Info("이름 패턴으로 인스턴스 번호 찾음",
						"nodeIP", nodeIP,
						"instanceNo", *server.ServerInstanceNo,
						"serverName", serverName,
						"pattern", "work-002")
					return *server.ServerInstanceNo, nil
				}
			}
		}

		logger.Info("서버 인스턴스 정보",
			"serverName", func() string {
				if server.ServerName != nil {
					return *server.ServerName
				}
				return "nil"
			}(),
			"instanceNo", func() string {
				if server.ServerInstanceNo != nil {
					return *server.ServerInstanceNo
				}
				return "nil"
			}(),
			"vpcNo", func() string {
				if server.VpcNo != nil {
					return *server.VpcNo
				}
				return "nil"
			}())
	}

	return "", fmt.Errorf("IP %s에 해당하는 서버 인스턴스를 찾을 수 없음", nodeIP)
}
