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

package minio_util

import (
	"bytes"
	"context"
	"io"
	"path/filepath"

	"github.com/teamgram/teamgram-server/app/service/dfs/internal/model"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/minio/minio-go/v7/pkg/encrypt"
	"github.com/zeromicro/go-zero/core/logx"
)

// BucketConfig
// endpoint := "127.0.0.1:9000"
// accessKeyID := "Q3AM3UQ867SPQQA43P2F"
// secretAccessKey := "zuf+tfteSlswRu7BJ86wekitnifILbZam1KYY3TG"
// useSSL := true
type BucketConfig struct {
	Documents      string `json:",default=documents"`
	Photos         string `json:",default=photos"`
	Videos         string `json:",default=videos"`
	EncryptedFiles string `json:",default=encryptedfiles"`
}

type MinioConfig struct {
	// Endpoint пуст — файлы лежат на диске в каталоге Dir, объектное хранилище не нужно
	Endpoint        string `json:",optional"`
	AccessKeyID     string `json:",optional"`
	SecretAccessKey string `json:",optional"`
	UseSSL          bool   `json:",optional"`
	Dir             string `json:",optional"`

	Bucket BucketConfig
}

type MinioUtil struct {
	c     *MinioConfig
	minio *minio.Core
	local *localStore
}

func MustNewMinioClient(c *MinioConfig) *MinioUtil {
	// Без адреса объектного хранилища работаем с файлами на диске.
	if c.Endpoint == "" {
		dir := c.Dir
		if dir == "" {
			dir = "../data/files"
		}
		logx.Infof("файлы хранятся локально: %s", dir)
		return &MinioUtil{c: c, local: newLocalStore(dir)}
	}

	core, err := minio.NewCore(
		c.Endpoint,
		&minio.Options{
			Creds:  credentials.NewStaticV4(c.AccessKeyID, c.SecretAccessKey, ""),
			Secure: c.UseSSL,
		})
	if err != nil {
		logx.Must(err)
	}

	return &MinioUtil{
		c:     c,
		minio: core,
	}
}

//func (d *Dao) Read() {
//
//}

func s3PutOptions(encrypted bool, contentType string) minio.PutObjectOptions {
	options := minio.PutObjectOptions{}
	if encrypted {
		options.ServerSideEncryption = encrypt.NewSSE()
	}
	options.ContentType = contentType

	return options
}

func (m *MinioUtil) GetPhotoFileData(ctx context.Context, path string) ([]byte, error) {
	data, err := m.readAll(ctx, m.c.Bucket.Photos, path)
	if err != nil {
		logx.WithContext(ctx).Errorf("GetPhotoFileData (%s) error: %v", path, err)
	}
	return data, err
}

func (m *MinioUtil) GetFile(ctx context.Context, bucket, path string, offset int64, limit int32) (bytes []byte, err error) {
	bytes, err = m.readRange(ctx, bucket, path, offset, limit)
	if err != nil {
		logx.WithContext(ctx).Errorf("GetFile (%s) error: %v", path, err)
	}
	return
}

func (m *MinioUtil) GetPhotoFile(ctx context.Context, path string, offset int64, limit int32) (bytes []byte, err error) {
	return m.GetFile(ctx, m.c.Bucket.Photos, path, offset, limit)
}

func (m *MinioUtil) PutPhotoFile(ctx context.Context, path string, buf []byte) (n minio.UploadInfo, err error) {
	var (
		contentType string
	)

	if ext := filepath.Ext(path); model.IsFileExtImage(ext) {
		contentType = model.GetImageMimeType(ext)
	} else {
		contentType = "binary/octet-stream"
	}

	n, err = m.write(ctx, m.c.Bucket.Photos, path, bytes.NewReader(buf), int64(len(buf)), contentType)
	if err != nil {
		logx.WithContext(ctx).Errorf("PutPhotoFile (%s) error: %v", path, err)
	}
	return
}

