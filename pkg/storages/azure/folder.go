package azure

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/wal-g/tracelog"

	"github.com/wal-g/wal-g/pkg/storages/storage"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/blob"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/bloberror"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/blockblob"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/container"
	"github.com/pkg/errors"
)

// TODO: Unit tests
type Folder struct {
	path                string
	containerClient     container.Client
	uploadStreamOptions azblob.UploadStreamOptions
	timeout             time.Duration
}

func NewFolder(
	path string,
	containerClient container.Client,
	uploadStreamOptions azblob.UploadStreamOptions,
	timeout time.Duration,
) *Folder {
	// Trim leading slash because there's no difference between absolute and relative paths in Azure.
	path = strings.TrimPrefix(path, "/")
	return &Folder{
		path,
		containerClient,
		uploadStreamOptions,
		timeout,
	}
}

func (folder *Folder) GetPath() string {
	return folder.path
}

func (folder *Folder) Exists(objectRelativePath string) (bool, error) {
	path := storage.JoinPath(folder.path, objectRelativePath)
	ctx := context.Background()
	blobClient := folder.containerClient.NewBlockBlobClient(path)
	_, err := blobClient.GetProperties(ctx, nil)
	var stgErr *azcore.ResponseError
	if err != nil && errors.As(err, &stgErr) && stgErr.ErrorCode == string(bloberror.BlobNotFound) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("get Azure object stats %q: %w", path, err)
	}
	return true, nil
}

func (folder *Folder) ListFolder() (objects []storage.Object, subFolders []storage.Folder, err error) {
	blobPager := folder.containerClient.NewListBlobsHierarchyPager("/", &container.ListBlobsHierarchyOptions{Prefix: &folder.path})
	for blobPager.More() {
		page, err := blobPager.NextPage(context.Background())
		if err != nil {
			return objects, subFolders, err
		}
		for _, blob := range page.Segment.BlobItems {
			objName := strings.TrimPrefix(*blob.Name, folder.path)
			updated := *blob.Properties.LastModified

			objects = append(objects, storage.NewLocalObject(objName, updated, *blob.Properties.ContentLength))
		}

		// Get subFolder names
		blobPrefixes := page.Segment.BlobPrefixes
		// add subFolders to the list of storage folders
		for _, blobPrefix := range blobPrefixes {
			subFolderPath := *blobPrefix.Name

			subFolders = append(subFolders, NewFolder(
				subFolderPath,
				folder.containerClient,
				folder.uploadStreamOptions,
				folder.timeout,
			))
		}
	}
	return objects, subFolders, err
}

func (folder *Folder) GetSubFolder(subFolderRelativePath string) storage.Folder {
	return NewFolder(
		storage.AddDelimiterToPath(storage.JoinPath(folder.path, subFolderRelativePath)),
		folder.containerClient,
		folder.uploadStreamOptions,
		folder.timeout)
}

func (folder *Folder) ReadObject(objectRelativePath string) (io.ReadCloser, error) {
	path := storage.JoinPath(folder.path, objectRelativePath)
	blobClient := folder.containerClient.NewBlockBlobClient(path)
	get, err := blobClient.DownloadStream(context.Background(), nil)
	if err != nil {
		var storageError *azcore.ResponseError
		errors.As(err, &storageError)
		if storageError.ErrorCode == string(bloberror.BlobNotFound) {
			return nil, storage.NewObjectNotFoundError(path)
		}
		return nil, fmt.Errorf("download blob %q: %w", path, err)
	}
	reader := get.Body
	return reader, nil
}

func (folder *Folder) PutObject(name string, content io.Reader) error {
	return folder.PutObjectWithContext(context.Background(), name, content)
}

func (folder *Folder) PutObjectWithContext(ctx context.Context, name string, content io.Reader) error {
	tracelog.DebugLogger.Printf("Put %v into %v\n", name, folder.path)
	// Upload content to a block blob using full path
	path := storage.JoinPath(folder.path, name)
	blobClient := folder.containerClient.NewBlockBlobClient(path)
	_, err := blobClient.UploadStream(ctx, content, &folder.uploadStreamOptions)
	if err != nil {
		return fmt.Errorf("upload blob %q: %w", path, err)
	}

	tracelog.DebugLogger.Printf("Put %v done\n", name)
	return nil
}

func (folder *Folder) CopyObject(srcPath string, dstPath string) error {
	var exists bool
	var err error
	if exists, err = folder.Exists(srcPath); !exists {
		if err == nil {
			return storage.NewObjectNotFoundError(srcPath)
		}
		return err
	}
	var srcClient, dstClient *blockblob.Client
	srcClient = folder.containerClient.NewBlockBlobClient(srcPath)
	dstClient = folder.containerClient.NewBlockBlobClient(dstPath)
	tireAccess := blob.AccessTierHot
	_, err = dstClient.StartCopyFromURL(context.Background(), srcClient.URL(),
		&blob.StartCopyFromURLOptions{Tier: &tireAccess})
	return err
}

func (folder *Folder) DeleteObjects(objectRelativePaths []string) error {
	for _, objectRelativePath := range objectRelativePaths {
		// Delete blob using blobClient obtained from full path to blob
		path := storage.JoinPath(folder.path, objectRelativePath)
		blobClient := folder.containerClient.NewBlockBlobClient(path)
		tracelog.DebugLogger.Printf("Delete %v\n", path)
		deleteType := azblob.DeleteSnapshotsOptionTypeInclude
		_, err := blobClient.Delete(context.Background(),
			&azblob.DeleteBlobOptions{DeleteSnapshots: &deleteType})
		var stgErr *azcore.ResponseError
		if err != nil && errors.As(err, &stgErr) && stgErr.ErrorCode == string(bloberror.BlobNotFound) {
			continue
		}
		if err != nil {
			return fmt.Errorf("delete object %q: %w", path, err)
		}
		// blob is deleted
	}
	return nil
}
