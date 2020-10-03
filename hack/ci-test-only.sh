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

# TODO: are these on by default on the golang docker image?
export PATH=$(go env GOPATH)/bin:$PATH
mkdir -p $(go env GOPATH)/bin

set -o errexit
set -o nounset
set -o pipefail

hack_dir=$(dirname ${BASH_SOURCE})
source ${hack_dir}/common.sh
source ${hack_dir}/setup-envtest.sh

ENVTEST_K8S_VERSION=${KUBE_VER?must set KUBE_VER (env.kube-ver in workflow YAML)}

${hack_dir}/ci-fetch-envtest.sh
setup_envtest_env "${ENVTEST_UTILS_PATH}"

# success or failure is determined by the next step, always exit success
set +o pipefail
( ${hack_dir}/test-all.sh ) || exit 0
set -o pipefail
