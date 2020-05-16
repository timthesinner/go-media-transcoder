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
	"time"

	"github.com/fsnotify/fsnotify"
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
	Movie             string `json:"movie"`
	OriginalMovie     string `json:"originalFile"`
	OriginalCodec     string `json:"originalCodec"`
	OriginalWidth     int    `json:"originalWidth"`
	OriginalPixFormat string `json:"originalPixFormat"`

	TranscodedMovie string `json:"transcodedFile"`
	TranscodedCodec string `json:"transcodedCodec"`
	TranscodedWidth int    `json:"transcodedWidth"`
	TranscodedSize  int64  `json:"transcodedSize"`
	TranscodedSpeed string `json:"transcodedSpeed"`
	TranscodeCRF    int    `json:"transcodedCRF"`

	TranscodedBitrate  string `json:"transcodedBitrate"`
	TranscodedDuration string `json:"transcodedDuration"`
}

func handle(err error) {
	if err != nil {
		log.Fatal(err)
	}
}

func movieAsMkv(movie string) string {
	baseName := filepath.Base(movie)
	return strings.TrimSuffix(baseName, filepath.Ext(baseName)) + ".mkv"
}

func transcodedMovie(originalMovie string) string {
	return filepath.Join(filepath.Dir(originalMovie), "transcode-"+movieAsMkv(originalMovie))
}

