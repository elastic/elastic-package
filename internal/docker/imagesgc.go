// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package docker

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"

	"golang.org/x/sys/execabs"

	"github.com/elastic/elastic-package/internal/common"
)

type ImagesGCConfig struct {
	// Enabled controls if the garbage collector performs any deletion.
	// When set to false, it does not remove anything, but it still keeps track of images.
	Enabled bool `yaml:"enabled"`

	// MaxTotalSize removes images only after the total size of images is beyond this number, zero to disable.
	MaxTotalSize common.ByteSize `yaml:"max_total_size"`

	// MaxUnused removes only images that haven't been used for the specified time.
	MaxUnused time.Duration `yaml:"max_unused"`
}

func DefaultImagesGCConfig() ImagesGCConfig {
	return ImagesGCConfig{
		Enabled:      false,
		MaxTotalSize: 0,
		MaxUnused:    4 * 7 * 24 * time.Hour, // 4 weeks
	}
}

type ImagesGC struct {
	// path contains the path to the GC cache when read from disk.
	path string

	// images contains the entries of the tracked docker images.
	images []gcEntry

	// clock returns the current time.
	clock func() time.Time

	// client implements a docker client.
	client imagesGCClient

	ImagesGCConfig
}

type imagesGCClient interface {
	// ListImages should list local images in the same format as "docker-compose images".
	ListImages() ([]string, error)

	// RemoveImage should try to remove an image. If the image is busy, it returns ErrBusyImage.
	RemoveImage(image string) error

	// TotalImagesSize returns the total size of the local images.
	TotalImagesSize() (common.ByteSize, error)
}

var ErrBusyImage = errors.New("image is being used")

func defaultImagesGC() ImagesGC {
	return ImagesGC{
		clock:          time.Now,
		client:         defaultImagesGCClient(),
		ImagesGCConfig: DefaultImagesGCConfig(),
	}
}

type gcEntry struct {
	ImageTag string    `json:"image_tag"`
	LastUsed time.Time `json:"last_used"`
}

func NewImagesGCFromCacheDir(cacheDir string) (*ImagesGC, error) {
	return NewImagesGC(filepath.Join(cacheDir, "docker-images-gc.json"))
}

func NewImagesGC(path string) (*ImagesGC, error) {
	d, err := os.Open(path)
	if errors.Is(err, fs.ErrNotExist) {
		gc := defaultImagesGC()
		gc.path = path
		return &gc, nil
	}
	if err != nil {
		return nil, err
	}
	defer d.Close()
	dec := json.NewDecoder(d)
	var entries []gcEntry
	if err := dec.Decode(&entries); err != nil {
		return nil, err
	}

	gc := defaultImagesGC()
	gc.path = path
	gc.images = entries
	return &gc, nil
}

// Persist saves the list of images to disk.
func (gc *ImagesGC) Persist() error {
	if gc.path == "" {
		return errors.New("GC list was not created with a path")
	}
	if len(gc.images) == 0 {
		return nil
	}

	err := os.MkdirAll(filepath.Dir(gc.path), 0755)
	if err != nil && !errors.Is(err, os.ErrExist) {
		return err
	}

	d, err := json.Marshal(gc.images)
	if err != nil {
		return fmt.Errorf("failed to encode list of images: %w", err)
	}
	return os.WriteFile(gc.path, d, 0644)
}

// Track images before they are downloaded. Images already present are ignored if they are not already tracked.
func (gc *ImagesGC) Track(images ...string) error {
	present, err := gc.client.ListImages()
	if err != nil {
		return fmt.Errorf("failed to list local Docker images: %w", err)
	}

	now := gc.clock()
	for _, image := range images {
		currentIndex := slices.IndexFunc(gc.images, func(i gcEntry) bool { return i.ImageTag == image })
		if currentIndex >= 0 {
			// Already tracked, update last used time.
			gc.images[currentIndex].LastUsed = now
			continue
		}

		if slices.Contains(present, image) {
			// Don't track images already present.
			continue
		}

		gc.images = append(gc.images, gcEntry{
			ImageTag: image,
			LastUsed: now,
		})
	}

	return nil
}

