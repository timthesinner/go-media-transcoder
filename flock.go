//By TimTheSinner
package main

import (
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"

	"github.com/nightlyone/lockfile"
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

type Lockfile struct {
	name string
	lockfile.Lockfile
}

var (
	ErrPidMissmatch   = errors.New("Lockfile has a different pid")
	ErrMissingPidFile = errors.New("Could not find pid file")
)

func NewLockfile(path string) (Lockfile, error) {
	lock, err := lockfile.New(path)
	if err != nil {
		return Lockfile{"", lock}, err
	}

	ourLock := Lockfile{path, lock}
	err = ourLock.TryLock()
	if err != nil {
		return Lockfile{"", lockfile.Lockfile("")}, err
	}
	return ourLock, nil
}

func (l Lockfile) TryLock() error {
	name := l.name

	tmplock, err := ioutil.TempFile(filepath.Dir(name), filepath.Base(name)+".")
	if err != nil {
		return err
	}

	if err = writePidLine(tmplock, os.Getpid()); err != nil {
		return err
	} else if err = tmplock.Close(); err != nil {
		return err
	}

	//If the lockfile exists
	if _, err = os.Stat(name); err == nil {
		_ = os.Remove(tmplock.Name())
		if proc, err := l.GetOwner(); err != nil {
			return err
		} else if proc.Pid != os.Getpid() {
			return ErrPidMissmatch
		} else {
			fmt.Printf("pid=%d currentPid=%d", proc.Pid, os.Getpid())
		}
		return nil
	}

	if err = os.Rename(tmplock.Name(), name); err != nil {
		return err
	}

	_, err = os.Stat(name)
	if err != nil {
		if os.IsNotExist(err) {
			return ErrMissingPidFile
		}
		return err
	}

	// Pause for a short bit to make sure no one else wrote to the same file
	time.Sleep(100 * time.Millisecond)

	if proc, err := l.GetOwner(); err != nil {
		return err
	} else if proc.Pid != os.Getpid() {
		return ErrPidMissmatch
	}
	return nil
}

func writePidLine(w io.Writer, pid int) error {
	_, err := io.WriteString(w, fmt.Sprintf("%d\n", pid))
	return err
}
