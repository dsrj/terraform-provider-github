package github

import (
	"context"
	"fmt"
	

	githubv4 "github.com/shurcooL/githubv4"
)

//
// RepoV4Data holds all repository fields we want to cache
//
type RepoV4Data struct {
	Name                     string
	Description              string
	Visibility               string
	IsArchived               bool
	IsPrivate                bool
	Topics                   []string
	DefaultBranch            string
	HomepageURL              string
	HasIssues                bool
	HasDiscussions           bool
	HasProjects              bool
	HasWiki                  bool
	IsTemplate               bool
	AllowAutoMerge           bool
	AllowMergeCommit         bool
	AllowRebaseMerge         bool
	AllowSquashMerge         bool
	AllowUpdateBranch        bool
	AllowForking             bool
	DeleteBranchOnMerge      bool
	WebCommitSignoffRequired bool
	MergeCommitMessage       string
	MergeCommitTitle         string
	SquashMergeCommitMessage string
	SquashMergeCommitTitle   string
	Fork                     bool
	ParentOwner              string
	ParentName               string
	TemplateOwner            string
	TemplateRepo             string
	HTMLURL                  string
	SSHURL                   string
	GitURL                   string
	SVNURL                    string
	PrimaryLanguage          string
	SecurityAnalysis         map[string]any
	VulnerabilityAlerts      bool
	HasPages                 bool
}


//
// loadAllReposV4
// ----------------
// Load all repos in organization with full data
// Uses sync.Once so it only runs once
//
func (o *Owner) loadAllReposV4(ctx context.Context) error {
	var loadErr error
	o.repoCacheOnce.Do(func() {
		o.repoCache = make(map[string]*RepoV4Data)

		var query struct {
			Organization struct {
				Repositories struct {
					Nodes []struct {
						Name                     string
						Description              string
						Visibility               githubv4.RepositoryVisibility
						IsArchived               bool
						IsPrivate                bool
						Topics                   []string
						DefaultBranchRef struct {
							Name string
						} `graphql:"defaultBranchRef"`
						HomepageURL              string `graphql:"homepageUrl"`
						HasIssues                bool   `graphql:"hasIssuesEnabled"`
						HasDiscussions           bool   `graphql:"hasDiscussionsEnabled"`
						HasProjects              bool   `graphql:"hasProjectsEnabled"`
						HasWiki                  bool   `graphql:"hasWikiEnabled"`
						IsTemplate               bool   `graphql:"isTemplate"`
						AllowAutoMerge           bool
						AllowMergeCommit         bool
						AllowRebaseMerge         bool
						AllowSquashMerge         bool
						AllowUpdateBranch        bool
						AllowForking             bool
						DeleteBranchOnMerge      bool
						WebCommitSignoffRequired bool
						MergeCommitMessage       string
						MergeCommitTitle         string
						SquashMergeCommitMessage string
						SquashMergeCommitTitle   string
						Fork                     bool
						Parent struct {
							Owner struct {
								Login string
							}
							Name string
						}
						TemplateRepository struct {
							Owner struct {
								Login string
							}
							Name string
						}
						URL            string `graphql:"url"`
						SSHURL         string `graphql:"sshUrl"`
						GitURL         string `graphql:"gitUrl"`
						SVNURL         string `graphql:"svnUrl"`
						PrimaryLanguage struct {
							Name string
						}
						SecurityAnalysis struct {
							AdvancedSecurityEnabled bool
							VulnerabilityAlerts     bool
						} `graphql:"securityAndAnalysis"`
						HasPages bool `graphql:"hasPages"`
					}
					PageInfo struct {
						HasNextPage githubv4.Boolean
						EndCursor   githubv4.String
					}
				} `graphql:"repositories(first: 100, after: $cursor)"`
			} `graphql:"organization(login: $login)"`
		}

		variables := map[string]interface{}{
			"login":  githubv4.String(o.name),
			"cursor": (*githubv4.String)(nil),
		}

		for {
			err := o.v4client.Query(ctx, &query, variables)
			if err != nil {
				loadErr = err
				return
			}

			for _, r := range query.Organization.Repositories.Nodes {
				o.repoCache[r.Name] = &RepoV4Data{
					Name:                     r.Name,
					Description:              r.Description,
					Visibility:               string(r.Visibility),
					IsArchived:               r.IsArchived,
					IsPrivate:                r.IsPrivate,
					Topics:                   r.Topics,
					DefaultBranch:            r.DefaultBranchRef.Name,
					HomepageURL:              r.HomepageURL,
					HasIssues:                r.HasIssues,
					HasDiscussions:           r.HasDiscussions,
					HasProjects:              r.HasProjects,
					HasWiki:                  r.HasWiki,
					IsTemplate:               r.IsTemplate,
					AllowAutoMerge:           r.AllowAutoMerge,
					AllowMergeCommit:         r.AllowMergeCommit,
					AllowRebaseMerge:         r.AllowRebaseMerge,
					AllowSquashMerge:         r.AllowSquashMerge,
					AllowUpdateBranch:        r.AllowUpdateBranch,
					AllowForking:             r.AllowForking,
					DeleteBranchOnMerge:      r.DeleteBranchOnMerge,
					WebCommitSignoffRequired: r.WebCommitSignoffRequired,
					MergeCommitMessage:       r.MergeCommitMessage,
					MergeCommitTitle:         r.MergeCommitTitle,
					SquashMergeCommitMessage: r.SquashMergeCommitMessage,
					SquashMergeCommitTitle:   r.SquashMergeCommitTitle,
					Fork:                     r.Fork,
					ParentOwner:              r.Parent.Owner.Login,
					ParentName:               r.Parent.Name,
					TemplateOwner:            r.TemplateRepository.Owner.Login,
					TemplateRepo:             r.TemplateRepository.Name,
					HTMLURL:                  r.URL,
					SSHURL:                   r.SSHURL,
					GitURL:                   r.GitURL,
					SVNURL:                    r.SVNURL,
					PrimaryLanguage:          r.PrimaryLanguage.Name,
					SecurityAnalysis: map[string]any{
						"advanced_security":    r.SecurityAnalysis.AdvancedSecurityEnabled,
						"vulnerability_alerts": r.SecurityAnalysis.VulnerabilityAlerts,
					},
					VulnerabilityAlerts: r.SecurityAnalysis.VulnerabilityAlerts,
					HasPages:            r.HasPages,
				}
			}

			if !bool(query.Organization.Repositories.PageInfo.HasNextPage) {
				break
			}
			variables["cursor"] = githubv4.NewString(query.Organization.Repositories.PageInfo.EndCursor)
		}
	})

	return loadErr
}