func (m *MinioUtil) PutPhotoFileV2(ctx context.Context, path string, r io.Reader) (n minio.UploadInfo, err error) {
	var (
		contentType string
	)

	if ext := filepath.Ext(path); model.IsFileExtImage(ext) {
		contentType = model.GetImageMimeType(ext)
	} else {
		contentType = "binary/octet-stream"
	}

	n, err = m.write(ctx, m.c.Bucket.Photos, path, r, -1, contentType)
	if err != nil {
		logx.Errorf("PutPhotoFile (%s) error: %v", path, err)
	}
	return
}

func (m *MinioUtil) GetVideoFile(ctx context.Context, path string, offset int64, limit int32) (bytes []byte, err error) {
	return m.GetFile(ctx, m.c.Bucket.Videos, path, offset, limit)
}

func (m *MinioUtil) PutVideoFile(ctx context.Context, path string, buf []byte) (n minio.UploadInfo, err error) {
	var (
		contentType string
	)

	if ext := filepath.Ext(path); model.IsFileExtImage(ext) {
		contentType = model.GetImageMimeType(ext)
	} else {
		contentType = "binary/octet-stream"
	}

	n, err = m.write(ctx, m.c.Bucket.Videos, path, bytes.NewReader(buf), int64(len(buf)), contentType)
	if err != nil {
		logx.WithContext(ctx).Errorf("PutVideoFile (%s) error: %v", path, err)
	}

	return
}

func (m *MinioUtil) PutVideoFileV2(ctx context.Context, path string, r io.Reader) (n minio.UploadInfo, err error) {
	var (
		contentType string
	)

	if ext := filepath.Ext(path); model.IsFileExtImage(ext) {
		contentType = model.GetImageMimeType(ext)
	} else {
		contentType = "binary/octet-stream"
	}

	n, err = m.write(ctx, m.c.Bucket.Videos, path, r, -1, contentType)
	if err != nil {
		logx.Errorf("PutVideoFileV2 (%s) error: %v", path, err)
	}
	return
}

func (m *MinioUtil) GetDocumentFile(ctx context.Context, path string, offset int64, limit int32) (bytes []byte, err error) {
	return m.GetFile(ctx, m.c.Bucket.Documents, path, offset, limit)
}

func (m *MinioUtil) PutDocumentFile(ctx context.Context, path string, r io.Reader) (n minio.UploadInfo, err error) {
	var (
		contentType string
	)

	if ext := filepath.Ext(path); model.IsFileExtImage(ext) {
		contentType = model.GetImageMimeType(ext)
	} else {
		contentType = "binary/octet-stream"
	}

	n, err = m.write(ctx, m.c.Bucket.Documents, path, r, -1, contentType)
	if err != nil {
		logx.WithContext(ctx).Errorf("PutDocumentFile (%s) error: %v", path, err)
	}

	return
}

func (m *MinioUtil) FPutDocumentFile(ctx context.Context, path string, r string) (n minio.UploadInfo, err error) {
	var (
		contentType string
	)

	if ext := filepath.Ext(path); model.IsFileExtImage(ext) {
		contentType = model.GetImageMimeType(ext)
	} else {
		contentType = "binary/octet-stream"
	}

	n, err = m.writeFile(ctx, m.c.Bucket.Documents, path, r, contentType)
	if err != nil {
		logx.WithContext(ctx).Errorf("PutDocumentFile (%s) error: %v", path, err)
	}

	return
}

func (m *MinioUtil) GetEncryptedFile(ctx context.Context, path string, offset int64, limit int32) (bytes []byte, err error) {
	return m.GetFile(ctx, m.c.Bucket.EncryptedFiles, path, offset, limit)
}

func (m *MinioUtil) PutEncryptedFile(ctx context.Context, path string, r io.Reader) (n minio.UploadInfo, err error) {
	n, err = m.write(ctx, m.c.Bucket.EncryptedFiles, path, r, -1, "binary/octet-stream")
	if err != nil {
		logx.WithContext(ctx).Errorf("PutEncryptedFile (%s) error: %v", path, err)
	}

	return
}
