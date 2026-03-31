// Copyright 2022 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package log

import (
	"flag"

	"github.com/GoogleContainerTools/config-sync/pkg/version"
	"k8s.io/klog/v2"
)

// setFlag is a helper type that abstracts over the global flag.Set and
// a custom flag.FlagSet.Set so that ConfigureKlog can work with both.
type setFlag func(name, value string) error

// ConfigureKlog opts into the fixed stderrthreshold behavior introduced in
// klog v2.140.0 (kubernetes/klog#212). Call this after klog.InitFlags and
// before flag.Parse.
//
// If fs is nil the global flag.CommandLine is used; otherwise the provided
// FlagSet is used. This keeps the logic in one place regardless of whether
// the caller uses the global flags or a custom FlagSet.
func ConfigureKlog(fs *flag.FlagSet) {
	var set setFlag
	if fs != nil {
		set = fs.Set
	} else {
		set = flag.Set
	}
	if err := set("legacy_stderr_threshold_behavior", "false"); err != nil {
		klog.Fatalf("Failed to set flag %q: %v", "legacy_stderr_threshold_behavior", err)
	}
	if err := set("stderrthreshold", "INFO"); err != nil {
		klog.Fatalf("Failed to set flag %q: %v", "stderrthreshold", err)
	}
}

// Setup sets up default logging configs for Nomos applications and logs the preamble.
func Setup() {
	klog.InitFlags(nil)
	ConfigureKlog(nil)
	if err := flag.Set("logtostderr", "true"); err != nil {
		klog.Fatal(err)
	}
	flag.Parse()
	klog.Infof("Build Version: %s", version.VERSION)
}
