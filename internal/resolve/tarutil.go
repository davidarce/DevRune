// SPDX-License-Identifier: MIT

package resolve

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
)

// extractFilesFromTar decompresses and reads all regular files from a
// gzip-compressed tar archive, returning a map of path → content.
// Paths are as stored in the archive (including any top-level prefix directory).
func extractFilesFromTar(data []byte) (map[string][]byte, error) {
	gr, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("open gzip: %w", err)
	}
	defer func() { _ = gr.Close() }()

	tr := tar.NewReader(gr)
	files := make(map[string][]byte)

	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("read tar entry: %w", err)
		}

		if hdr.Typeflag != tar.TypeReg {
			continue
		}

		content, err := io.ReadAll(tr)
		if err != nil {
			return nil, fmt.Errorf("read content of %q: %w", hdr.Name, err)
		}
		files[hdr.Name] = content
	}

	return files, nil
}
