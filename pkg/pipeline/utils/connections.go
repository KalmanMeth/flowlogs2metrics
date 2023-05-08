/*
 * Copyright (C) 2023 IBM, Inc.
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
 *
 */

package utils

import (
	"fmt"

	"github.com/netobserv/flowlogs-pipeline/pkg/config"
)

const subnetBatchSize = 254

// GenerateConnectionFlowEntries generates data with one entry for each of nConnections
// Create the entries in a predictable manner so that the first K entries in each call
// to the function reproduce the same connection.
func GenerateConnectionFlowEntries(nConnections int) []config.GenericMap {
	entries := make([]config.GenericMap, 0)
	n1 := (nConnections / subnetBatchSize) + 1
	if n1 > 254 {
		n1 = 254
	}
	n2 := (nConnections / (subnetBatchSize * subnetBatchSize)) + 1
	if n2 > 254 {
		n2 = 254
	}
	n3 := (nConnections / (subnetBatchSize * subnetBatchSize * subnetBatchSize)) + 1
	if n3 > 254 {
		n3 = 254
	}
	count := 0
	for l := 1; l <= n3; l++ {
		for k := 1; k <= n2; k++ {
			for j := 1; j <= n1; j++ {
				for i := 1; i <= subnetBatchSize; i++ {
					srcAddr := fmt.Sprintf("%d.%d.%d.%d", l, k, j, i)
					count++
					entry := config.GenericMap{
						"SrcAddr":      srcAddr,
						"SrcPort":      1234,
						"DstAddr":      "11.1.1.1",
						"DstPort":      8000,
						"Bytes":        100,
						"Packets":      1,
						"Proto":        6,
						"SrcAS":        0,
						"DstAS":        0,
						"TimeReceived": 0,
					}
					entries = append(entries, entry)
					if count >= nConnections {
						return entries
					}
				}
			}
		}
	}
	return entries
}
