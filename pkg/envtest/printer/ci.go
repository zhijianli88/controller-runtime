/*
Copyright 2020 The Kubernetes Authors.

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

package printer

import (
	"os"

	"github.com/onsi/ginkgo"
)

// hasEnv checks that the given env vars are set to *something*
func hasEnv(vars ...string) bool {
	for _, envVar := range vars {
		if os.Getenv(envVar) == "" {
			return false
		}
	}
	return true
}

// CIReporters returns a set of reporters appropriate for the current test
// environment.  For local tests, this is just the normal reporter.  For CI
// tests, this also potentially turns on Prow-styled JUnit output (if running
// in Prow) or remote JSON output (if running in GitHub Actions or a similar
// environment).
//
// In particular:
// - Prow = $CI and $ARTIFACTS and ($JUNIT_OUTPUT or $PROW_JOB_ID)
// - Actions/JSON = $CI and $REMOTE_TEST_OUT_PORT
func CIReporters(suiteName string) []ginkgo.Reporter {
	reporters := []ginkgo.Reporter{NewlineReporter{}}
	onCI := hasEnv("CI")
	if !onCI {
		return reporters
	}

	// prow wants JUnit, but give another way to turn this on manually
	if wantJUnit := hasEnv("JUNIT_OUTPUT") || hasEnv("PROW_JOB_ID"); hasEnv("ARTIFACTS") && wantJUnit {
		reporters = append(reporters, NewProwReporter(suiteName))
	}

	
	if remoteAddr := os.Getenv("REMOTE_TEST_OUT_ADDR"); remoteAddr != "" {
		reporters = append(reporters, &RemoteJSONReporter{Addr: remoteAddr})
	}

	return reporters
}
