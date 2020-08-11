package promote

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"

	"github.com/pkg/errors"

	"github.com/elastic/elastic-package/internal/github"
)

type createPullRequestResponse struct {
	Number    int      `json:"number"`
	HTMLURL   string   `json:"html_url"`
	Assignees []string `json:"assignees"`
}

// OpenPullRequestWithRemovedPackages method opens a PR against "base" branch with removed packages.
// Head is the branch containing the changes that will be added to the base branch.
func OpenPullRequestWithRemovedPackages(username, head, base, sourceStage, destinationStage string, promotedPackages PackageRevisions) (string, error) {
	title := fmt.Sprintf("[%s] Remove packages from %s to %s", destinationStage, sourceStage, destinationStage)
	description := buildPullRequestRemoveDescription(base, head, promotedPackages)
	return openPullRequestWithPackages(username, head, base, title, description)
}

// OpenPullRequestWithPromotedPackages method opens a PR against "base" branch with promoted packages.
// Head is the branch containing the changes that will be added to the base branch.
func OpenPullRequestWithPromotedPackages(username, head, base, sourceStage, destinationStage string, promotedPackages PackageRevisions) (string, error) {
	title := fmt.Sprintf("[%s] Promote packages from %s to %s", destinationStage, sourceStage, destinationStage)
	description := buildPullRequestPromoteDescription(base, head, promotedPackages)
	return openPullRequestWithPackages(username, head, base, title, description)
}

func openPullRequestWithPackages(user, head, base, title, description string) (string, error) {
	authToken, err := github.AuthToken()
	if err != nil {
		return "", errors.Wrap(err, "reading auth token failed")
	}

	requestBody, err := buildPullRequestRequestBody(title, user, head, base, description)
	if err != nil {
		return "", errors.Wrap(err, "building request body failed")
	}

	request, err := http.NewRequest("POST", "https://api.github.com/repos/elastic/package-storage/pulls", bytes.NewReader(requestBody))
	if err != nil {
		return "", errors.Wrap(err, "creating new HTTP request failed")
	}

	request.Header.Add("Authorization", fmt.Sprintf("token %s", authToken))
	request.Header.Add("Content-Type", "application/json")
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return "", errors.Wrap(err, "making HTTP call failed")
	}

	if response.StatusCode < 200 || response.StatusCode > 299 {
		return "", fmt.Errorf("unexpected status code return while opening a pull request: %d", response.StatusCode)
	}

	var data createPullRequestResponse
	defer response.Body.Close()
	body, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return "", errors.Wrap(err, "can't read response body")
	}

	if err := json.Unmarshal(body, &data); err != nil {
		return "", errors.Wrap(err, "unmarshalling response failed")
	}

	err = updatePullRequestAssignee(data.Number, user)
	if err != nil {
		return "", errors.Wrapf(err, "updating assignees failed (pull request ID: %d, user: %s)", data.Number, user)
	}
	return data.HTMLURL, nil
}

func updatePullRequestAssignee(pullRequestID int, user string) error {
	authToken, err := github.AuthToken()
	if err != nil {
		return errors.Wrap(err, "reading auth token failed")
	}

	requestBody, err := buildPullRequestAssigneesRequestBody(user)
	if err != nil {
		return errors.Wrap(err, "building assignees request body failed")
	}

	request, err := http.NewRequest("POST", fmt.Sprintf("https://api.github.com/repos/elastic/package-storage/pulls/%d/requested_reviewers", pullRequestID),
		bytes.NewReader(requestBody))
	if err != nil {
		return errors.Wrap(err, "creating new HTTP request failed")
	}

	request.Header.Add("Authorization", fmt.Sprintf("token %s", authToken))
	request.Header.Add("Content-Type", "application/json")
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return errors.Wrap(err, "making HTTP call failed")
	}

	if response.StatusCode < 200 || response.StatusCode > 299 {
		return fmt.Errorf("unexpected status code return while opening a pull request: %d", response.StatusCode)
	}

	var data createPullRequestResponse
	defer response.Body.Close()
	body, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return errors.Wrap(err, "can't read response body")
	}

	if err := json.Unmarshal(body, &data); err != nil {
		return errors.Wrap(err, "unmarshalling response failed")
	}

	if len(data.Assignees) == 0 {
		return errors.New("no users assigned to the pull request")
	}
	return nil
}

func buildPullRequestRemoveDescription(sourceStage, destinationStage string, revisions PackageRevisions) string {
	var builder strings.Builder
	builder.WriteString(fmt.Sprintf(`This PR removes packages from "%s" to "%s".\n`, sourceStage, destinationStage))
	builder.WriteString("\n")
	builder.WriteString("Removed packages:")
	for _, revision := range revisions {
		builder.WriteString(fmt.Sprintf("* %s-%s\n", revision.Name, revision.Version))
	}
	return builder.String()
}

func buildPullRequestPromoteDescription(sourceStage, destinationStage string, revisions PackageRevisions) string {
	var builder strings.Builder
	builder.WriteString(fmt.Sprintf(`This PR promotes packages from "%s" to "%s".\n`, sourceStage, destinationStage))
	builder.WriteString("\n")
	builder.WriteString("Promoted packages:")
	for _, revision := range revisions {
		builder.WriteString(fmt.Sprintf("* %s-%s\n", revision.Name, revision.Version))
	}
	return builder.String()
}

func buildPullRequestRequestBody(title, user, head, base, description string) ([]byte, error) {
	requestBody := map[string]interface{}{
		"title":                 title,
		"head":                  fmt.Sprintf("%s:%s", user, head),
		"base":                  base,
		"body":                  description,
		"maintainer_can_modify": true,
	}

	m, err := json.Marshal(&requestBody)
	if err != nil {
		return nil, errors.Wrap(err, "marshaling request body failed")
	}
	return m, nil
}

func buildPullRequestAssigneesRequestBody(assignee string) ([]byte, error) {
	requestBody := map[string]interface{}{
		"assignees": []string{assignee},
	}

	m, err := json.Marshal(&requestBody)
	if err != nil {
		return nil, errors.Wrap(err, "marshaling request body failed")
	}
	return m, nil
}
