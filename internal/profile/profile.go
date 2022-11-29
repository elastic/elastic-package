package profile

import (
	"embed"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/Masterminds/semver"
	"github.com/elastic/elastic-package/internal/configuration/locations"
	"github.com/elastic/go-resource"
)

const (
	// PackageProfileMetaFile is the filename of the profile metadata file
	PackageProfileMetaFile = "profile.json"

	// SnapshotFile is the docker-compose snapshot.yml file name.
	SnapshotFile = "snapshot.yml"

	// ElasticsearchConfigFile is the elasticsearch config file.
	ElasticsearchConfigFile = "elasticsearch.yml"

	// KibanaConfigFile is the kibana config file.
	KibanaConfigFile = "kibana.yml"

	// PackageRegistryConfigFile is the config file for the Elastic Package registry
	PackageRegistryConfigFile = "package-registry.yml"

	// PackageRegistryBaseImage is the base Docker image of the Elastic Package Registry.
	PackageRegistryBaseImage = "docker.elastic.co/package-registry/distribution:snapshot"

	// ElasticAgentEnvFile is the elastic agent environment variables file.
	ElasticAgentEnvFile = "elastic-agent.env"

	// DefaultProfile is the name of the default profile.
	DefaultProfile = "default"

	profileStackPath = "stack"
)

//go:embed _static
var static embed.FS

var (
	templateFuncs = template.FuncMap{
		"semverLessThan": semverLessThan,
	}
	staticSource     = resource.NewSourceFS(static).WithTemplateFuncs(templateFuncs)
	profileResources = []resource.Resource{
		&resource.File{
			Provider: "profile-file",
			Path:     PackageProfileMetaFile,
			Content:  profileMetadataContent,
		},
		&resource.File{
			Provider: "stack-file",
			Path:     "Dockerfile.package-registry",
			Content:  staticSource.Template("_static/Dockerfile.package-registry.tmpl"),
		},
		&resource.File{
			Provider: "stack-file",
			Path:     SnapshotFile,
			Content:  staticSource.File("_static/docker-compose-stack.yml"),
		},
		&resource.File{
			Provider: "stack-file",
			Path:     ElasticsearchConfigFile,
			Content:  staticSource.Template("_static/elasticsearch.yml.tmpl"),
		},
		&resource.File{
			Provider: "stack-file",
			Path:     KibanaConfigFile,
			Content:  staticSource.Template("_static/kibana.yml.tmpl"),
		},
		&resource.File{
			Provider: "stack-file",
			Path:     PackageRegistryConfigFile,
			Content:  staticSource.File("_static/package-registry.yml"),
		},
		&resource.File{
			Provider: "stack-file",
			Path:     ElasticAgentEnvFile,
			Content:  staticSource.Template("_static/elastic-agent.env.tmpl"),
		},
	}
)

type Options struct {
	PackagePath       string
	Name              string
	FromProfile       string
	OverwriteExisting bool
}

func CreateProfile(options Options) error {
	if options.PackagePath == "" {
		loc, err := locations.NewLocationManager()
		if err != nil {
			return fmt.Errorf("error finding profile dir location: %w", err)
		}
		options.PackagePath = loc.ProfileDir()
	}

	// If they're creating from Default, assume they want the actual default, and
	// not whatever is currently inside default.
	if from := options.FromProfile; from != "" && from != DefaultProfile {
		return createProfileFrom(options)
	}

	resources, err := initProfileResources(options)
	if err != nil {
		return err
	}

	return createProfile(options, resources)
}

func initProfileResources(options Options) ([]resource.Resource, error) {
	profileName := options.Name
	if profileName == "" {
		profileName = DefaultProfile
	}
	profileDir := filepath.Join(options.PackagePath, profileName)

	resources := append([]resource.Resource{}, profileResources...)

	certResources, err := initTLSCertificates("profile-file", profileDir)
	if err != nil {
		return nil, fmt.Errorf("failed to create TLS files: %w", err)
	}
	resources = append(resources, certResources...)

	return resources, nil
}

