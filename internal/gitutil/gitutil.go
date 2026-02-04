package gitutil

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	git "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/transport"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
)

type RefType int

const (
	RefAuto RefType = iota
	RefBranch
	RefTag
	RefCommit
)

type Ref struct {
	Name string
	Type RefType
}

type Auth struct {
	TokenEnv    string
	UsernameEnv string
}

// FetchFile clones a repository, checks out the requested ref, and returns the contents of filePath.
// It first tries without auth to avoid prompting on public repos, then retries with env-provided
// credentials if available.
func FetchFile(repoURL string, ref Ref, filePath string, auth Auth) ([]byte, error) {
	creds := auth.credentials()
	attempts := []*http.BasicAuth{nil}
	if creds != nil {
		attempts = append(attempts, creds)
	}

	var lastErr error
	for _, attempt := range attempts {
		b, err := cloneAndRead(repoURL, ref, filePath, attempt)
		if err == nil {
			return b, nil
		}
		lastErr = err
		// Only retry with auth if the failure looks auth-related and we have creds.
		if attempt == nil && creds != nil && isAuthError(err) {
			continue
		}
		if attempt == nil && creds != nil {
			// Non-auth failure; retrying with creds is unlikely to help.
			break
		}
	}
	return nil, lastErr
}

func cloneAndRead(repoURL string, ref Ref, filePath string, basicAuth *http.BasicAuth) ([]byte, error) {
	tmpDir, err := os.MkdirTemp("", "yamldiff_repo_*")
	if err != nil {
		return nil, err
	}
	// Clean up temp dir after reading file
	defer func() { _ = os.RemoveAll(tmpDir) }()

	cloneOpts := &git.CloneOptions{URL: repoURL}
	if basicAuth != nil {
		cloneOpts.Auth = basicAuth
	}
	// shallow clone default branch; ref checkout happens after
	repo, err := git.PlainClone(tmpDir, false, cloneOpts)
	if err != nil {
		return nil, err
	}

	if err := checkoutRef(repo, ref); err != nil {
		return nil, err
	}

	abs := filepath.Join(tmpDir, filePath)
	// confirm file exists and is not a directory
	info, err := os.Stat(abs)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, fmt.Errorf("file not found in repo: %s", filePath)
		}
		return nil, err
	}
	if info.IsDir() {
		return nil, fmt.Errorf("path is a directory, expected file: %s", filePath)
	}
	return os.ReadFile(abs)
}

func checkoutRef(repo *git.Repository, ref Ref) error {
	if ref.Name == "" {
		return nil
	}
	wt, err := repo.Worktree()
	if err != nil {
		return err
	}

	switch ref.Type {
	case RefBranch:
		return wt.Checkout(&git.CheckoutOptions{Branch: plumbing.NewBranchReferenceName(ref.Name)})
	case RefTag:
		return wt.Checkout(&git.CheckoutOptions{Branch: plumbing.NewTagReferenceName(ref.Name)})
	case RefCommit:
		return wt.Checkout(&git.CheckoutOptions{Hash: plumbing.NewHash(ref.Name)})
	default:
		// Auto: try branch, then tag, then commit hash.
		if err := wt.Checkout(&git.CheckoutOptions{Branch: plumbing.NewBranchReferenceName(ref.Name)}); err == nil {
			return nil
		}
		if err := wt.Checkout(&git.CheckoutOptions{Branch: plumbing.NewTagReferenceName(ref.Name)}); err == nil {
			return nil
		}
		if err := wt.Checkout(&git.CheckoutOptions{Hash: plumbing.NewHash(ref.Name)}); err == nil {
			return nil
		}
		return fmt.Errorf("checkout ref '%s' failed", ref.Name)
	}
}

func isAuthError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, transport.ErrAuthenticationRequired) {
		return true
	}
	msg := err.Error()
	return strings.Contains(msg, "authentication required") || strings.Contains(msg, "Invalid username or token")
}

func (a Auth) credentials() *http.BasicAuth {
	if a.TokenEnv == "" {
		return nil
	}
	token := os.Getenv(a.TokenEnv)
	if token == "" {
		return nil
	}
	user := os.Getenv(a.UsernameEnv)
	if user == "" {
		user = "token"
	}
	return &http.BasicAuth{Username: user, Password: token}
}
