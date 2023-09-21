package onprem

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/kelseyhightower/envconfig"
	"github.com/openshift-assisted/assisted-events-streams/internal/types"
	"github.com/sirupsen/logrus"
)

type EventTypeMatch struct {
	glob         string
	eventName    string
	clusterIDKey string
}

//go:generate mockgen -source=event_extractor.go -package=onprem -destination=mock_event_extractor.go
type IEventExtractor interface {
	ExtractEvents(tarFilename string) (chan types.EventEnvelope, error)
}
type EventExtractor struct {
	logger           *logrus.Logger
	workDir          string
	eventsBufferSize int
}
type ExtractorConfig struct {
	WorkDir string `envconfig:"WORK_DIRECTORY" required:"true"`
}

func NewEventExtractorFromEnv(logger *logrus.Logger, channelsConfig *ChannelsConfig) (*EventExtractor, error) {
	config := &ExtractorConfig{}
	err := envconfig.Process("", config)
	if err != nil {
		return nil, err
	}

	return &EventExtractor{
		logger:           logger,
		workDir:          config.WorkDir,
		eventsBufferSize: channelsConfig.EventChannelBufferSize,
	}, nil
}

func (e *EventExtractor) ExtractEvents(tarFilename string) (chan types.EventEnvelope, error) {
	eventChannel := make(chan types.EventEnvelope, e.eventsBufferSize)
	tmpdir, err := ioutil.TempDir("", ".untar-")
	if err != nil {
		e.logger.WithFields(logrus.Fields{
			"err":  err,
			"file": tarFilename,
		}).Error("failed to create tmpdir")
		return eventChannel, err
	}
	defer os.RemoveAll(tmpdir)

	e.logger.WithFields(logrus.Fields{
		"tmpdir": tmpdir,
		"file":   tarFilename,
	}).Info("untarring file into tmpdir")
	e.untarFile(tarFilename, tmpdir)

	e.logger.WithFields(logrus.Fields{
		"directory": tmpdir,
	}).Info("extracting events from files in directory")
	e.extractEvents(tmpdir, eventChannel)
	return eventChannel, nil
}

// untar target tarball into tmpdir
func (e *EventExtractor) untarFile(filename string, tmpdir string) {
	gzipfile, err := os.Open(filename)
	if err != nil {
		e.logger.WithFields(logrus.Fields{
			"err":  err,
			"file": filename,
		}).Error("failed to open file")
		return
	}
	reader, err := gzip.NewReader(gzipfile)
	if err != nil {
		e.logger.WithFields(logrus.Fields{
			"err":  err,
			"file": filename,
		}).Error("failed to unzip file")
		return
	}
	defer reader.Close()

	if err != nil {
		e.logger.WithFields(logrus.Fields{
			"err": err,
		}).Error("error creating temp directory")
	}
	tarReader := tar.NewReader(reader)
	for {
		if !e.readTarLine(tarReader, tmpdir) {
			break
		}
	}
}

func (e *EventExtractor) readTarLine(tarReader *tar.Reader, tmpdir string) bool {
	header, err := tarReader.Next()
	if err == io.EOF {
		return false
	} else if err != nil {
		e.logger.WithFields(logrus.Fields{
			"err": err,
		}).Error("error while reading tar file")
		return false
	}

	path := filepath.Join(tmpdir, header.Name)
	info := header.FileInfo()
	if info.IsDir() {
		if err = os.MkdirAll(path, info.Mode()); err != nil {
			e.logger.WithFields(logrus.Fields{
				"dir": path,
				"err": err,
			}).Error("error while creating directory")
			return true
		}
	}
	dir := filepath.Dir(path)
	if err = os.MkdirAll(dir, 0744); err != nil {
		e.logger.WithFields(logrus.Fields{
			"err":      err,
			"filename": path,
			"dir":      dir,
		}).Error("error creating directory for file")
		return true
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, info.Mode())
	if err != nil {
		e.logger.WithFields(logrus.Fields{
			"err":  err,
			"dst":  path,
			"mode": info.Mode(),
		}).Error("error creating file")
		return true
	}
	defer file.Close()
	_, err = io.Copy(file, tarReader)
	if err != nil {
		e.logger.WithFields(logrus.Fields{
			"err": err,
			"dst": path,
		}).Error("error copying file")
		return true
	}
	return true
}

