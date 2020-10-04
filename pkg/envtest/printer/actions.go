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
	"fmt"
	"os"
	"net/http"
	"encoding/json"
	"bytes"
	"time"

	"github.com/onsi/ginkgo/config"
	"github.com/onsi/ginkgo/types"

)

// RemoteJSONReport sends JSON status reports to http://localhost:<Port>/report-suite.
// Requests are POSTed in the message format described by SuiteMessage.
//
// Updates are sent at most 1 per second, to avoid overloading the server with
// quick tests, but sending start-/end-suite messages are not limited.
type RemoteJSONReporter struct {
	Addr string

	suite Suite
	nextSpecs []PartStatus
	lastUpdate time.Time
}

func (r *RemoteJSONReporter) req(action SuiteAction, reason string) {
	if err := r.reqInt(action); err != nil {
		fmt.Fprintf(os.Stderr, "unable to send %s for suite %q from %s: %v", action, r.suite.Name, reason, err)
	}
}

func (r *RemoteJSONReporter) reqInt(action SuiteAction) error {
	if action == ActionSuiteUpdate && time.Now().Sub(r.lastUpdate) < updateThreshold {
		return nil
	}

	msg := SuiteMessage{Action: action, Suite: &r.suite}
	msgJSON, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("unable to marshal %s request: %w", action, err)
	}

	url := fmt.Sprintf("http://%s/report-suite", r.Addr)
	resp, err := http.Post(url, "application/json", bytes.NewReader(msgJSON))
	if err != nil {
		return fmt.Errorf("unable to post %s request: %w", action, err)
	}

	switch resp.StatusCode {
	case http.StatusOK, http.StatusCreated, http.StatusAccepted:
		r.lastUpdate = time.Now()
		r.suite.MoreTestCases = r.suite.MoreTestCases[:0]
		return nil
	default:
		return fmt.Errorf("server did not accept %s request: %q", action, resp.Status)
	}
}

func (r *RemoteJSONReporter) SpecSuiteWillBegin(config config.GinkgoConfigType, summary *types.SuiteSummary) {
	r.suite.Name = summary.SuiteDescription
	r.suite.ID = summary.SuiteID

	r.req(ActionSuiteStart, "Suite (start)")
}
func (r *RemoteJSONReporter) SpecWillRun(specSummary *types.SpecSummary) {
	// No-op
}

func (r *RemoteJSONReporter) BeforeSuiteDidRun(sum *types.SetupSummary) {
	stat := PartStatus{
		Components: []StatusComponent{
			{Location: locToLoc(sum.CodeLocation)},
		},
		RunTime: sum.RunTime,
		Output: sum.CapturedOutput,
	}.withState(sum.State, sum.Failure)
	r.suite.BeforeSuite = &stat
	r.req(ActionSuiteUpdate, "BeforeSuite")
}
func (r *RemoteJSONReporter) SpecDidComplete(sum *types.SpecSummary) {
	r.suite.MoreTestCases = append(r.suite.MoreTestCases, PartStatus{
		Components: summaryToComponents(sum.ComponentTexts, sum.ComponentCodeLocations),
		RunTime: sum.RunTime,
		Output: sum.CapturedOutput,
	}.withState(sum.State, sum.Failure))

	r.req(ActionSuiteUpdate, "Spec")
}
func (r *RemoteJSONReporter) AfterSuiteDidRun(sum *types.SetupSummary) {
	stat := PartStatus{
		Components: []StatusComponent{
			{Location: locToLoc(sum.CodeLocation)},
		},
		RunTime: sum.RunTime,
		Output: sum.CapturedOutput,
	}.withState(sum.State, sum.Failure)
	r.suite.BeforeSuite = &stat
	r.req(ActionSuiteUpdate, "AfterSuite")
}
func (r *RemoteJSONReporter) SpecSuiteDidEnd(sum *types.SuiteSummary) {
	r.suite.RunTime = sum.RunTime
	r.suite.Stats = &SuiteStats{
		Total:            sum.NumberOfTotalSpecs,
		Pending:          sum.NumberOfPendingSpecs,
		Skipped:          sum.NumberOfSkippedSpecs,
		Passed:           sum.NumberOfPassedSpecs,
		Failed:           sum.NumberOfFailedSpecs,
		Flakes:           sum.NumberOfFlakedSpecs,
	}
	r.req(ActionSuiteEnd, "Suite (end)")
}

