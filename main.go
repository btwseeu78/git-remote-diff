package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	yaml "gopkg.in/yaml.v3"

	"git-remote-diff/internal/gitutil"
	"git-remote-diff/internal/source"
	"git-remote-diff/internal/yamlutil"
)

func main() {
	var baseGitRepo, baseGitFile string
	var baseGitRef, baseBranch, baseTag, baseCommit string
	var updatedGitRepo, updatedGitFile string
	var updatedGitRef, updatedBranch, updatedTag, updatedCommit string
	var gitTokenEnv, gitUsernameEnv string

	rootCmd := &cobra.Command{
		Use:   "yamldiff [base.yaml] [updated.yaml]",
		Short: "Show only fields updated in the second YAML",
		Args: func(cmd *cobra.Command, args []string) error {
			baseGitSpec, err := buildGitSpec(baseGitRepo, baseGitFile, baseGitRef, baseBranch, baseTag, baseCommit)
			if err != nil {
				return err
			}
			updatedGitSpec, err := buildGitSpec(updatedGitRepo, updatedGitFile, updatedGitRef, updatedBranch, updatedTag, updatedCommit)
			if err != nil {
				return err
			}

			localNeeded := 0
			if baseGitSpec == nil {
				localNeeded++
			}
			if updatedGitSpec == nil {
				localNeeded++
			}
			if len(args) != localNeeded {
				return fmt.Errorf("expected %d local file(s) but got %d", localNeeded, len(args))
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			baseGitSpec, err := buildGitSpec(baseGitRepo, baseGitFile, baseGitRef, baseBranch, baseTag, baseCommit)
			if err != nil {
				return err
			}
			updatedGitSpec, err := buildGitSpec(updatedGitRepo, updatedGitFile, updatedGitRef, updatedBranch, updatedTag, updatedCommit)
			if err != nil {
				return err
			}

			argIdx := 0
			var baseSpec source.Spec
			if baseGitSpec != nil {
				baseSpec = source.Spec{Git: baseGitSpec}
			} else {
				baseSpec = source.Spec{LocalPath: args[argIdx]}
				argIdx++
			}

			var updatedSpec source.Spec
			if updatedGitSpec != nil {
				updatedSpec = source.Spec{Git: updatedGitSpec}
			} else {
				updatedSpec = source.Spec{LocalPath: args[argIdx]}
			}

			loader := source.Loader{GitAuth: gitutil.Auth{TokenEnv: gitTokenEnv, UsernameEnv: gitUsernameEnv}}

			baseNode, err := loader.Load(baseSpec)
			if err != nil {
				return fmt.Errorf("load base: %w", err)
			}
			updatedNode, err := loader.Load(updatedSpec)
			if err != nil {
				return fmt.Errorf("load updated: %w", err)
			}

			diff := yamlutil.DiffOnlyUpdated(baseNode, updatedNode)
			if diff == nil || yamlutil.IsEmptyMapOrSeq(diff) {
				fmt.Println("# no updates")
				return nil
			}

			enc := yaml.NewEncoder(os.Stdout)
			enc.SetIndent(2)
			if err := enc.Encode(diff); err != nil {
				return fmt.Errorf("encode diff: %w", err)
			}
			return nil
		},
	}

	// Flags
	rootCmd.Flags().StringVar(&baseGitRepo, "base-gitrepo", "", "Git repo URL for base (HTTPS or SSH)")
	rootCmd.Flags().StringVar(&baseGitFile, "base-gitfile", "", "Path to YAML file in repo for base")
	rootCmd.Flags().StringVar(&baseGitRef, "base-gitref", "", "Git ref (auto branch/tag/commit) for base")
	rootCmd.Flags().StringVar(&baseBranch, "base-branch", "", "Branch name for base git source")
	rootCmd.Flags().StringVar(&baseTag, "base-tag", "", "Tag name for base git source")
	rootCmd.Flags().StringVar(&baseCommit, "base-commit", "", "Commit hash for base git source")

	rootCmd.Flags().StringVar(&updatedGitRepo, "updated-gitrepo", "", "Git repo URL for updated (HTTPS or SSH)")
	rootCmd.Flags().StringVar(&updatedGitFile, "updated-gitfile", "", "Path to YAML file in repo for updated")
	rootCmd.Flags().StringVar(&updatedGitRef, "updated-gitref", "", "Git ref (auto branch/tag/commit) for updated")
	rootCmd.Flags().StringVar(&updatedBranch, "updated-branch", "", "Branch name for updated git source")
	rootCmd.Flags().StringVar(&updatedTag, "updated-tag", "", "Tag name for updated git source")
	rootCmd.Flags().StringVar(&updatedCommit, "updated-commit", "", "Commit hash for updated git source")

	gitTokenEnv = "GIT_AUTH_TOKEN"
	gitUsernameEnv = "GIT_USERNAME"
	rootCmd.Flags().StringVar(&gitTokenEnv, "git-token-env", gitTokenEnv, "Env var name containing HTTPS token (optional)")
	rootCmd.Flags().StringVar(&gitUsernameEnv, "git-username-env", gitUsernameEnv, "Env var name for HTTPS username (defaults to 'token')")

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func buildGitSpec(repo, file, autoRef, branch, tag, commit string) (*source.GitSpec, error) {
	if repo == "" && file == "" && autoRef == "" && branch == "" && tag == "" && commit == "" {
		return nil, nil
	}
	if repo == "" || file == "" {
		return nil, fmt.Errorf("git repo and gitfile must both be set for git-backed source")
	}

	refCount := countNonEmpty(branch, tag, commit, autoRef)
	if refCount > 1 {
		return nil, fmt.Errorf("specify only one of gitref/branch/tag/commit for %s", repo)
	}

	ref := gitutil.Ref{}
	switch {
	case branch != "":
		ref = gitutil.Ref{Name: branch, Type: gitutil.RefBranch}
	case tag != "":
		ref = gitutil.Ref{Name: tag, Type: gitutil.RefTag}
	case commit != "":
		ref = gitutil.Ref{Name: commit, Type: gitutil.RefCommit}
	case autoRef != "":
		ref = gitutil.Ref{Name: autoRef, Type: gitutil.RefAuto}
	}

	return &source.GitSpec{RepoURL: repo, Ref: ref, FilePath: file}, nil
}

func countNonEmpty(values ...string) int {
	c := 0
	for _, v := range values {
		if v != "" {
			c++
		}
	}
	return c
}
