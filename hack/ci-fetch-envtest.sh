#!/usr/bin/env bash

#  Copyright 2018 The Kubernetes Authors.
#
#  Licensed under the Apache License, Version 2.0 (the "License");
#  you may not use this file except in compliance with the License.
#  You may obtain a copy of the License at
#
#      http://www.apache.org/licenses/LICENSE-2.0
#
#  Unless required by applicable law or agreed to in writing, software
#  distributed under the License is distributed on an "AS IS" BASIS,
#  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
#  See the License for the specific language governing permissions and
#  limitations under the License.

set -e

export TRACE=1

set -o errexit
set -o nounset
set -o pipefail

hack_dir=$(dirname ${BASH_SOURCE})
source ${hack_dir}/common.sh
source ${hack_dir}/setup-envtest.sh

ENVTEST_K8S_VERSION=${KUBE_VER?must set KUBE_VER (env.kube-ver in workflow YAML)}

fetch_envtest_tools "${ENVTEST_UTILS_PATH?must set ENVTEST_UTILS_PATH (env.envtest-utils-path)}"
# NB(directxman12): I think this can't just be a symbolic link because these
# might be mounted at different paths when in the docker container.
cp -r "${ENVTEST_UTILS_PATH}"/* "${hack_dir}/../pkg/internal/testing/integration/assets"