// summaryToComponents bundles ComponentTexts & ComponentCodeLocations to
// the equivalent serializable forms.
func summaryToComponents(texts []string, locs []types.CodeLocation) []StatusComponent {
	res := make([]StatusComponent, len(texts))
	for i, txt := range texts {
		res[i].Text = txt
		res[i].Location = locToLoc(locs[i])
	}
	return res
}

// locToLoc converts CodeLocations to the equivalent serializable form.
func locToLoc(loc types.CodeLocation) Location {
	return Location{
		File: loc.FileName,
		Line: loc.LineNumber,
		Stack: loc.FullStackTrace,
	}
}

// SuiteAction represents the type of action that this message represents
// (beginning, end, or intermediate test result updates).
type SuiteAction string
const (
	// ActionSuiteStart is sent at the beginning of a suite, before anything
	// has run.
	ActionSuiteStart  SuiteAction = "suite-start"
	// ActionSuiteEnd is sent at the end of a suite, after it completes.
	ActionSuiteEnd    SuiteAction = "suite-end"
	// ActionSuiteUpdate is sent when new specs complete, beforesuite or
	// aftersuite is run, etc.
	ActionSuiteUpdate SuiteAction = "suite-update"
)

// SpecState represents the outcome of running a spec (test case).
type SpecState string 
const (
	SpecStatePassed   SpecState = "passed"
	SpecStateSkipped  SpecState = "skipped"
	SpecStateFailed   SpecState = "failed"
	SpecStatePanicked SpecState = "panicked"
	SpecStateTimedOut SpecState = "timed-out"
	SpecStatePending  SpecState = "pending"
)

// ComponentType represents a single "container" in the path
// of a spec.  The types generally correspond to the their name,
// except "container", which is a Describe or Context.
type ComponentType string
const (
	ComponentTypeContainer      ComponentType = "Container"
	ComponentTypeBeforeEach                   = "BeforeEach"
	ComponentTypeJustBeforeEach               = "AfterEach"
	ComponentTypeJustAfterEach                = "JustBeforeEach"
	ComponentTypeAfterEach                    = "JustAfterEach"
	ComponentTypeIt                           = "It"
	ComponentTypeOther                        = "Unknown"
)

const (
	updateThreshold = 1 * time.Second
)

// SuiteMessage is the envelope for the suite event calls.
type SuiteMessage struct {
	// Action is the type of event that this message represents.
	Action SuiteAction `json:"action"`

	// Suite contains the updates for a given suite.
	Suite *Suite `json:"suite,omitempty"`
}

// Suite represents an in-progress or complete test suite.
type Suite struct {
	// Name is the description of the test suite.
	Name string `json:"name"`
	// ID is a unique semi-random identifier for this suite.
	ID string `json:"id"`

	// RunTime is the amount of time spent running this suite.
	// Only set on "suite-end".
	RunTime time.Duration `json:"runTime"`

	// BeforeSuite is the result of running the BeforeSuite, if any.
	BeforeSuite *PartStatus `json:"beforeSuite,omitempty"`
	// AfterSuite is the result of running the AfterSuite, if any.
	AfterSuite *PartStatus `json:"beforeSuite,omitempty"`
	// MoreTestCases contains another chunk of completed testcases.
	// Any test cases present are new, should be appended to the list --
	// they will not be sent again if the response from the server
	// is OK, Created, or Accepted.
	MoreTestCases []PartStatus `json:"moreTestCases,omitempty"`

	// Stats are the final tallies of test cases run.
	// Only set on "suite-end".
	Stats *SuiteStats `json:"stats,omitempty"`
}

