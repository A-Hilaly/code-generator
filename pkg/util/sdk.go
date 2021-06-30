package util

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"gopkg.in/src-d/go-git.v4"
	"gopkg.in/src-d/go-git.v4/plumbing"
)

// FallBackFindServiceID reads through aws-sdk-go/models/apis/*/*/api-2.json
// Returns ServiceID (as newSuppliedAlias) if supplied service Alias matches with serviceID in api-2.json
// If not a match, return the supllied alias.
func FallBackFindServiceID(sdkDir, svcAlias string) (string, error) {
	basePath := filepath.Join(sdkDir, "models", "apis")
	var files []string
	err := filepath.Walk(basePath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		files = append(files, path)
		return nil
	})
	if err != nil {
		return svcAlias, err
	}
	for _, file := range files {
		if strings.Contains(file, "api-2.json") {
			f, err := os.Open(file)
			if err != nil {
				return svcAlias, err
			}
			defer f.Close()
			scanner := bufio.NewScanner(f)
			for scanner.Scan() {
				if strings.Contains(scanner.Text(), "serviceId") {
					getServiceID := strings.Split(scanner.Text(), ":")
					re := regexp.MustCompile(`[," \t]`)
					svcID := strings.ToLower(re.ReplaceAllString(getServiceID[1], ``))
					if svcAlias == svcID {
						getNewSvcAlias := strings.Split(file, string(os.PathSeparator))
						return getNewSvcAlias[len(getNewSvcAlias)-3], nil
					}
				}
			}
		}
	}
	return svcAlias, nil
}

// LoadRepository loads a repository from the local file system.
// TODO(a-hilaly): load repository into a memory filesystem (needs go1.16
// migration or use somethign like https://github.com/spf13/afero
func LoadRepository(path string) (*git.Repository, error) {
	return git.PlainOpen(path)
}

// CloneRepository clones a git repository into a given directory.
//
// Calling his function is equivalent to executing `git clone $repositoryURL $path`
func CloneRepository(ctx context.Context, path, repositoryURL string) error {
	_, err := git.PlainCloneContext(ctx, path, false, &git.CloneOptions{
		URL:      repositoryURL,
		Progress: nil,
		// Clone and fetch all tags
		Tags: git.AllTags,
	})
	return err
}

// FetchRepositoryTags fetches a repository remote tags.
//
// Calling this function is equivalent to executing `git -C $path fetch --all --tags`
func FetchRepositoryTags(ctx context.Context, path string) error {
	// PlainOpen will make the git commands run against the local
	// repository and directly make changes to it. So no need to
	// save/rewrite the refs
	repo, err := git.PlainOpen(path)
	if err != nil {
		return err
	}

	err = repo.FetchContext(ctx, &git.FetchOptions{
		Progress: nil,
		Tags:     git.AllTags,
	})
	// weirdly go-git returns a error "Already up to date" when all tags
	// are already fetched. We should ignore this error.
	if err == git.NoErrAlreadyUpToDate {
		return nil
	}
	return err
}

// getRepositoryTagRef returns the git reference (commit hash) of a given tag.
// NOTE: It is not possible to checkout a tag without knowing it's reference.
//
// Calling this function is equivalent to executing `git rev-list -n 1 $tagName`
func getRepositoryTagRef(repo *git.Repository, tagName string) (*plumbing.Reference, error) {
	tagRefs, err := repo.Tags()
	if err != nil {
		return nil, err
	}

	for {
		tagRef, err := tagRefs.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("error finding tag reference: %v", err)
		}
		if tagRef.Name().Short() == tagName {
			return tagRef, nil
		}
	}
	return nil, errors.New("tag reference not found")
}

// CheckoutRepositoryTag checkouts a repository tag by looking for the tag
// reference then calling the checkout function.
//
// Calling This function is equivalent to executing `git checkout tags/$tag`
func CheckoutRepositoryTag(repo *git.Repository, tag string) error {
	tagRef, err := getRepositoryTagRef(repo, tag)
	if err != nil {
		return err
	}
	wt, err := repo.Worktree()
	if err != nil {
		return err
	}
	err = wt.Checkout(&git.CheckoutOptions{
		// Checkout only take hashes or branch names.
		Hash: tagRef.Hash(),
	})
	return err
}
