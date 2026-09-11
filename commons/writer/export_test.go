package writer

import (
	"context"
	"io"
	"os"
)

// WriteFileWithSync exposes the filesystem write path with an injected sync operation for tests.
func WriteFileWithSync(ctx context.Context, w *FileSystemWriter, name string, reader io.Reader, syncFile func(*os.File) error) error {
	return w.writeFileWithSync(ctx, name, reader, syncFile)
}
