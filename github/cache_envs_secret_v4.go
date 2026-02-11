package github

import (
	"context"
	"fmt"

	githubv4 "github.com/shurcooL/githubv4"
)

// EnvSecretV4Data represents a secret in a repository environment
type EnvSecretV4Data struct {
	Name          string
	CreatedAt     string
	UpdatedAt     string
	Visibility    string   // "private", "selected", "organization"
	SelectedTeams []string
	SelectedRepos []string
}

// Load all secrets for an environment in a repository (v4 GraphQL)
func (o *Owner) loadAllEnvSecretsV4(ctx context.Context, repoName, envName string) error {
	o.envSecretCacheOnce.Do(func() {
		if o.envSecretCache == nil {
			o.envSecretCache = make(map[string]map[string]map[string]*EnvSecretV4Data)
			// repo -> env -> secretName -> secretData
		}
	})

	if _, ok := o.envSecretCache[repoName]; !ok {
		o.envSecretCache[repoName] = make(map[string]map[string]*EnvSecretV4Data)
	}
	if _, ok := o.envSecretCache[repoName][envName]; ok {
		// Already loaded
		return nil
	}

	o.envSecretCache[repoName][envName] = make(map[string]*EnvSecretV4Data)

	var query struct {
		Repository struct {
			Environment struct {
				Secrets struct {
					Nodes []struct {
						Name        string
						CreatedAt   githubv4.DateTime
						UpdatedAt   githubv4.DateTime
						Visibility  string
						SelectedRepositories []struct {
							Name string
						}
						SelectedTeams []struct {
							Name string
						}
					}
					PageInfo struct {
						HasNextPage githubv4.Boolean
						EndCursor   githubv4.String
					}
				} `graphql:"secrets(first: 100, after: $cursor)"`
			} `graphql:"environment(name: $envName)"`
		} `graphql:"repository(name: $repoName, owner: $owner)"`
	}

	variables := map[string]interface{}{
		"owner":    githubv4.String(o.name),
		"repoName": githubv4.String(repoName),
		"envName":  githubv4.String(envName),
		"cursor":   (*githubv4.String)(nil),
	}

	for {
		if err := o.v4client.Query(ctx, &query, variables); err != nil {
			return fmt.Errorf("failed to load environment secrets for repo %s/%s: %w", repoName, envName, err)
		}

		for _, s := range query.Repository.Environment.Secrets.Nodes {
			selectedRepos := make([]string, 0, len(s.SelectedRepositories))
			for _, r := range s.SelectedRepositories {
				selectedRepos = append(selectedRepos, r.Name)
			}
			selectedTeams := make([]string, 0, len(s.SelectedTeams))
			for _, t := range s.SelectedTeams {
				selectedTeams = append(selectedTeams, t.Name)
			}

			o.envSecretCache[repoName][envName][s.Name] = &EnvSecretV4Data{
				Name:          s.Name,
				CreatedAt:     s.CreatedAt.String(),
				UpdatedAt:     s.UpdatedAt.String(),
				Visibility:    s.Visibility,
				SelectedRepos: selectedRepos,
				SelectedTeams: selectedTeams,
			}
		}

		if !bool(query.Repository.Environment.Secrets.PageInfo.HasNextPage) {
			break
		}
		variables["cursor"] = githubv4.NewString(query.Repository.Environment.Secrets.PageInfo.EndCursor)
	}

	return nil
}

// Get single secret from cache (v4 only)
func (o *Owner) GetEnvSecretFromCache(ctx context.Context, repoName, envName, secretName string) (*EnvSecretV4Data, error) {
	if o.envSecretCache == nil || o.envSecretCache[repoName] == nil || o.envSecretCache[repoName][envName] == nil {
		if err := o.loadAllEnvSecretsV4(ctx, repoName, envName); err != nil {
			return nil, err
		}
	}

	if s, ok := o.envSecretCache[repoName][envName][secretName]; ok {
		return s, nil
	}

	return nil, fmt.Errorf("environment secret %s not found in repo %s/%s", secretName, repoName, envName)
}