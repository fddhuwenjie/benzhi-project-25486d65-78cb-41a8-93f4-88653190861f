package web

import (
	"mime"
	"path/filepath"
)

func assetContentType(name string) string {
	if typ := mime.TypeByExtension(filepath.Ext(name)); typ != "" {
		return typ
	}
	return "application/octet-stream"
}
