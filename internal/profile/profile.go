package profile

import (
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/Masterminds/semver"
	"github.com/elastic/go-resource"
)

// PackageRegistryBaseImage is the base Docker image of the Elastic Package Registry.
const PackageRegistryBaseImage = "docker.elastic.co/package-registry/distribution:snapshot"

//go:embed _static
var static embed.FS

var (
	templateFuncs = template.FuncMap{
		"semverLessThan": semverLessThan,
	}
	staticSource     = resource.NewSourceFS(static).WithTemplateFuncs(templateFuncs)
	profileResources = []resource.Resource{
		&resource.File{
			Provider: "stack-file",
			Path:     "Dockerfile.package-registry",
			Content:  staticSource.Template("_static/Dockerfile.package-registry.tmpl"),
		},
		&resource.File{
			Provider: "stack-file",
			Path:     "snapshot.yml",
			Content:  staticSource.File("_static/docker-compose-stack.yml"),
		},
		&resource.File{
			Provider: "stack-file",
			Path:     "elasticsearch.yml",
			Content:  staticSource.Template("_static/elasticsearch.yml.tmpl"),
		},
		&resource.File{
			Provider: "stack-file",
			Path:     "kibana.yml",
			Content:  staticSource.Template("_static/kibana.yml.tmpl"),
		},
		&resource.File{
			Provider: "stack-file",
			Path:     "package_registry.yml",
			Content:  staticSource.File("_static/package_registry.yml"),
		},
		&resource.File{
			Provider: "stack-file",
			Path:     "elastic-agent.env",
			Content:  staticSource.Template("_static/elastic-agent.env.tmpl"),
		},
	}
)

func CreateProfile(path string) error {
	stackVersion := "8.1.0" // TODO: Parameterize this.

	resourceManager := resource.NewManager()
	resourceManager.AddFacter(resource.StaticFacter{
		"registry_base_image": PackageRegistryBaseImage,

		"elasticsearch_version": stackVersion,
		"kibana_version":        stackVersion,
		"agent_version":         stackVersion,
	})

	stackDir := filepath.Join(path, "stack")
	os.MkdirAll(stackDir, 0755)
	resourceManager.RegisterProvider("stack-file", &resource.FileProvider{
		Prefix: stackDir,
	})

	results, err := resourceManager.Apply(profileResources)
	if err != nil {
		var errors []string
		for _, result := range results {
			if err := result.Err(); err != nil {
				errors = append(errors, err.Error())
			}
		}
		return fmt.Errorf("%w: %s", err, strings.Join(errors, ", "))
	}
	return nil
}

func semverLessThan(a, b string) (bool, error) {
	sa, err := semver.NewVersion(a)
	if err != nil {
		return false, err
	}
	sb, err := semver.NewVersion(b)
	if err != nil {
		return false, err
	}

	return sa.LessThan(sb), nil
}
