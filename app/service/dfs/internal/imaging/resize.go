// Copyright 2022 Teamgram Authors
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
//
// Author: teamgramio (teamgram.io@gmail.com)
//

package imaging

import (
	"bytes"
	"image"
	"strings"

	"github.com/teamgram/marmota/pkg/bytes2"
	"github.com/teamgram/proto/mtproto"

	"github.com/disintegration/imaging"
	"github.com/zeromicro/go-zero/core/logx"
)

const (
	JPEG imaging.Format = iota
	PNG
	GIF
	TIFF
	BMP
	WEBP
)

type resizeInfo struct {
	isWidth bool
	size    int
}

func makeResizeInfo(img image.Image) resizeInfo {
	w := img.Bounds().Dx()
	h := img.Bounds().Dy()

	if w >= h {
		return resizeInfo{
			isWidth: true,
			size:    w,
		}
	} else {
		return resizeInfo{
			isWidth: false,
			size:    h,
		}
	}
}

func getImageFormat(extName string) (int, error) {
	formats := map[string]imaging.Format{
		".jpg":  JPEG,
		".jpeg": JPEG,
		".png":  PNG,
		".tif":  TIFF,
		".tiff": TIFF,
		".bmp":  BMP,
		".gif":  GIF,
		// ".webp": WEBP,
	}

	ext := strings.ToLower(extName)
	f, ok := formats[ext]
	if !ok {
		return -1, imaging.ErrUnsupportedFormat
	}

	return int(f), nil
}

func ReSizeImage(rb []byte, extName string, isABC bool, cb func(szType string, localId int, w, h int32, b []byte) error) (err error) {
	var (
		img image.Image
	)

	img, err = imaging.Decode(bytes.NewReader(rb))
	if err != nil {
		logx.Errorf("decode r(%d) error: %v", len(rb), err)
		return
	}

	return ReSizeImageByImage(img, extName, isABC, cb)
}

func ReSizeImageByImage(img image.Image, extName string, isABC bool, cb func(szType string, localId int, w, h int32, b []byte) error) (err error) {
	if isABC {
		if img.Bounds().Dx() >= mtproto.PhotoSZDSize && img.Bounds().Dy() >= mtproto.PhotoSZDSize {
			if img.Bounds().Dx() != img.Bounds().Dy() {
				img = imaging.Fill(img, mtproto.PhotoSZDSize, mtproto.PhotoSZDSize, imaging.Center, imaging.Lanczos)
			}
		} else if img.Bounds().Dx() <= mtproto.PhotoSZCSize && img.Bounds().Dy() <= mtproto.PhotoSZCSize {
			img = imaging.Fill(img, mtproto.PhotoSZCSize, mtproto.PhotoSZCSize, imaging.Center, imaging.Lanczos)
		} else {
			if img.Bounds().Dx() != img.Bounds().Dy() {
				img = imaging.Fill(img, mtproto.PhotoSZCSize, mtproto.PhotoSZCSize, imaging.Center, imaging.Lanczos)
			}
		}
	}

	imgSz := makeResizeInfo(img)

	var (
		szList    []mtproto.ReSizeInfo
		willBreak = false
		rsz       int
	)

	if isABC {
		szList = mtproto.ReSizeInfoABCList
	} else {
		szList = mtproto.ReSizeInfoPhotoList
	}

	// A photo size is a JPEG whatever arrived. The encoder used to follow the
	// uploaded file's name: a PNG made PNG previews - 32 bits with an alpha
	// channel, and the resampler's ringing drove the 90-pixel size heavier
	// than the whole picture, so a client picking "the photo" by weight
	// showed a thumbnail. A file with no name made no sizes at all. And 95
	// was paying full price for the ringing; a preview does not need it.
	type made struct {
		szType  string
		localId int
		w, h    int32
		dst     *image.NRGBA
		data    []byte
	}
	encode := func(dst *image.NRGBA, quality int) ([]byte, error) {
		o := bytes2.NewBuffer(make([]byte, 0, 512*1024))
		encErr := imaging.Encode(o, dst, imaging.JPEG, imaging.JPEGQuality(quality))
		return o.Bytes(), encErr
	}

	var sizes []made
	for _, sz := range szList {
		rsz = sz.Size
		if rsz >= imgSz.size {
			rsz = imgSz.size
			willBreak = true
		}

		var dst *image.NRGBA
		if imgSz.isWidth {
			dst = imaging.Resize(img, rsz, 0, imaging.Lanczos)
		} else {
			dst = imaging.Resize(img, 0, rsz, imaging.Lanczos)
		}

		var data []byte
		data, err = encode(dst, 87)
		if err != nil {
			logx.Error(err.Error())
			return
		}
		sizes = append(sizes, made{sz.Type, sz.LocalId, int32(dst.Bounds().Dx()), int32(dst.Bounds().Dy()), dst, data})

		if willBreak {
			break
		}
	}

	// A preview never outweighs a larger size of the same photo. One quality
	// for every class does not guarantee that on its own: the largest class
	// is often the picture untouched while the one below it went through the
	// resampler, whose ringing is exactly what an encoder pays for. So the
	// classes are walked from the largest down, and any that outweighs its
	// larger neighbour is re-encoded a step rougher until it fits under it -
	// it is a preview, and a preview that cannot be smaller than the picture
	// has no reason to exist at full polish.
	for i := len(sizes) - 2; i >= 0; i-- {
		for quality := 82; len(sizes[i].data) > len(sizes[i+1].data) && quality >= 40; quality -= 7 {
			var data []byte
			data, err = encode(sizes[i].dst, quality)
			if err != nil {
				logx.Error(err.Error())
				return
			}
			sizes[i].data = data
		}
	}

	for _, sz := range sizes {
		err = cb(sz.szType, sz.localId, sz.w, sz.h, sz.data)
		if err != nil {
			return
		}
	}

	return
}
