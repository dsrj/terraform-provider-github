package github

import (
	"context"
	"fmt"

	githubv4 "github.com/shurcooL/githubv4"
)

// TeamRepoV4Data represents a repository that a team has access to
type TeamRepoV4Data struct {
	Name       string
	Permission string
}

// Load all repositories a team has access to using GraphQL
func (o *Owner) loadAllTeamReposV4(ctx context.Context, teamID int64) error {
	o.teamRepoCacheOnce.Do(func() {
		if o.teamRepoCache == nil {
			o.teamRepoCache = make(map[int64]map[string]*TeamRepoV4Data)
		}
	})

	if _, ok := o.teamRepoCache[teamID]; ok {
		// already loaded
		return nil
	}

	o.teamRepoCache[teamID] = make(map[string]*TeamRepoV4Data)

	var query struct {
		Organization struct {
			Team struct {
				Repositories struct {
					Nodes []struct {
						Name       string
						Permission string
					}
					PageInfo struct {
						HasNextPage githubv4.Boolean
						EndCursor   githubv4.String
					}
				} `graphql:"repositories(first: 100, after: $cursor)"`
			} `graphql:"team(id: $teamId)"`
		} `graphql:"organization(login: $owner)"`
	}

	variables := map[string]interface{}{
		"owner":  githubv4.String(o.name),
		"teamId": githubv4.ID(fmt.Sprintf("%d", teamID)),
		"cursor": (*githubv4.String)(nil),
	}

	for {
		if err := o.v4client.Query(ctx, &query, variables); err != nil {
			return fmt.Errorf("failed to load repositories for team %d: %w", teamID, err)
		}

		for _, r := range query.Organization.Team.Repositories.Nodes {
			o.teamRepoCache[teamID][r.Name] = &TeamRepoV4Data{
				Name:       r.Name,
				Permission: r.Permission,
			}
		}

		if !bool(query.Organization.Team.Repositories.PageInfo.HasNextPage) {
			break
		}
		variables["cursor"] = githubv4.NewString(query.Organization.Team.Repositories.PageInfo.EndCursor)
	}

	return nil
}

// Get a single team repository from cache or fetch if missing
func (o *Owner) GetTeamRepoFromCache(ctx context.Context, teamID int64, repoName string) (*TeamRepoV4Data, error) {
	if o.teamRepoCache == nil || o.teamRepoCache[teamID] == nil {
		if err := o.loadAllTeamReposV4(ctx, teamID); err != nil {
			return nil, err
		}
	}

	if repo, ok := o.teamRepoCache[teamID][repoName]; ok {
		return repo, nil
	}

	// Rare cache miss: optionally fetch single repository via GraphQL
	// (You can implement a single repo query here if needed)
	return nil, fmt.Errorf("team %d does not have access to repo %s", teamID, repoName)
}

// Remove a team-repo from cache (after deletion)
func (o *Owner) RemoveTeamRepoFromCache(teamID int64, repoName string) {
	if o.teamRepoCache != nil && o.teamRepoCache[teamID] != nil {
		delete(o.teamRepoCache[teamID], repoName)
	}
}