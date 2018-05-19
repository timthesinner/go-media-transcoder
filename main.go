//By TimTheSinner
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path"
	"path/filepath"
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

type Transcode struct {
	Movie           string `json:"movie"`
	OriginalMovie   string `json:"originalFile"`
	OriginalCodec   string `json:"originalCodec"`
	OriginalWidth   int    `json:"originalWidth"`
	TranscodedMovie string `json:"transcodedFile"`
	TranscodedCodec string `json:"transcodedCodec"`
	TranscodedWidth int    `json:"transcodedWidth"`
	TranscodedHash  string `json:"transcodedHash"`
	TranscodedSize  int64  `json:"transcodedSize"`
	TranscodedSpeed string `json:"transcodedSpeed"`
	TranscodeCRF    int    `json:"transcodedCRF"`
}

func handle(err error) {
	if err != nil {
		log.Fatal(err)
	}
}

func transcodedMovie(originalMovie string) string {
	return filepath.Join(filepath.Dir(originalMovie), "transcode-"+filepath.Base(originalMovie))
}

func transcode(originalMovie string, hwaccel string, threads int, crf int, codec string) *Transcode {
	lock, err := NewLockfile(filepath.Join(filepath.Dir(originalMovie), "transcoding.lck"))
	if err != nil {
		fmt.Printf("Cannot init lock. reason: %v", err)
		return nil
	}
	defer lock.Unlock()

	streams := movieMetadata(originalMovie)["streams"].([]interface{})
	videoStream := streams[0].(map[string]interface{})

	scale := "scale=1920:-1"
	width, ok := videoStream["coded_width"].(float64)
	if !ok {
		return nil
	}

	if int(width) < 1920 {
		scale = "scale=" + strconv.Itoa(int(width)) + ":-1"
	}

	english := FilterEnglishStreams(streams)
	if len(english) == 0 {
		fmt.Println("Did not detect any english streams")
		return nil
	}
	targetMovie := transcodedMovie(originalMovie)
	transcodeArgs := []string{
		"-nostdin",
		"-hide_banner",
	}

	if strings.TrimSpace(hwaccel) != "" {
		transcodeArgs = append(transcodeArgs, "-hwaccel", hwaccel)
	}

	transcodeArgs = append(append(append(transcodeArgs,
		"-analyzeduration", "250M", "-probesize", "250M",
		"-i", originalMovie,
		"-map_metadata:g", "0:g",
		"-map_metadata:s:v", "0:s:v",
		//"-map_metadata:s:t?", "0:s:t", <-- This does not map metadata
		"-map", "v:0",
	), english...), []string{
		"-map", "d?",
		"-map", "t?",
		"-c:v", codec, "-vf", scale, "-crf", strconv.Itoa(crf), "-preset", *speed, "-tune", "fastdecode", "-movflags", "+faststart",
		"-c:a", "libopus", "-af", "aformat=channel_layouts='7.1|6.1|5.1|stereo'",
		"-c:s", "copy",
		"-metadata:s:a", "language=eng",
		"-metadata:s:s", "language=eng",
	}...)

	if threads > 0 {
		transcodeArgs = append(transcodeArgs, "-threads", strconv.Itoa(threads))
	}

	transcodeArgs = append(transcodeArgs, targetMovie)

	runCommand("rm", "-f", targetMovie)
	if !runCommand("ffmpeg", transcodeArgs...) {
		return nil
	}

	//rawMovie := "NOT-PRESERVED"
	// Preserve the original movie only if it is greater than 1920 (1080P)
	/*if int(videoStream["coded_width"].(float64)) > 1920 {
		rawMovie = originalMovie + "-orig"
		runCommand("mv", originalMovie, rawMovie)
	} else {
		runCommand("rm", "-f", originalMovie)
	}*/

	// Until we are comfortable with the settings always Preserve
	rawMovie := originalMovie + "-orig"
	runCommand("mv", originalMovie, rawMovie)

	// Move the transcoded movie over the original
	runCommand("mv", targetMovie, originalMovie)
	info, err := os.Stat(originalMovie)
	handle(err)

	transcodedStream := movieMetadata(originalMovie)["streams"].([]interface{})[0].(map[string]interface{})
	return &Transcode{
		OriginalMovie:   rawMovie,
		TranscodedMovie: originalMovie,
		OriginalCodec:   videoStream["codec_name"].(string),
		OriginalWidth:   int(videoStream["width"].(float64)),
		TranscodedCodec: transcodedStream["codec_name"].(string),
		TranscodedWidth: int(transcodedStream["width"].(float64)),
		TranscodedHash:  md5FromFile(originalMovie),
		TranscodedSize:  info.Size(),
		TranscodedSpeed: *speed,
		TranscodeCRF:    crf,
	}
}

