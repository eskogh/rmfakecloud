package fs

import (
	"archive/zip"
	"encoding/json"
	"io"
	"time"

	"github.com/ddvk/rmfakecloud/internal/common"
	"github.com/ddvk/rmfakecloud/internal/storage"
	"github.com/ddvk/rmfakecloud/internal/storage/models"
)

func (fs *FileSystemStorage) exportOriginal(uid, id string) (io.ReadCloser, error) {
	id = common.Sanitize(id)
	metadata, err := fs.GetMetadata(uid, id)
	if err != nil {
		return nil, err
	}
	source, err := zip.OpenReader(fs.getPathFromUser(uid, id+storage.ZipFileExt))
	if err != nil {
		return nil, err
	}
	reader, writer := io.Pipe()
	go func() {
		defer source.Close()
		archive := zip.NewWriter(writer)
		var copyErr error
		for _, entry := range source.File {
			if entry.Name == id+storage.MetadataFileExt {
				continue
			}
			input, err := entry.Open()
			if err != nil {
				copyErr = err
				break
			}
			out, err := archive.Create(entry.Name)
			if err == nil {
				_, err = io.Copy(out, input)
			}
			input.Close()
			if err != nil {
				copyErr = err
				break
			}
		}
		if copyErr == nil {
			modified, _ := time.Parse(time.RFC3339Nano, metadata.ModifiedClient)
			out, err := archive.Create(id + storage.MetadataFileExt)
			if err == nil {
				err = json.NewEncoder(out).Encode(models.MetadataFile{DocumentName: metadata.VissibleName, CollectionType: metadata.Type, Parent: metadata.Parent, Version: metadata.Version, LastModified: models.FromTime(modified)})
			}
			copyErr = err
		}
		closeErr := archive.Close()
		if copyErr == nil {
			copyErr = closeErr
		}
		writer.CloseWithError(copyErr)
	}()
	return reader, nil
}
