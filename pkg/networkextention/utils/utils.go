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
package utils

import "unsafe"

// Bytes2str is used to convert byte to string.
// Note: Not safe in multi-goroutine scenarios.
func Bytes2str(b []byte) string {
	return *(*string)(unsafe.Pointer(&b))
}

// DeepCopyString is used to deep copy string.
// Note: Not safe in multi-goroutine scenarios.
func DeepCopyString(s string) string {
	b := make([]byte, len(s))
	copy(b, s)
	return Bytes2str(b)
}
