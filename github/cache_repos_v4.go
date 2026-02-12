package github

import (
	"context"
	"fmt"

	githubv4 "github.com/shurcooL/githubv4"
)

//
// RepoV4Data holds only essential repository fields
//
type RepoV4Data struct {
	Name        string
	Description string
	Visibility  string
	IsArchived  bool
	IsPrivate   bool
	TemplateRepo string
}

//
// loadAllReposV4
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
						TemplateRepository struct {
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
					Name:         r.Name,
					Description:  r.Description,
					Visibility:   string(r.Visibility),
					IsArchived:   r.IsArchived,
					IsPrivate:    r.IsPrivate,
					TemplateRepo: r.TemplateRepository.Name,
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
//
func (o *Owner) GetRepoFromCache(ctx context.Context, name string) (*RepoV4Data, error) {

	if o.repoCache == nil {
		if err := o.loadAllReposV4(ctx); err != nil {
			return nil, err
		}
	}

	if repo, ok := o.repoCache[name]; ok {
		return repo, nil
	}

	var query struct {
		Repository struct {
			Name        string
			Description string
			Visibility  githubv4.RepositoryVisibility
			IsArchived  bool
			IsPrivate   bool
			TemplateRepository struct {
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
		Name:         query.Repository.Name,
		Description:  query.Repository.Description,
		Visibility:   string(query.Repository.Visibility),
		IsArchived:   query.Repository.IsArchived,
		IsPrivate:    query.Repository.IsPrivate,
		TemplateRepo: query.Repository.TemplateRepository.Name,
	}

	o.repoCache[name] = repo

	return repo, nil
}