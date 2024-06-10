package onprem

import (
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/kelseyhightower/envconfig"
	"github.com/sirupsen/logrus"
)

func NewFileDownloaderFromEnv(logger *logrus.Logger) (*FileDownloader, error) {
	config := &DownloaderConfig{}
	err := envconfig.Process("", config)
	if err != nil {
		return nil, err
	}
	return NewFileDownloader(logger, config.DownloadDir), nil

}

func NewFileDownloader(logger *logrus.Logger, downloadDir string) *FileDownloader {
	return &FileDownloader{
		logger:      logger,
		downloadDir: downloadDir,
	}
}

type DownloaderConfig struct {
	DownloadDir string `envconfig:"DOWNLOAD_DIRECTORY" required:"true"`
}

//go:generate mockgen -source=downloader.go -package=onprem -destination=mock_downloader.go
type IFileDownloader interface {
	DownloadFile(rawFileURL string) (string, error)
	Close()
}

type FileDownloader struct {
	logger      *logrus.Logger
	downloadDir string
}

// download from given URL, returns full path of downloaded file
func (d *FileDownloader) DownloadFile(rawFileURL string) (string, error) {
	logger := d.logger.WithField("url", rawFileURL)

	fileURL, err := url.Parse(rawFileURL)
	if err != nil {
		logger.WithError(err).Error("Failed to parse url")
		return "", err
	}
	path := fileURL.Path
	segments := strings.Split(path, "/")
	filename := filepath.Join(d.downloadDir, segments[len(segments)-1])

	logger = logger.WithField("filename", filename)
	logger.Info("Downloading file from url")

	client := http.Client{
		CheckRedirect: func(r *http.Request, via []*http.Request) error {
			r.URL.Opaque = r.URL.Path
			return nil
		},
	}

	resp, err := client.Get(rawFileURL)
	if err != nil {
		logger.WithError(err).Error("Failed to download file")
		return "", err
	}
	defer resp.Body.Close()

	file, err := os.Create(filename)
	if err != nil {
		logger.WithError(err).Error("Failed to create file for download")
		return "", err
	}
	defer file.Close()

	_, err = io.Copy(file, resp.Body)
	if err != nil {
		logger.WithError(err).Error("Failed to copy file")
		return "", err
	}

	return filename, nil
}

func (d *FileDownloader) Close() {
	os.RemoveAll(d.downloadDir)
}
