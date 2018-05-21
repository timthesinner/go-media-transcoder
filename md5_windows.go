//By TimTheSinner
package main

import (
	"regexp"
	"strings"
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

var md5Regex = regexp.MustCompile(`(?mi):\s*(?P<md5>([a-f0-9]{2} )*[a-f0-9]{2})\s*CertUtil:`)

func md5FromFile(file string) string {
	return strings.Replace(groupsFromRegex(md5Regex, runCommandOutput("CertUtil", "-hashfile", file, "MD5"))["md5"], " ", "", -1)
}