// extract events from given target tmpdir (where extracted files would be)
func (e *EventExtractor) extractEvents(tmpdir string, eventChannel chan types.EventEnvelope) {
	eventTypesMatches := []EventTypeMatch{
		{
			glob:         "/*/events/cluster.json",
			eventName:    "ClusterState",
			clusterIDKey: "id",
		},
		{
			glob:         "/*/events/hosts.json",
			eventName:    "HostState",
			clusterIDKey: "cluster_id",
		},
		{
			glob:         "/*/events/infraenv.json",
			eventName:    "InfraEnv",
			clusterIDKey: "cluster_id",
		},
		{
			glob:         "/*/events.json",
			eventName:    "Event",
			clusterIDKey: "cluster_id",
		},
	}

	for _, eventTypeMatch := range eventTypesMatches {
		e.extractEventsForEventType(eventTypeMatch, tmpdir, eventChannel)
	}
	close(eventChannel)
}

func (e *EventExtractor) extractEventsForEventType(eventTypeMatch EventTypeMatch, tmpdir string, eventChannel chan types.EventEnvelope) {
	files, err := filepath.Glob(fmt.Sprintf("%s%s", tmpdir, eventTypeMatch.glob))
	if err != nil {
		e.logger.WithFields(logrus.Fields{
			"err":    err,
			"target": tmpdir,
		}).Error("error globbing files")
		return
	}
	e.logger.WithFields(logrus.Fields{
		"files": files,
		"glob":  eventTypeMatch.glob,
	}).Info("extracting events from files in directory")

	for _, jFile := range files {
		resources, err := e.getResourcesFromFile(jFile)
		if err != nil {
			e.logger.WithFields(logrus.Fields{
				"err":  err,
				"file": jFile,
			}).Error("Error getting resources from json file")
		}
		e.transformResourcesToEvents(eventTypeMatch, resources, eventChannel)
	}
}

func (e *EventExtractor) transformResourcesToEvents(eventTypeMatch EventTypeMatch, resources []map[string]interface{}, eventChannel chan types.EventEnvelope) {
	for _, resource := range resources {
		clusterID, ok := resource[eventTypeMatch.clusterIDKey]
		if !ok {
			e.logger.WithFields(logrus.Fields{
				"resource":       resource,
				"cluster_id_key": eventTypeMatch.clusterIDKey,
			}).Warning("could not extract cluster_id from resource")
			return
		}
		if eventTypeMatch.eventName == "ClusterState" {
			resource["onprem"] = true
			// delete hosts, as message could be too big
			// hosts are covered by the HostState event type
			delete(resource, "hosts")
		}
		event := types.Event{
			Name:    eventTypeMatch.eventName,
			Payload: resource,
		}
		e.logger.WithFields(logrus.Fields{
			"cluster_id": clusterID,
			"event_type": event.Name,
		}).Info("generating event for cluster")
		cID, ok := clusterID.(string)
		if !ok {
			e.logger.WithFields(logrus.Fields{
				"cluster_id":     clusterID,
				"cluster_id_key": eventTypeMatch.clusterIDKey,
			}).Warning("cluster_id not a string")
			return
		}

		e.logger.WithFields(logrus.Fields{
			"event":      event,
			"cluster_id": cID,
		}).Debug("generated event")
		eventChannel <- types.EventEnvelope{
			Key:   []byte(cID),
			Event: event,
		}
	}

}

func (e *EventExtractor) getResourcesFromFile(filename string) ([]map[string]interface{}, error) {
	resources := []map[string]interface{}{}
	jsonFile, err := os.Open(filename)
	if err != nil {
		e.logger.WithFields(logrus.Fields{
			"file": filename,
			"err":  err,
		}).Error("Error opening file")
		return []map[string]interface{}{}, err
	}
	defer jsonFile.Close()
	byteValue, _ := ioutil.ReadAll(jsonFile)
	err = json.Unmarshal(byteValue, &resources)
	if err != nil {
		e.logger.WithFields(logrus.Fields{
			"err": err,
		}).Warning("Error decoding json file for resource list, trying resource")
		resource := map[string]interface{}{}
		err = json.Unmarshal(byteValue, &resource)
		if err != nil {
			return []map[string]interface{}{}, err
		}
		resources = append(resources, resource)
	}
	return resources, nil
}
