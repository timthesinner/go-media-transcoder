//By TimTheSinner
package main

import (
	"regexp"
	"strconv"
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

func groupsFromRegex(r *regexp.Regexp, search string) (paramsMap map[string]string) {
	match := r.FindStringSubmatch(search)
	if len(match) == 0 {
		return
	}

	paramsMap = make(map[string]string)
	for i, name := range r.SubexpNames() {
		if i != 0 {
			paramsMap[name] = match[i]
		}
	}
	return
}

var md5Regex = regexp.MustCompile(`(?mi):\s*(?P<md5>([a-f0-9]{2} )*[a-f0-9]{2})\s*CertUtil:`)

//var md5Regex = regexp.MustCompile(`.* = (?P<md5>[a-z0-9]+)`)

func md5FromFile(file string) string {
	return strings.Replace(groupsFromRegex(md5Regex, runCommandOutput("CertUtil", "-hashfile", file, "MD5"))["md5"], " ", "", -1)
	//return groupsFromRegex(md5Regex, runCommandOutput("md5", file))["md5"]
}

func FilterEnglishStreams(streams []interface{}) []string {
	english := make([]string, 0)
	for _, _stream := range streams {
		stream, ok := _stream.(map[string]interface{})
		if !ok {
			return english
		}

		if stream["codec_type"] == "audio" || stream["codec_type"] == "subtitle" {
			tags, ok := stream["tags"].(map[string]interface{})
			if !ok {
				continue
			}

			if language, ok := tags["language"]; ok && language == "eng" {
				english = append(english, "-map", "0:"+strconv.Itoa(int(stream["index"].(float64))))
			}
		}
	}

	return english
}

func HasAttachmentStreams(streams []interface{}) bool {
	for _, _stream := range streams {
		stream, ok := _stream.(map[string]interface{})
		if !ok {
			return false
		}

		if stream["codec_type"] == "attachment" {
			return true
		}
	}

	return false
}
