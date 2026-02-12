package github

import (
	"context"
	"fmt"

	githubv4 "github.com/shurcooL/githubv4"
)

//
// RepoV4Data holds repository fields safe for GraphQL across GitHub + Enterprise
//
type RepoV4Data struct {
	Name            string
	Description     string
	Visibility      string
	IsArchived      bool
	IsPrivate       bool
	DefaultBranch   string
	HomepageURL     string
	HasIssues       bool
	HasProjects     bool
	HasWiki         bool
	IsTemplate      bool
	Fork            bool
	ParentOwner     string
	ParentName      string
	TemplateOwner   string
	TemplateRepo    string
	HTMLURL         string
	SSHURL          string
	GitURL          string
	SVNURL          string
	PrimaryLanguage string
}

//
// loadAllReposV4
// Loads all repositories in the organization once and caches them
//
func (o *Owner) loadAllReposV4(ctx context.Context) error {
	var loadErr error

	o.repoCacheOnce.Do(func() {
		o.repoCache = make(map[string]*RepoV4Data)

		var query struct {
			Organization struct {
				Repositories struct {
					Nodes []struct {
						Name        string
						Description string
						Visibility  githubv4.RepositoryVisibility
						IsArchived  bool
						IsPrivate   bool

						DefaultBranchRef struct {
							Name string
						} `graphql:"defaultBranchRef"`

						HomepageURL string `graphql:"homepageUrl"`

						HasIssues   bool `graphql:"hasIssuesEnabled"`
						HasProjects bool `graphql:"hasProjectsEnabled"`
						HasWiki     bool `graphql:"hasWikiEnabled"`

						IsTemplate bool
						Fork       bool

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

						URL    string `graphql:"url"`
						SSHURL string `graphql:"sshUrl"`
						GitURL string `graphql:"gitUrl"`
						SVNURL string `graphql:"svnUrl"`

						PrimaryLanguage struct {
							Name string
						}
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
					Name:            r.Name,
					Description:     r.Description,
					Visibility:      string(r.Visibility),
					IsArchived:      r.IsArchived,
					IsPrivate:       r.IsPrivate,
					DefaultBranch:   r.DefaultBranchRef.Name,
					HomepageURL:     r.HomepageURL,
					HasIssues:       r.HasIssues,
					HasProjects:     r.HasProjects,
					HasWiki:         r.HasWiki,
					IsTemplate:      r.IsTemplate,
					Fork:            r.Fork,
					ParentOwner:     r.Parent.Owner.Login,
					ParentName:      r.Parent.Name,
					TemplateOwner:   r.TemplateRepository.Owner.Login,
					TemplateRepo:    r.TemplateRepository.Name,
					HTMLURL:         r.URL,
					SSHURL:          r.SSHURL,
					GitURL:          r.GitURL,
					SVNURL:          r.SVNURL,
					PrimaryLanguage: r.PrimaryLanguage.Name,
				}
			}

			if !bool(query.Organization.Repositories.PageInfo.HasNextPage) {
				break
			}

			variables["cursor"] =
				githubv4.NewString(query.Organization.Repositories.PageInfo.EndCursor)
		}
	})

	return loadErr
}

//
// GetRepoFromCache
// Returns repository from cache or fetches single repo if missing
//
func (o *Owner) GetRepoFromCache(ctx context.Context, name string) (*RepoV4Data, error) {

	// Load all repos if cache not initialized
	if o.repoCache == nil {
		if err := o.loadAllReposV4(ctx); err != nil {
			return nil, err
		}
	}

	// Cache hit
	if repo, ok := o.repoCache[name]; ok {
		return repo, nil
	}

	// Cache miss → fetch single repo
	var query struct {
		Repository struct {
			Name        string
			Description string
			Visibility  githubv4.RepositoryVisibility
			IsArchived  bool
			IsPrivate   bool

			DefaultBranchRef struct {
				Name string
			} `graphql:"defaultBranchRef"`

			HomepageURL string `graphql:"homepageUrl"`

			HasIssues   bool `graphql:"hasIssuesEnabled"`
			HasProjects bool `graphql:"hasProjectsEnabled"`
			HasWiki     bool `graphql:"hasWikiEnabled"`

			IsTemplate bool
			Fork       bool

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

			URL    string `graphql:"url"`
			SSHURL string `graphql:"sshUrl"`
			GitURL string `graphql:"gitUrl"`
			SVNURL string `graphql:"svnUrl"`

			PrimaryLanguage struct {
				Name string
			}
		} `graphql:"repository(owner: $owner, name: $name)"`
	}

	variables := map[string]interface{}{
		"owner": githubv4.String(o.name),
		"name":  githubv4.String(name),
	}

	if err := o.v4client.Query(ctx, &query, variables); err != nil {
		return nil, fmt.Errorf("failed to fetch repository %s: %w", name, err)
	}

	repo := &RepoV4Data{
		Name:            query.Repository.Name,
		Description:     query.Repository.Description,
		Visibility:      string(query.Repository.Visibility),
		IsArchived:      query.Repository.IsArchived,
		IsPrivate:       query.Repository.IsPrivate,
		DefaultBranch:   query.Repository.DefaultBranchRef.Name,
		HomepageURL:     query.Repository.HomepageURL,
		HasIssues:       query.Repository.HasIssues,
		HasProjects:     query.Repository.HasProjects,
		HasWiki:         query.Repository.HasWiki,
		IsTemplate:      query.Repository.IsTemplate,
		Fork:            query.Repository.Fork,
		ParentOwner:     query.Repository.Parent.Owner.Login,
		ParentName:      query.Repository.Parent.Name,
		TemplateOwner:   query.Repository.TemplateRepository.Owner.Login,
		TemplateRepo:    query.Repository.TemplateRepository.Name,
		HTMLURL:         query.Repository.URL,
		SSHURL:          query.Repository.SSHURL,
		GitURL:          query.Repository.GitURL,
		SVNURL:          query.Repository.SVNURL,
		PrimaryLanguage: query.Repository.PrimaryLanguage.Name,
	}

	o.repoCache[name] = repo

	return repo, nil
}