// SuiteStats contains the final tallies of test cases run, broken down by
// outcome.
type SuiteStats struct {
	Total int `json:"total"`
	Pending int `json:"pending"`
	Skipped int `json:"skipped"`
	Passed int `json:"passed"`
	Failed int `json:"failed"`
	Flakes int `json:"flakes"`
}

// Location represents a source code location.
type Location struct {
	File string `json:"file"`
	Line int `json:"line"`
	Stack string `json:"stack,omitempty"`
}

// StatusComponent represents one "level"/grouping of tests
// (in ginkgo terminology, a Describe, Context, or It).
type StatusComponent struct {
	Text string `json:"text"`
	Location Location `json:"location"`
}

// PartStatus contains the outcome for a Spec/BeforeSuite/AfterSuite.
type PartStatus struct {
	// Componets uniquly identifies the "path" to this test case
	// (e.g. Describe --> Describe --> It).
	Components []StatusComponent `json:"components"`

	// State is the conclusion of the test case.
	State SpecState `json:"state"`
	// RunTime is the total time spend running the test case.
	RunTime time.Duration `json:"runTime"`
	// Failure provides details when the test case didn't succeed
	// (i.e. state isn't "succeeded" or "pending").
	Failure *FailureInfo `json:"failure,omitempty"`

	// Output contains the captured printed output during the test.
	Output string `json:"output,omitempty"`
}
func (p PartStatus) withState(state types.SpecState, failure types.SpecFailure) PartStatus {
	switch state {
	case types.SpecStatePending:
		p.State = SpecStatePending
	case types.SpecStateSkipped:
		p.State = SpecStateSkipped
	case types.SpecStatePassed:
		p.State = SpecStatePassed
	case types.SpecStateFailed:
		p.State = SpecStateFailed
	case types.SpecStatePanicked:
		p.State = SpecStatePanicked
	case types.SpecStateTimedOut:
		p.State = SpecStateTimedOut
	case types.SpecStateInvalid:
		fallthrough
	default:
		panic("encountered spec with invalid state")
	}

	if state != types.SpecStatePassed && state != types.SpecStatePending {
		var typ ComponentType
		switch failure.ComponentType {
		case types.SpecComponentTypeContainer:
			typ = ComponentTypeContainer
		case types.SpecComponentTypeBeforeEach:
			typ = ComponentTypeBeforeEach
		case types.SpecComponentTypeJustBeforeEach:
			typ = ComponentTypeJustBeforeEach
		case types.SpecComponentTypeJustAfterEach:
			typ = ComponentTypeJustAfterEach
		case types.SpecComponentTypeAfterEach:
			typ = ComponentTypeAfterEach
		case types.SpecComponentTypeIt:
			typ = ComponentTypeIt
		default:
			typ = ComponentTypeOther
		}
		p.Failure = &FailureInfo{
			Message: failure.Message,
			Location: locToLoc(failure.Location),
			Panic: failure.ForwardedPanic,

			Component: FailureComponent{
				Index: failure.ComponentIndex,
				Type: typ,
				Location: locToLoc(failure.ComponentCodeLocation),
			},
		}
	}

	return p
}

// FailureInfo describes a failed test case.
type FailureInfo struct {
	// Message is the printed failure message from the test case.
	Message string `json:"message"`
	// Location is the specific location where this failure occurred.
	Location Location `json:"location"`
	// Panic contains the contents of any capture panic that occurred
	// during the test, if one did.
	Panic string `json:"panic,omitempty"`

	// Component describes the component of the test that this failure
	// ocurred in.
	Component FailureComponent `json:"component"`
}

// FailureComponent describes the component in which a test case failure occurred
// (e.g. BeforeEach, It, etc).
type FailureComponent struct {
	// Type is the type of the component (e.g. BeforeSuite, It, etc).
	Type ComponentType `json:"type"`
	// Location is the location of this component.
	Location Location `json:"location"`
	// Index is... I don't know, something from Ginkgo.  It think the index
	// of this component in the list of components in the overall test case
	// that this occurred in.
	Index int `json:"index"`
}