func createProfile(options Options, resources []resource.Resource) error {
	stackVersion := "8.1.0" // TODO: Parameterize this.
	fmt.Printf("%+v\n", options)
	fmt.Printf("%+v\n", resources)

	profileName := options.Name
	if profileName == "" {
		profileName = DefaultProfile
	}
	profileDir := filepath.Join(options.PackagePath, profileName)
	stackDir := filepath.Join(options.PackagePath, profileName, profileStackPath)

	resourceManager := resource.NewManager()
	resourceManager.AddFacter(resource.StaticFacter{
		"profile_name": profileName,
		"profile_path": profileDir,

		"registry_base_image": PackageRegistryBaseImage,

		"elasticsearch_version": stackVersion,
		"kibana_version":        stackVersion,
		"agent_version":         stackVersion,
	})

	os.MkdirAll(stackDir, 0755)
	resourceManager.RegisterProvider("profile-file", &resource.FileProvider{
		Prefix: profileDir,
	})
	resourceManager.RegisterProvider("stack-file", &resource.FileProvider{
		Prefix: stackDir,
	})

	results, err := resourceManager.Apply(resources)
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

func createProfileFrom(options Options) error {
	from, err := LoadProfile(options.FromProfile)
	if err != nil {
		return fmt.Errorf("failed to load profile to copy %q: %w", options.FromProfile, err)
	}

	return createProfile(options, from.resources)
}

// Profile manages a a given user config profile
type Profile struct {
	// ProfilePath is the absolute path to the profile
	ProfilePath      string
	ProfileStackPath string
	ProfileName      string

	resources []resource.Resource
}

// ErrNotAProfile is returned in cases where we don't have a valid profile directory
var ErrNotAProfile = errors.New("not a profile")

// FetchPath returns an absolute path to the given file
func (profile Profile) FetchPath(name string) string {
	for _, r := range profile.resources {
		file, ok := r.(*resource.File)
		if !ok {
			continue
		}

		if file.Path != name {
			continue
		}

		return filepath.Join(profile.ProfileStackPath, file.Path)
	}
	panic(fmt.Sprintf("%q profile file is not defined", name))
}

// ComposeEnvVars returns a list of environment variables that can be passed
// to docker-compose for the sake of filling out paths and names in the snapshot.yml file.
func (profile Profile) ComposeEnvVars() []string {
	return []string{
		fmt.Sprintf("PROFILE_NAME=%s", profile.ProfileName),
		fmt.Sprintf("STACK_PATH=%s", profile.ProfileStackPath),
	}
}

// DeleteProfile deletes a profile from the default elastic-package config dir
func DeleteProfile(profileName string) error {
	if profileName == DefaultProfile {
		return errors.New("cannot remove default profile")
	}

	loc, err := locations.NewLocationManager()
	if err != nil {
		return fmt.Errorf("error finding stack dir location: %w", err)
	}

	pathToDelete := filepath.Join(loc.ProfileDir(), profileName)
	return os.RemoveAll(pathToDelete)
}

// FetchAllProfiles returns a list of profile values
func FetchAllProfiles(elasticPackagePath string) ([]Metadata, error) {
	dirList, err := os.ReadDir(elasticPackagePath)
	if err != nil {
		return []Metadata{}, fmt.Errorf("error reading from directory %s: %w", elasticPackagePath, err)
	}

	var profiles []Metadata
	// TODO: this should read a profile.json file or something like that
	for _, item := range dirList {
		if !item.IsDir() {
			continue
		}
		profile, err := loadProfile(elasticPackagePath, item.Name())
		if errors.Is(err, ErrNotAProfile) {
			continue
		}
		if err != nil {
			return profiles, fmt.Errorf("error loading profile %s: %w", item.Name(), err)
		}
		metadata, err := loadProfileMetadata(filepath.Join(profile.ProfilePath, PackageProfileMetaFile))
		if err != nil {
			return profiles, fmt.Errorf("error reading profile metadata: %w", err)
		}
		profiles = append(profiles, metadata)
	}
	return profiles, nil
}

// LoadProfile loads an existing profile from the default elastic-package config dir.
func LoadProfile(profileName string) (*Profile, error) {
	loc, err := locations.NewLocationManager()
	if err != nil {
		return nil, fmt.Errorf("error finding stack dir location: %w", err)
	}

	return loadProfile(loc.ProfileDir(), profileName)
}

// loadProfile loads an existing profile
func loadProfile(elasticPackagePath string, profileName string) (*Profile, error) {
	profilePath := filepath.Join(elasticPackagePath, profileName)

	isValid, err := isProfileDir(profilePath)
	if err != nil {
		return nil, fmt.Errorf("error checking profile %q: %w", profileName, err)
	}
	if !isValid {
		return nil, ErrNotAProfile
	}

	resources := append([]resource.Resource{}, profileResources...)

	certResources, err := initTLSCertificates("profile-file", profilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to create TLS files: %w", err)
	}
	resources = append(resources, certResources...)

	profile := Profile{
		ProfileName:      profileName,
		ProfilePath:      profilePath,
		ProfileStackPath: filepath.Join(profilePath, profileStackPath),
		resources:        resources,
	}

	return &profile, nil
}

// isProfileDir checks to see if the given path points to a valid profile
func isProfileDir(path string) (bool, error) {
	metaPath := filepath.Join(path, string(PackageProfileMetaFile))
	_, err := os.Stat(metaPath)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("error stat: %s: %w", metaPath, err)
	}
	return true, nil
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
