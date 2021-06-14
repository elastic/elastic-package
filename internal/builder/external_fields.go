package builder

import (
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/elastic/go-ucfg"
	"github.com/elastic/go-ucfg/yaml"
	"github.com/pkg/errors"

	"github.com/elastic/elastic-package/internal/logger"
)

type buildManifest struct {
	Dependencies struct {
		ECS struct {
			Reference string `config:"reference"`
		} `config:"ecs"`
	} `config:"dependencies"`
}

func (bm *buildManifest) hasDependencies() bool {
	return bm.Dependencies.ECS.Reference != ""
}

func resolveExternalFields(packageRoot, destinationDir string) error {
	bm, ok, err := readBuildManifest(packageRoot)
	if err != nil {
		return errors.Wrap(err, "can't read build manifest")
	}
	if !ok {
		logger.Debugf("Build manifest hasn't been defined for the package")
		return nil
	}
	if !bm.hasDependencies() {
		logger.Debugf("Package doesn't have any external dependencies defined")
		return nil
	}

	logger.Debugf("Package has external dependencies defined")

	// TODO Initialize FieldManager with dependencies

	fieldsFile, err := filepath.Glob(filepath.Join(destinationDir, "data_stream", "*", "fields", "*"))
	if err != nil {
		return err
	}
	for _, file := range fieldsFile {
		/*data*/ _, err := ioutil.ReadFile(file)
		if err != nil {
			return err
		}

		// TODO Load fields file into structure

		var resolvable bool // TODO check if there are external definitions
		var output []byte

		if resolvable {
			// TODO Resolve field dependencies

			err = ioutil.WriteFile(file, output, 0644)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func readBuildManifest(packageRoot string) (*buildManifest, bool, error) {
	path := filepath.Join(packageRoot, "_dev", "build", "build.yml")
	cfg, err := yaml.NewConfigWithFile(path, ucfg.PathSep("."))
	if errors.Is(err, os.ErrNotExist) {
		return nil, false, nil // ignore not found errors
	}
	if err != nil {
		return nil, false, errors.Wrapf(err, "reading file failed (path: %s)", path)
	}

	var bm buildManifest
	err = cfg.Unpack(&bm)
	if err != nil {
		return nil, true, errors.Wrapf(err, "unpacking build manifest failed (path: %s)", path)
	}
	return &bm, true, nil
}