func transcode(originalMovie string, hwaccel string, threads int, crf int, codec string) *Transcode {
	lock, err := NewLockfile(filepath.Join(filepath.Dir(originalMovie), "transcoding.lck"))
	if err != nil {
		return nil
	}
	defer lock.Unlock()

	streams := movieMetadata(originalMovie)["streams"].([]interface{})
	videoStream := findVideoStream(streams)
	if videoStream == nil {
		return nil
	}

	scale := "scale=1920:-2"
	width, ok := videoStream["coded_width"].(float64)
	if !ok {
		return nil
	}

	if int(width) <= 1920 {
		scale = ""
	}

	english := FilterEnglishStreams(streams)
	if len(english) == 0 {
		if _, err := os.Stat(filepath.Join(filepath.Dir(originalMovie), "verified-english")); err == nil {
			english = []string{"-map", "0:a"}
		} else {
			fmt.Println("Did not detect any english streams")
			for i := 1; i <= 10; i++ {
				fmt.Println("Please Verify English: " + filepath.Dir(originalMovie))
			}
			return nil
		}
	}

	targetMovie := transcodedMovie(originalMovie)
	transcodeArgs := []string{
		"-nostdin",
		"-hide_banner",
		"-avioflags", "direct",
		"-rtbufsize", "64M",
	}

	if strings.TrimSpace(hwaccel) != "" {
		transcodeArgs = append(transcodeArgs, "-hwaccel", hwaccel)
	}

	transcodeArgs = append(transcodeArgs,
		"-analyzeduration", "512M", "-probesize", "512M", "-fix_sub_duration",
		"-i", originalMovie,
		"-max_muxing_queue_size", "65536",
		"-map_metadata:g", "0:g",
		"-map_metadata:s:v", "0:s:v")

	if HasAttachmentStreams(streams) {
		transcodeArgs = append(transcodeArgs, "-map_metadata:s:t", "0:s:t")
	}

	transcodeArgs = append(append(append(transcodeArgs, "-map", "0:v:0"), english...),
		"-map", "0:t?",
		"-c:v", codec)

	if scale != "" {
		transcodeArgs = append(transcodeArgs, "-vf", scale)
	}

	transcodeArgs = append(transcodeArgs,
		"-crf", strconv.Itoa(crf), "-preset", *speed, "-pix_fmt", *pixFmt, "-tune", "fastdecode", "-movflags", "+faststart",
		"-c:a", "libopus", "-b:a", "512k", "-vbr", "on", "-af", "aformat=channel_layouts='7.1|6.1|5.1|stereo'", "-compression_level", "10", "-frame_duration", "60",
		"-c:s", *subtitleCodec,
		"-metadata:s:a", "language=eng",
		"-metadata:s:s", "language=eng",
		"-metadata:s:v", "language=eng",
		"-metadata:s:v", "title="+filepath.Base(filepath.Dir(originalMovie)),
		"-metadata:s:v", "description=Encoded by https://github.com/timthesinner/go-media-transcoder")

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
	originalMovie = filepath.Join(filepath.Dir(originalMovie), movieAsMkv(originalMovie))
	runCommand("mv", targetMovie, originalMovie)
	info, err := os.Stat(originalMovie)
	handle(err)

	transcodedMetadata := movieMetadata(originalMovie)
	transcodedFormat := transcodedMetadata["format"].(map[string]interface{})
	duration, _ := time.ParseDuration(transcodedFormat["duration"].(string) + "s")
	transcodedStream := transcodedMetadata["streams"].([]interface{})[0].(map[string]interface{})
	return &Transcode{
		OriginalMovie:     filepath.Base(rawMovie),
		OriginalCodec:     videoStream["codec_name"].(string),
		OriginalWidth:     int(videoStream["width"].(float64)),
		OriginalPixFormat: videoStream["pix_fmt"].(string),

		TranscodedMovie: filepath.Base(originalMovie),
		TranscodedCodec: transcodedStream["codec_name"].(string),
		TranscodedWidth: int(transcodedStream["width"].(float64)),
		//TranscodedHash:  md5FromFile(originalMovie),
		TranscodedSize:     info.Size(),
		TranscodedSpeed:    *speed,
		TranscodeCRF:       crf,
		TranscodedDuration: duration.String(),
		TranscodedBitrate:  transcodedFormat["bit_rate"].(string),
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

func movieProcessor(mediaDir string) func(os.FileInfo) {
	mediaMetadata := readMetadata(mediaDir)
	processMovie := func(movieName os.FileInfo) {
		if !movieName.IsDir() {
			return
		}

		movieDir := filepath.Join(mediaDir, movieName.Name())
		files, err := ioutil.ReadDir(movieDir)
		handle(err)

		for _, file := range files {
			if !file.IsDir() {
				movie := filepath.Join(movieDir, file.Name())

				if process, ok := PROCESS_FILE_EXTENSIONS[filepath.Ext(file.Name())]; !ok {
					fmt.Printf("UNKNOWN FILE TYPE %s in %s\n", filepath.Ext(file.Name()), movieName.Name())
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

	return processMovie
}

var PROCESS_FILE_EXTENSIONS = map[string]bool{
	".ts":   true,
	".avi":  true,
	".mkv":  true,
	".mp4":  true,
	".m2ts": true,
	".m4v":  true,
	".wmv":  true,

	// Do not process originals
	".ts-orig":   false,
	".avi-orig":  false,
	".mkv-orig":  false,
	".mp4-orig":  false,
	".m2ts-orig": false,
	".m4v-orig":  false,
	".wmv-orig":  false,

	// Do not process lock files
	".lck": false,

	".srt":      false,
	".nfo":      false,
	".jpg":      false,
	".DS_Store": false,
	".nfo-orig": false,
	"":          false,
}

const MIN_FILE_SIZE = 256 * 1024 * 1024

var hwaccel = flag.String("hwaccel", "", "Hardware Acceleration Driver")
var threads = flag.Int("threads", 0, "Number of threads")
var crf = flag.Int("crf", 20, "CRF (Quality Factor)")
var codec = flag.String("codec", "hevc_amf", "Video encoding codec")
var speed = flag.String("speed", "placebo", "Encoder speed")
var pixFmt = flag.String("pix_fmt", "yuv420p", "Video color depth, dont go deeper than yuv420p if your encoding for a pi")
var subtitleCodec = flag.String("subtitle-codec", "copy", "Codec to use when interacting with the subtitles stream")

func main() {
	flag.Parse()

	mediaDir := "/Volumes/downloads/movies/"
	if flag.NArg() > 0 {
		mediaDir = flag.Arg(0)
	}

	processor := movieProcessor(mediaDir)

	watcher, err := fsnotify.NewWatcher()
	handle(err)
	handle(watcher.Add(mediaDir))

	movies, err := ioutil.ReadDir(mediaDir)
	handle(err)

	// Process all movies immediatley
	for _, movieName := range movies {
		processor(movieName)
	}

	done := make(chan bool)

	go func() {
		for {
			select {
			// watch for events
			case event := <-watcher.Events:
				fileStat, err := os.Stat(event.Name)
				handle(err)
				processor(fileStat)

			case err := <-watcher.Errors:
				handle(err)
			}
		}
	}()

	<-done
}
