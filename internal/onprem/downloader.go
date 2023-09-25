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

	fileURL, err := url.Parse(rawFileURL)
	if err != nil {
		d.logger.WithFields(logrus.Fields{
			"err": err,
			"url": rawFileURL,
		}).Error("failed to parse url")
		return "", err
	}
	path := fileURL.Path
	segments := strings.Split(path, "/")
	filename := filepath.Join(d.downloadDir, segments[len(segments)-1])
	d.logger.WithFields(logrus.Fields{
		"filename": filename,
		"url":      rawFileURL,
	}).Info("downloading file from url")
	file, err := os.Create(filename)
	if err != nil {
		d.logger.WithFields(logrus.Fields{
			"err":  err,
			"file": filename,
		}).Error("failed to create file for download")
		return "", err
	}
	client := http.Client{
		CheckRedirect: func(r *http.Request, via []*http.Request) error {
			r.URL.Opaque = r.URL.Path
			return nil
		},
	}

	resp, err := client.Get(rawFileURL)
	if err != nil {
		d.logger.WithFields(logrus.Fields{
			"err": err,
			"url": rawFileURL,
		}).Error("failed to download file")
		return "", err
	}
	defer resp.Body.Close()

	_, err = io.Copy(file, resp.Body)

	if err != nil {
		d.logger.WithFields(logrus.Fields{
			"err":  err,
			"file": filename,
		}).Error("failed to copy file")
		return "", err
	}
	defer file.Close()
	return filename, nil
}

func (d *FileDownloader) Close() {
	os.RemoveAll(d.downloadDir)
}
