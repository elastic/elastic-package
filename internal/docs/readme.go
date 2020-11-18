package docs

import (
	"bytes"
	"html/template"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/pkg/errors"

	"github.com/elastic/elastic-package/internal/packages"
)

const ReadmeFile = "README.md"

// IsReadmeUpToDate function checks if the README file is up-to-date.
func IsReadmeUpToDate() (bool, error) {
	return false, nil
}

// UpdateReadme function updates the README file using ą defined template file. The function doesn't perform any action
// if the template file is not present.
func UpdateReadme() error {
	packageRoot, err := packages.MustFindPackageRoot()
	if err != nil {
		return errors.Wrap(err, "package root not found")
	}

	templatePath, found, err := findReadmeTemplatePath(packageRoot)
	if err != nil {
		return errors.Wrapf(err, "can't locate %s template file", ReadmeFile)
	}
	if !found {
		return nil // README file is static, can't be generated from the template file
	}

	manifest, err := packages.ReadPackageManifestForPackage(packageRoot)
	if err != nil {
		return errors.Wrapf(err, "reading package manifest failed (packageRoot: %s)", packageRoot)
	}

	rendered, err := renderReadme(manifest.Name, templatePath)
	if err != nil {
		return errors.Wrap(err, "rendering Readme failed")
	}

	err = writeReadme(templatePath, rendered)
	if err != nil {
		return errors.Wrapf(err, "writing %s file failed", ReadmeFile)
	}
	return nil
}

func findReadmeTemplatePath(packageRoot string) (string, bool, error) {
	templatePath := filepath.Join(packageRoot, "_dev", "build", "docs", ReadmeFile)
	_, err := os.Stat(templatePath)
	if err != nil && os.IsNotExist(err) {
		return "", false, nil // README.md file not found
	}
	if err != nil {
		return "", false, errors.Wrapf(err, "can't located the %s file", ReadmeFile)
	}
	return templatePath, true, nil
}

func renderReadme(packageName, templatePath string) ([]byte, error) {
	t := template.New(ReadmeFile)
	t, err := t.Funcs(template.FuncMap{
		"event": func(dataStreamName string) (string, error) {
			return renderSampleEvent(packageName, dataStreamName)
		},
		"fields": func(dataStreamName string) (string, error) {
			return renderExportedFields(packageName, dataStreamName)
		},
	}).ParseFiles(templatePath)
	if err != nil {
		return nil, errors.Wrapf(err, "parsing README template failed (path: %s)", templatePath)
	}

	var rendered bytes.Buffer
	err = t.Execute(&rendered, nil)
	if err != nil {
		return nil, errors.Wrap(err, "executing template failed")
	}
	return rendered.Bytes(), nil
}

func writeReadme(packageRoot string, content []byte) error {
	docsPath := filepath.Join(packageRoot, "docs")
	err := os.MkdirAll(docsPath, 0755)
	if err != nil {
		return errors.Wrapf(err, "mkdir failed (path: %s)", docsPath)
	}

	readmePath := filepath.Join(docsPath, ReadmeFile)
	err = ioutil.WriteFile(readmePath, content, 0644)
	if err != nil {
		return errors.Wrapf(err, "writing file failed (path: %s)", readmePath)
	}
	return nil
}
