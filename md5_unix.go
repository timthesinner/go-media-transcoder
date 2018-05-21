// +build darwin dragonfly freebsd linux nacl netbsd openbsd solaris

//By TimTheSinner
package main

import (
	"regexp"
)

/**
 * Copyright (c) 2016 TimTheSinner All Rights Reserved.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

var md5Regex = regexp.MustCompile(`.* = (?P<md5>[a-z0-9]+)`)

func md5FromFile(file string) string {
	return groupsFromRegex(md5Regex, runCommandOutput("md5", file))["md5"]
}