//
// GetRepoFromCache
// -----------------
// Return repo from cache, loading all repos first if needed
//
func (o *Owner) GetRepoFromCache(ctx context.Context, name string) (*RepoV4Data, error) {
	if o.repoCache == nil {
		if err := o.loadAllReposV4(ctx); err != nil {
			return nil, err
		}
	}

	repo, ok := o.repoCache[name]
	if ok {
		return repo, nil
	}

	// Rare cache miss — fetch single repo fully
	var query struct {
		Repository struct {
			Name                     string
			Description              string
			Visibility               githubv4.RepositoryVisibility
			IsArchived               bool
			IsPrivate                bool
			Topics                   []string
			DefaultBranchRef struct {
				Name string
			} `graphql:"defaultBranchRef"`
			HomepageURL              string `graphql:"homepageUrl"`
			HasIssues                bool   `graphql:"hasIssuesEnabled"`
			HasDiscussions           bool   `graphql:"hasDiscussionsEnabled"`
			HasProjects              bool   `graphql:"hasProjectsEnabled"`
			HasWiki                  bool   `graphql:"hasWikiEnabled"`
			IsTemplate               bool   `graphql:"isTemplate"`
			AllowAutoMerge           bool
			AllowMergeCommit         bool
			AllowRebaseMerge         bool
			AllowSquashMerge         bool
			AllowUpdateBranch        bool
			AllowForking             bool
			DeleteBranchOnMerge      bool
			WebCommitSignoffRequired bool
			MergeCommitMessage       string
			MergeCommitTitle         string
			SquashMergeCommitMessage string
			SquashMergeCommitTitle   string
			Fork                     bool
			Parent struct {
				Owner struct {
					Login string
				}
				Name string
			}
			TemplateRepository struct {
				Owner struct {
					Login string
				}
				Name string
			}
			URL            string `graphql:"url"`
			SSHURL         string `graphql:"sshUrl"`
			GitURL         string `graphql:"gitUrl"`
			SVNURL         string `graphql:"svnUrl"`
			PrimaryLanguage struct {
				Name string
			}
			SecurityAnalysis struct {
				AdvancedSecurityEnabled bool
				VulnerabilityAlerts     bool
			} `graphql:"securityAndAnalysis"`
			HasPages bool `graphql:"hasPages"`
		} `graphql:"repository(owner: $owner, name: $name)"`
	}

	variables := map[string]interface{}{
		"owner": githubv4.String(o.name),
		"name":  githubv4.String(name),
	}

	if err := o.v4client.Query(ctx, &query, variables); err != nil {
		return nil, fmt.Errorf("failed to fetch repository %s: %w", name, err)
	}

	// Add repo to cache
	repo = &RepoV4Data{
		Name:                     query.Repository.Name,
		Description:              query.Repository.Description,
		Visibility:               string(query.Repository.Visibility),
		IsArchived:               query.Repository.IsArchived,
		IsPrivate:                query.Repository.IsPrivate,
		Topics:                   query.Repository.Topics,
		DefaultBranch:            query.Repository.DefaultBranchRef.Name,
		HomepageURL:              query.Repository.HomepageURL,
		HasIssues:                query.Repository.HasIssues,
		HasDiscussions:           query.Repository.HasDiscussions,
		HasProjects:              query.Repository.HasProjects,
		HasWiki:                  query.Repository.HasWiki,
		IsTemplate:               query.Repository.IsTemplate,
		AllowAutoMerge:           query.Repository.AllowAutoMerge,
		AllowMergeCommit:         query.Repository.AllowMergeCommit,
		AllowRebaseMerge:         query.Repository.AllowRebaseMerge,
		AllowSquashMerge:         query.Repository.AllowSquashMerge,
		AllowUpdateBranch:        query.Repository.AllowUpdateBranch,
		AllowForking:             query.Repository.AllowForking,
		DeleteBranchOnMerge:      query.Repository.DeleteBranchOnMerge,
		WebCommitSignoffRequired: query.Repository.WebCommitSignoffRequired,
		MergeCommitMessage:       query.Repository.MergeCommitMessage,
		MergeCommitTitle:         query.Repository.MergeCommitTitle,
		SquashMergeCommitMessage: query.Repository.SquashMergeCommitMessage,
		SquashMergeCommitTitle:   query.Repository.SquashMergeCommitTitle,
		Fork:                     query.Repository.Fork,
		ParentOwner:              query.Repository.Parent.Owner.Login,
		ParentName:               query.Repository.Parent.Name,
		TemplateOwner:            query.Repository.TemplateRepository.Owner.Login,
		TemplateRepo:             query.Repository.TemplateRepository.Name,
		HTMLURL:                  query.Repository.URL,
		SSHURL:                   query.Repository.SSHURL,
		GitURL:                   query.Repository.GitURL,
		SVNURL:                    query.Repository.SVNURL,
		PrimaryLanguage:          query.Repository.PrimaryLanguage.Name,
		SecurityAnalysis: map[string]any{
			"advanced_security":    query.Repository.SecurityAnalysis.AdvancedSecurityEnabled,
			"vulnerability_alerts": query.Repository.SecurityAnalysis.VulnerabilityAlerts,
		},
		VulnerabilityAlerts: query.Repository.SecurityAnalysis.VulnerabilityAlerts,
		HasPages:            query.Repository.HasPages,
	}

	o.repoCache[name] = repo
	return repo, nil
}

