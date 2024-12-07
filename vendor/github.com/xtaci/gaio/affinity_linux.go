// The MIT License (MIT)
//
// Copyright (c) 2019 xtaci
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.

//go:build linux

package gaio

/*
#define _GNU_SOURCE
#include <sched.h>
#include <pthread.h>

// lock_thread sets the CPU affinity for the calling thread to the specified CPU.
// This ensures that the thread runs only on the designated CPU, which can help
// with performance optimization and consistent execution timing on multi-core systems.
void lock_thread(int cpuid) {
	pthread_t tid;
	cpu_set_t cpuset;

	tid = pthread_self();               // Get the ID of the calling thread
	CPU_ZERO(&cpuset);                  // Initialize the CPU set to be empty
	CPU_SET(cpuid, &cpuset);            // Add the specified CPU to the set
	pthread_setaffinity_np(tid, sizeof(cpu_set_t), &cpuset); // Set the thread's CPU affinity
}
*/
import "C"
import (
	"runtime"
)

// setAffinity binds the current goroutine and its underlying thread to a specific CPU.
// This function locks the goroutine to the current thread, then sets the thread's CPU affinity
// to the specified CPU, which can improve performance by reducing context switching.
func setAffinity(cpuId int32) {
	runtime.LockOSThread()      // Lock the current goroutine to its current thread
	C.lock_thread(C.int(cpuId)) // Set the thread's CPU affinity to the specified CPU
}
