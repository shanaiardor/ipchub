// Copyright calabashdad. https://github.com/calabashdad/seal.git
//
// Copyright (c) 2019,CAOHONGJU All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package mpegts

import (
	"github.com/cnotch/ipchub/av/codec/aac"
	"github.com/cnotch/ipchub/av/codec/h264"
)

// the mpegts header specifed the video/audio pid.
const (
	tsVideoPid = 256
	tsAudioPid = 257
)

// the mpegts header specifed the stream id.
const (
	tsAudioAac = 0xc0 // ts aac stream id.
	tsVideoAvc = 0xe0 // ts avc stream id.
)

// Frame mpegts frame
type Frame struct {
	Pid      int
	StreamID int
	Dts      int64
	Pts      int64
	Header   []byte // 1. AAC-ADTS Header; 2. aud nal [+sps nal+pps nal]+sample nal start code
	Payload  []byte // data without startcode
	key      bool
}

// IsVideo .
func (frame *Frame) IsVideo() bool {
	return frame.Pid == tsVideoPid
}

// IsAudio .
func (frame *Frame) IsAudio() bool {
	return frame.Pid == tsAudioPid
}

// IsKeyFrame 判断是否是 video key frame
func (frame *Frame) IsKeyFrame() bool {
	return frame.key
}

func (frame *Frame) prepareAvcHeader(sps, pps []byte) {
	// a ts sample is format as:
	// 00 00 00 01 // header
	//       xxxxxxx // data bytes
	// 00 00 01 // continue header
	//       xxxxxxx // data bytes.
	// so, for each sample, we append header in aud_nal, then appends the bytes in sample.
	// for type1/5/6, insert aud packet.
	audNal := []byte{0x00, 0x00, 0x00, 0x01, 0x09, 0xf0}

	// step 1:
	// first, before each "real" sample,
	// we add some packets according to the nal_unit_type,
	// for example, when got nal_unit_type=5, insert SPS/PPS before sample.

	// 5bits, 7.3.1 NAL unit syntax,
	// H.264-AVC-ISO_IEC_14496-10.pdf, page 44.
	var nalUnitType uint8
	nalUnitType = frame.Payload[0]
	nalUnitType &= 0x1f

	// 6: Supplemental enhancement information (SEI) sei_rbsp( ), page 61
	// @see: ngx_rtmp_hls_append_aud
	// @remark, when got type 9, we donot send aud_nal, but it will make ios unhappy, so we remove it.
	if h264.NalSlice == nalUnitType || h264.NalIdrSlice == nalUnitType || h264.NalSei == nalUnitType {
		frame.Header = append(frame.Header, audNal...)
	}

	// 5: Coded slice of an IDR picture.
	// insert sps/pps before IDR or key frame is ok.
	if h264.NalIdrSlice == nalUnitType {
		// @see: ngx_rtmp_hls_append_sps_pps
		if len(sps) > 0 {
			// AnnexB prefix, for sps always 4 bytes header
			frame.Header = append(frame.Header, audNal[:4]...)
			// sps
			frame.Header = append(frame.Header, sps...)
		}

		if len(pps) > 0 {
			// AnnexB prefix, for pps always 4 bytes header
			frame.Header = append(frame.Header, audNal[:4]...)
			// pps
			frame.Header = append(frame.Header, pps...)
		}
	}

	// 7-9, ignore, @see: ngx_rtmp_hls_video
	if nalUnitType >= h264.NalSps && nalUnitType <= h264.NalAud {
		return
	}

	// step 2:
	// output the "real" sample, in buf.
	// when we output some special assist packets according to nal_unit_type

	// sample start prefix, '00 00 00 01' or '00 00 01'
	pAudnal := 0 + 1
	endAudnal := pAudnal + 3

	// first AnnexB prefix is long (4 bytes)
	if 0 == len(frame.Header) {
		pAudnal = 0
	}
	frame.Header = append(frame.Header, audNal[pAudnal:pAudnal+endAudnal-pAudnal]...)

	return
}

func (frame *Frame) prepareAacHeader(sps *aac.RawSPS) {
	adtsHeader := sps.ToAdtsHeader(len(frame.Payload))
	frame.Header = adtsHeader[:]
	return
}

// FrameWriter 包装 WriteMpegtsFrame 方法的接口
type FrameWriter interface {
	WriteMpegtsFrame(frame *Frame) error
}
