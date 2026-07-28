// Copyright 2026 Upbound Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package crdschema

// IsOfType accepts a list of oasdiff StringsDiff changes and checks
// if the changing type is the specified type t.
func IsOfType(changes []string, t string) bool {
	if changes == nil {
		return false
	}
	return len(changes) == 1 && changes[0] == t
}
