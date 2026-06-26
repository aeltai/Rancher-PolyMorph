package backup

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"io"
	"os"
)

func writeMinimalFixture(path string) error {
	members := map[string]any{
		"clusters.management.cattle.io#v3/c-aaaaa.json": map[string]any{
			"metadata": map[string]any{"name": "c-aaaaa"},
			"spec":     map[string]any{"displayName": "target-rke2", "rke2Config": map[string]any{}},
		},
		"clusters.management.cattle.io#v3/c-bbbbb.json": map[string]any{
			"metadata": map[string]any{"name": "c-bbbbb"},
			"spec":     map[string]any{"displayName": "other-rke1", "rancherKubernetesEngineConfig": map[string]any{}},
		},
		"clusters.management.cattle.io#v3/local.json": map[string]any{
			"metadata": map[string]any{"name": "local"},
			"spec":     map[string]any{"displayName": "local", "rke2Config": map[string]any{}},
		},
		"nodes.management.cattle.io#v3/local/m-loc.json":   map[string]any{"metadata": map[string]any{"name": "m-loc"}},
		"nodes.management.cattle.io#v3/c-aaaaa/m-aaa.json": map[string]any{"metadata": map[string]any{"name": "m-aaa"}},
		"nodes.management.cattle.io#v3/c-bbbbb/m-bbb.json": map[string]any{"metadata": map[string]any{"name": "m-bbb"}},
		"settings.management.cattle.io#v3/server-url.json": map[string]any{"value": "https://example.com"},
	}

	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	gw := gzip.NewWriter(f)
	defer gw.Close()
	tw := tar.NewWriter(gw)
	defer tw.Close()

	for name, doc := range members {
		data, err := json.Marshal(doc)
		if err != nil {
			return err
		}
		hdr := &tar.Header{Name: name, Mode: 0o644, Size: int64(len(data))}
		if err := tw.WriteHeader(hdr); err != nil {
			return err
		}
		if _, err := io.Copy(tw, mustReader(data)); err != nil {
			return err
		}
	}
	return tw.Close()
}

type bytesReader struct {
	b []byte
	i int
}

func mustReader(b []byte) *bytesReader { return &bytesReader{b: b} }

func (r *bytesReader) Read(p []byte) (int, error) {
	if r.i >= len(r.b) {
		return 0, io.EOF
	}
	n := copy(p, r.b[r.i:])
	r.i += n
	return n, nil
}
