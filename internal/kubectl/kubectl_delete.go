package kubectl

// Delete function removes resources from the Kubernetes cluster based on provided definitions.
func Delete(definitionPaths ...string) error {
	_, err := modifyKubernetesResources("delete", definitionPaths...)
	return err
}