/*
 * Copyright (C) 2021 IBM, Inc.
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

package extract

import (
	"github.com/netobserv/flowlogs2metrics/pkg/config"
	log "github.com/sirupsen/logrus"
)

type Extractor interface {
	Extract(in []config.GenericMap) []config.GenericMap
}

type extractNone struct {
}

// Extract extracts a flow before being stored
func (t *extractNone) Extract(f []config.GenericMap) []config.GenericMap {
	return f
}

// NewExtractNone create a new extract
func NewExtractNone() (Extractor, error) {
	log.Debugf("entering NewExtractNone")
	return &extractNone{}, nil
}
