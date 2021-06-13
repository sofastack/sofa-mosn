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

package types

import (
	"crypto/sha256"
	"github.com/stretchr/testify/assert"
	"testing"

	envoy_api_v2_auth "github.com/envoyproxy/go-control-plane/envoy/api/v2/auth"
)

func TestSecretConvert(t *testing.T) {
	var sec = envoy_api_v2_auth.Secret{
		Name: "mosn",
	}

	sdsSec := SecretConvertV2(&sec)
	assert.Equal(t, sdsSec.Name, "mosn")
}

func TestHashValue(t *testing.T) {
	data := []byte("123456")
	hash1 := NewHashValue(sha256.Sum256(data))
	hash2 := NewHashValue(sha256.Sum256(data))
	// same hash value compare
	if !hash1.Equal(hash2) {
		t.Fatalf("same data got different hash value %s: %s\n", hash1, hash2)
	}
	hash3 := NewHashValue(sha256.Sum256([]byte("1234567")))
	// different hash value compare
	if hash1.Equal(hash3) {
		t.Fatalf("different data got same hash value %s: %s", hash1, hash3)
	}
	// empty hash value compare
	var empty *HashValue
	if !empty.Equal(nil) {
		t.Fatal("empty hash compare failed")
	}
	if hash1.Equal(empty) {
		t.Fatal("hash value equals to nil")
	}
}

func BenchmarkHashValue(b *testing.B) {
	data := []byte("123456")
	hash1 := NewHashValue(sha256.Sum256(data))
	hash2 := NewHashValue(sha256.Sum256(data))
	b.Run("hash_equal", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = hash1.Equal(hash2)
		}
	})
	// bench string
	b.Run("hash_string", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = hash1.String()
		}
	})
}
