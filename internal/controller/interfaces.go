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
	"github.com/suslmk-lee/kube-controller01/internal/navercloud"
)

// NaverCloudClient는 하위 호환성을 위한 타입 별칭입니다.
// 실제 구현은 internal/navercloud 패키지에 있습니다.
type NaverCloudClient = navercloud.Client
