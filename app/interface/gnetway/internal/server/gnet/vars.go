// Copyright (c) 2021-present,  Teamgram Studio (https://teamgram.io).
//  All rights reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//   http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package gnet

const (
	STATE_ERROR              = 0x0000
	STATE_CONNECTED2         = 0x0100
	STATE_HANDSHAKE          = 0x0200
	STATE_pq                 = 0x0201
	STATE_pq_res             = 0x0202
	STATE_pq_ack             = 0x0203
	STATE_DH_params          = 0x0204
	STATE_DH_params_res      = 0x0205
	STATE_DH_params_res_fail = 0x0206
	STATE_DH_params_ack      = 0x0207
	STATE_dh_gen             = 0x0208
	STATE_dh_gen_res         = 0x0209
	STATE_dh_gen_res_retry   = 0x020a
	STATE_dh_gen_res_fail    = 0x020b
	STATE_dh_gen_ack         = 0x020c
	STATE_AUTH_KEY           = 0x0300
)

const (
	RES_STATE_NONE  = 0x00
	RES_STATE_OK    = 0x01
	RES_STATE_ERROR = 0x02
)

const (
	// How long a connection may stay silent before the reaper closes it.
	// The client goes quiet for minutes at a time while its pings are paused,
	// yet still holds the socket and expects to write to it, so the budget has
	// to sit well above that rather than just above the ping interval.
	kIdleTimeout = 900
	// A connection that has not spoken since it opened has nothing to lose by
	// being reaped: the client puts its first request on a connection right
	// away, and a phone whose app went to the background is meant to lose these.
	kSilentConnTimeout = 30
	kHttpIdleTimeout   = 60
)
