package onprem

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/sirupsen/logrus"
)

var _ = Describe("Download file", func() {
	var (
		downloader *FileDownloader
		logger     *logrus.Logger
	)
	BeforeEach(func() {
		logger = logrus.New()
		logger.Out = io.Discard
		tmpdir, err := os.MkdirTemp("", ".download-")
		Expect(err).To(BeNil())
		downloader = NewFileDownloader(logger, tmpdir)
	})
	AfterEach(func() {
		downloader.Close()
	})

	When("download a valid file", func() {
		It("should output a valid filename", func() {
			url := "https://example.com/foobar.tar.gz"
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// assert requests
				w.WriteHeader(http.StatusOK)
				_, err := w.Write([]byte(``))
				Expect(err).To(BeNil())
			}))
			defer server.Close()
			filename, err := downloader.DownloadFile(url)
			Expect(err).To(BeNil())
			_, err = os.Stat(filename)
			Expect(err).To(BeNil())
		})
	})
})
