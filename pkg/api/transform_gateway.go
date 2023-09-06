/*
 * Copyright (C) 2022 IBM, Inc.
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

package api

type TransformGateway struct {
	ServerAddr string `yaml:"serverAddr,omitempty" json:"serverAddr,omitempty" doc:"address of mbg log proxy:"`
	Addr1      string `yaml:"addr1,omitempty" json:"addr1,omitempty" doc:"field name in flow log of first IP address:"`
	Port1      string `yaml:"port1,omitempty" json:"port1,omitempty" doc:"field name in flow log of port of first IP address:"`
	Addr2      string `yaml:"addr2,omitempty" json:"addr2,omitempty" doc:"field name in flow log of second IP address:"`
	Port2      string `yaml:"port2,omitempty" json:"port2,omitempty" doc:"field name in flow log of port of second IP address:"`
}