// Run runs garbage collection, it removes images according to the rules.
func (gc *ImagesGC) Run() error {
	if !gc.Enabled {
		return nil
	}

	present, err := gc.client.ListImages()
	if err != nil {
		return fmt.Errorf("failed to list local Docker images: %w", err)
	}

	sizeOk := gc.MaxTotalSize == 0
	maxUnused := gc.clock().Add(-gc.MaxUnused)
	var images []gcEntry
	slices.SortFunc(gc.images, func(a, b gcEntry) int { return a.LastUsed.Compare(b.LastUsed) })
	for i, image := range gc.images {
		if !sizeOk {
			totalSize, err := gc.client.TotalImagesSize()
			if err != nil {
				gc.images = append(images, gc.images[i:]...)
				return fmt.Errorf("cannot get total images size: %w", err)
			}
			sizeOk = totalSize <= gc.MaxTotalSize
		}
		if !sizeOk || image.LastUsed.Before(maxUnused) {
			if slices.Contains(present, image.ImageTag) {
				err := gc.client.RemoveImage(image.ImageTag)
				if errors.Is(err, ErrBusyImage) {
					continue
				}
				if err != nil {
					gc.images = append(images, gc.images[i:]...)
					return fmt.Errorf("cannot remove image %s: %w", image.ImageTag, err)
				}
				continue
			}
		}

		images = append(images, image)
	}

	gc.images = images
	return nil
}

type localImagesGCClient struct {
}

func defaultImagesGCClient() localImagesGCClient {
	return localImagesGCClient{}
}

func (localImagesGCClient) ListImages() ([]string, error) {
	cmd := execabs.Command("docker", "image", "list", "--format=json")
	errOutput := new(bytes.Buffer)
	cmd.Stderr = errOutput

	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("docker image list failed (stderr=%q): %w", errOutput, err)
	}

	var line struct {
		Repository string `json:"Repository"`
		Tag        string `json:"Tag"`
	}
	var result []string
	dec := json.NewDecoder(bytes.NewReader(output))
	for dec.More() {
		err = dec.Decode(&line)
		if err != nil {
			return nil, fmt.Errorf("cannot decode output of docker image list: %w", err)
		}
		result = append(result, line.Repository+":"+line.Tag)
	}

	return result, nil
}

var removeConflictRegexp = regexp.MustCompile("container [^/s]+ is using its referenced image [^/s]+")

func (localImagesGCClient) RemoveImage(image string) error {
	cmd := execabs.Command("docker", "image", "rm", image)
	errOutput := new(bytes.Buffer)
	cmd.Stderr = errOutput

	err := cmd.Run()
	if err != nil {
		errMessage := errOutput.String()
		if removeConflictRegexp.MatchString(errMessage) {
			return ErrBusyImage
		}
		return fmt.Errorf("%w: %s", err, strings.TrimPrefix(errMessage, "Error response from daemon: "))
	}

	return nil
}

func (localImagesGCClient) TotalImagesSize() (common.ByteSize, error) {
	cmd := execabs.Command("docker", "system", "df", "--format=json")
	errOutput := new(bytes.Buffer)
	cmd.Stderr = errOutput

	output, err := cmd.Output()
	if err != nil {
		return 0, fmt.Errorf("docker system df failed (stderr=%q): %w", errOutput, err)
	}

	var df struct {
		Type string          `json:"Type"`
		Size common.ByteSize `json:"Size"`
	}
	dec := json.NewDecoder(bytes.NewReader(output))
	for dec.More() {
		err = dec.Decode(&df)
		if err != nil {
			return 0, fmt.Errorf("cannot decode output of docker system df: %w", err)
		}
		if df.Type == "Images" {
			return df.Size, nil
		}
	}

	return 0, fmt.Errorf("total images size not found")
}
