package promote

import "github.com/pkg/errors"

// OpenPullRequestWithPromotedPackages method opens a PR against "base" branch with promoted packages.
// Head is the branch containing the changes that will be added to the base branch.
func OpenPullRequestWithPromotedPackages(head, base string, promotedPackages PackageRevisions) error {
	return errors.New("OpenPullRequestWithPromotedPackages: not implemented yet") // TODO
}

// OpenPullRequestWithRemovedPackages method opens a PR against "base" branch with removed packages.
// Head is the branch containing the changes that will be added to the base branch.
func OpenPullRequestWithRemovedPackages(head, base string, removedPackages PackageRevisions) error {
	return errors.New("OpenPullRequestWithRemovedPackages: not implemented yet") // TODO
}
