// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

package integration

import (
	"context"
	"log"
	"os"

	"github.com/mohanjith/microsoft-authentication-library-for-go/apps/cache"
)

type TokenCache struct {
	file string
}

func (t *TokenCache) Replace(ctx context.Context, cache cache.Unmarshaler, hints cache.ReplaceHints) error {
	data, err := os.ReadFile(t.file)
	if err != nil {
		log.Println(err)
	}
	return cache.Unmarshal(data)
}

func (t *TokenCache) Export(ctx context.Context, cache cache.Marshaler, hints cache.ExportHints) error {
	data, err := cache.Marshal()
	if err != nil {
		log.Println(err)
	}
	return os.WriteFile(t.file, data, 0600)
}

func (t *TokenCache) Print() string {
	data, err := os.ReadFile(t.file)
	if err != nil {
		return err.Error()
	}
	return string(data)
}
