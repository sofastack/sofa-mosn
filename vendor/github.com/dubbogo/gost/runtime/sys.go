/*
 * Licensed to the Apache Software Foundation (ASF) under one or more
 * contributor license agreements.  See the NOTICE file distributed with
 * this work for additional information regarding copyright ownership.
 * The ASF licenses this file to You under the Apache License, Version 2.0
 * (the "License"); you may not use this file except in compliance with
 * the License.  You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package gxruntime

import (
	"io/ioutil"
	"os"
	"runtime"
	"runtime/pprof"
	"strconv"
	"strings"
	"time"
)

import (
	"github.com/shirou/gopsutil/process"
)

var (
	CurrentPID = os.Getpid()
)

const (
	cgroupMemLimitPath = "/sys/fs/cgroup/memory/memory.limit_in_bytes"
)

// GetThreadNum gets current process's thread number
func GetThreadNum() int {
	return pprof.Lookup("threadcreate").Count()
}

// GetGoroutineNum gets current process's goroutine number
func GetGoroutineNum() int {
	return runtime.NumGoroutine()
}

// GetProcessCPUStat gets current process's cpu stat
func GetProcessCPUStat() (float64, error) {
	p, err := process.NewProcess(int32(CurrentPID))
	if err != nil {
		return 0, err
	}

	cpuPercent, err := p.Percent(time.Second)
	if err != nil {
		return 0, err
	}

	// The default percent is if you use one core, then 100%, two core, 200%
	// but it's inconvenient to calculate the proper percent
	// here we multiply by core number, so we can set a percent bar more intuitively
	cpuPercent = cpuPercent / float64(runtime.GOMAXPROCS(-1))

	return cpuPercent, nil
}

// GetProcessMemoryStat gets current process's memory usage percent
func GetProcessMemoryPercent() (float32, error) {
	p, err := process.NewProcess(int32(CurrentPID))
	if err != nil {
		return 0, err
	}

	memPercent, err := p.MemoryPercent()
	if err != nil {
		return 0, err
	}

	return memPercent, nil
}

// GetProcessMemoryStat gets current process's memory usage in Byte
func GetProcessMemoryStat() (uint64, error) {
	p, err := process.NewProcess(int32(CurrentPID))
	if err != nil {
		return 0, err
	}

	memInfo, err := p.MemoryInfo()

	return memInfo.RSS, nil
}

// copied from https://github.com/containerd/cgroups/blob/318312a373405e5e91134d8063d04d59768a1bff/utils.go#L251
func parseUint(s string, base, bitSize int) (uint64, error) {
	v, err := strconv.ParseUint(s, base, bitSize)
	if err != nil {
		intValue, intErr := strconv.ParseInt(s, base, bitSize)
		// 1. Handle negative values greater than MinInt64 (and)
		// 2. Handle negative values lesser than MinInt64
		if intErr == nil && intValue < 0 {
			return 0, nil
		} else if intErr != nil &&
			intErr.(*strconv.NumError).Err == strconv.ErrRange &&
			intValue < 0 {
			return 0, nil
		}
		return 0, err
	}
	return v, nil
}

// copied from https://github.com/containerd/cgroups/blob/318312a373405e5e91134d8063d04d59768a1bff/utils.go#L243
func readUint(path string) (uint64, error) {
	v, err := ioutil.ReadFile(path)
	if err != nil {
		return 0, err
	}
	return parseUint(strings.TrimSpace(string(v)), 10, 64)
}

func GetCgroupMemoryLimit() (uint64, error) {
	return readUint(cgroupMemLimitPath)
}

// GetCgroupProcessMemoryPercent gets current process's memory usage percent in cgroup env
func GetCgroupProcessMemoryPercent() (float64, error) {
	p, err := process.NewProcess(int32(os.Getpid()))
	if err != nil {
		return 0, err
	}

	mem, err := p.MemoryInfo()
	if err != nil {
		return 0, err
	}

	memLimit, err := GetCgroupMemoryLimit()
	if err != nil {
		return 0, err
	}

	// mem.RSS / cgroup limit in bytes
	memPercent := float64(mem.RSS) * 100 / float64(memLimit)

	return memPercent, nil
}
