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
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/minio/minio-go/v7"
)

// localStore — хранилище файлов на диске вместо объектного хранилища.
//
// Файлы лежат в <Dir>/<бакет>/<путь>. Отдельный сервис ради этого не нужен:
// на одной машине каталог делает ровно то же, что бакет, и не требует
// ни сети, ни учётных данных, ни отдельного процесса.
type localStore struct {
	dir string
}

func newLocalStore(dir string) *localStore {
	return &localStore{dir: dir}
}

// resolve строит путь к файлу и не выпускает его за пределы каталога хранилища:
// имя объекта приходит снаружи, и «..» в нём не должно уводить к чужим файлам.
func (s *localStore) resolve(bucket, path string) (string, error) {
	clean := filepath.Clean("/" + strings.TrimPrefix(path, "/"))
	full := filepath.Join(s.dir, filepath.Clean("/"+bucket), clean)

	root := filepath.Clean(s.dir) + string(os.PathSeparator)
	if !strings.HasPrefix(full, root) {
		return "", fmt.Errorf("недопустимый путь к файлу: %s/%s", bucket, path)
	}
	return full, nil
}

func (s *localStore) readAll(bucket, path string) ([]byte, error) {
	name, err := s.resolve(bucket, path)
	if err != nil {
		return nil, err
	}
	return os.ReadFile(name)
}

// readRange читает кусок файла — так клиент забирает большие файлы частями.
func (s *localStore) readRange(bucket, path string, offset int64, limit int32) ([]byte, error) {
	name, err := s.resolve(bucket, path)
	if err != nil {
		return nil, err
	}

	file, err := os.Open(name)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	buffer := make([]byte, limit)
	n, err := file.ReadAt(buffer, offset)
	if n > 0 {
		// io.EOF в конце файла — не ошибка: кусок прочитан, просто он последний
		return buffer[:n], nil
	}
	if err == io.EOF {
		return nil, nil
	}
	return nil, err
}

func (s *localStore) write(bucket, path string, r io.Reader) (minio.UploadInfo, error) {
	name, err := s.resolve(bucket, path)
	if err != nil {
		return minio.UploadInfo{}, err
	}
	if err = os.MkdirAll(filepath.Dir(name), 0o755); err != nil {
		return minio.UploadInfo{}, err
	}

	// пишем через временный файл: оборванная загрузка не оставит битый файл на месте готового
	temporary, err := os.CreateTemp(filepath.Dir(name), ".upload-*")
	if err != nil {
		return minio.UploadInfo{}, err
	}
	defer os.Remove(temporary.Name())

	size, err := io.Copy(temporary, r)
	if err != nil {
		temporary.Close()
		return minio.UploadInfo{}, err
	}
	if err = temporary.Close(); err != nil {
		return minio.UploadInfo{}, err
	}
	if err = os.Rename(temporary.Name(), name); err != nil {
		return minio.UploadInfo{}, err
	}

	return minio.UploadInfo{Bucket: bucket, Key: path, Size: size}, nil
}

func (s *localStore) writeFile(bucket, path, source string) (minio.UploadInfo, error) {
	file, err := os.Open(source)
	if err != nil {
		return minio.UploadInfo{}, err
	}
	defer file.Close()
	return s.write(bucket, path, file)
}

// Ниже — единые точки доступа к хранилищу: методы MinioUtil ходят через них,
// а не напрямую в клиент, поэтому подмена хранилища не задевает их логику.

func (m *MinioUtil) readAll(ctx context.Context, bucket, path string) ([]byte, error) {
	if m.local != nil {
		return m.local.readAll(bucket, path)
	}

	object, err := m.minio.Client.GetObject(ctx, bucket, path, minio.GetObjectOptions{})
	if err != nil {
		return nil, err
	}
	defer object.Close()

	stat, err := object.Stat()
	if err != nil {
		return nil, err
	}

	data := make([]byte, stat.Size)
	n, _ := object.Read(data)
	if n <= 0 {
		return nil, err
	}
	return data[:n], nil
}

func (m *MinioUtil) readRange(ctx context.Context, bucket, path string, offset int64, limit int32) ([]byte, error) {
	if m.local != nil {
		return m.local.readRange(bucket, path, offset, limit)
	}

	object, err := m.minio.Client.GetObject(ctx, bucket, path, minio.GetObjectOptions{})
	if err != nil {
		return nil, err
	}
	defer object.Close()

	data := make([]byte, limit)
	n, err := object.ReadAt(data, offset)
	if n > 0 {
		return data[:n], nil
	}
	return nil, err
}

func (m *MinioUtil) write(ctx context.Context, bucket, path string, r io.Reader, size int64, contentType string) (minio.UploadInfo, error) {
	if m.local != nil {
		return m.local.write(bucket, path, r)
	}
	return m.minio.Client.PutObject(ctx, bucket, path, r, size, s3PutOptions(false, contentType))
}

func (m *MinioUtil) writeFile(ctx context.Context, bucket, path, source, contentType string) (minio.UploadInfo, error) {
	if m.local != nil {
		return m.local.writeFile(bucket, path, source)
	}
	return m.minio.Client.FPutObject(ctx, bucket, path, source, s3PutOptions(false, contentType))
}