func movieMetadata(movie string) (metadata map[string]interface{}) {
	rawMetadata := runCommandOutput("ffprobe", "-v", "quiet", "-print_format", "json", "-show_format", "-show_streams", movie)
	json.Unmarshal([]byte(rawMetadata), &metadata)
	return
}

func readMetadata(mediaDir string) (ret map[string]*Transcode) {
	mediaMeta, err := os.Open(path.Join(mediaDir, "transcode-metadata.json"))
	if err != nil {
		return make(map[string]*Transcode)
	}
	defer mediaMeta.Close()

	json.NewDecoder(mediaMeta).Decode(&ret)
	return
}

func writeMetadata(mediaDir string, meta *Transcode) map[string]*Transcode {
	mediaMetadata := readMetadata(mediaDir)
	mediaMetadata[meta.Movie] = meta

	mediaMeta, err := os.OpenFile(path.Join(mediaDir, "transcode-metadata.json"), os.O_WRONLY|os.O_CREATE, 0644)
	handle(err)
	defer mediaMeta.Close()

	json.NewEncoder(mediaMeta).Encode(mediaMetadata)
	return mediaMetadata
}

var PROCESS_FILE_EXTENSIONS = map[string]bool{
	".mkv": true,
	".mp4": true,

	// Do not process originals
	".mkv-orig": false,
	".mp4-orig": false,

	// Do not process lock files
	".lck": false,

	".srt":      false,
	".nfo":      false,
	".jpg":      false,
	".DS_Store": false,
	".nfo-orig": false,
}

const MIN_FILE_SIZE = 256 * 1024 * 1024

var hwaccel = flag.String("hwaccel", "", "Hardware Acceleration Driver")
var threads = flag.Int("threads", 0, "Number of threads")
var crf = flag.Int("crf", 22, "CRF (Quality Factor)")
var codec = flag.String("codec", "libx265", "Video encoding codec")
var speed = flag.String("speed", "slower", "Encoder speed")

func main() {
	flag.Parse()

	mediaDir := "/Volumes/downloads/movies/"
	if flag.NArg() > 0 {
		mediaDir = flag.Arg(0)
	}

	mediaMetadata := readMetadata(mediaDir)
	movies, err := ioutil.ReadDir(mediaDir)
	handle(err)

	for _, movieName := range movies {
		if !movieName.IsDir() {
			continue
		}

		movieDir := filepath.Join(mediaDir, movieName.Name())
		files, err := ioutil.ReadDir(movieDir)
		handle(err)

		for _, file := range files {
			if !file.IsDir() {
				movie := filepath.Join(movieDir, file.Name())
				fmt.Println(file.Name())
				if process, ok := PROCESS_FILE_EXTENSIONS[filepath.Ext(file.Name())]; !ok {
					fmt.Printf("UNKNOWN FILE TYPE %s\n", filepath.Ext(file.Name()))
				} else if strings.HasPrefix(file.Name(), "transcode-") {
					continue
				} else if process && file.Size() > MIN_FILE_SIZE {
					if meta, ok := mediaMetadata[movieName.Name()]; !ok || (meta.TranscodedSize != file.Size()) {
						if ok {
							runCommand("rm", "-f", meta.TranscodedMovie)
						}

						if meta = transcode(movie, *hwaccel, *threads, *crf, *codec); meta != nil {
							meta.Movie = movieName.Name()
							mediaMetadata = writeMetadata(mediaDir, meta)
						}
					}
				}
			}
		}
	}
}
