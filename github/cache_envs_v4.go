package github

import (
    "context"
    "fmt"
    

    githubv4 "github.com/shurcooL/githubv4"
)

type EnvV4Data struct {
     Name                   string
    CanAdminsBypass        bool
    WaitTimer              int
    PreventSelfReview      bool
    Reviewers              []EnvReviewer
    DeploymentBranchPolicy *BranchPolicyV4
    ProtectionRules        []ProtectionRuleV4
}
type ProtectionRuleV4 struct {
    Type              string
    WaitTimer         int
    PreventSelfReview bool
    Reviewers         []EnvReviewer
}
type EnvReviewer struct {
    Type string
    ID   int64
}

type BranchPolicyV4 struct {
    ProtectedBranches    bool
    CustomBranchPolicies bool
}



// Load all environments for a repository
func (o *Owner) loadAllEnvironmentsV4(ctx context.Context, repoName string) error {
    o.envCacheOnce.Do(func() {
        if o.envCache == nil {
            o.envCache = make(map[string]map[string]*EnvV4Data)
        }
    })

    if _, ok := o.envCache[repoName]; ok {
        // already loaded
        return nil
    }

    o.envCache[repoName] = make(map[string]*EnvV4Data)

    var query struct {
        Repository struct {
            Environments struct {
                Nodes []struct {
                    Name            string
                    CanAdminsBypass bool
                    WaitTimer       githubv4.Int
                    PreventSelfReview bool
                    Reviewers []struct {
                        Type string
                        ID   githubv4.Int
                    }
                    DeploymentBranchPolicy struct {
                        ProtectedBranches    bool
                        CustomBranchPolicies bool
                    }
                }
                PageInfo struct {
                    HasNextPage githubv4.Boolean
                    EndCursor   githubv4.String
                }
            } `graphql:"environments(first: 100, after: $cursor)"`
        } `graphql:"repository(name: $name, owner: $owner)"`
    }

    variables := map[string]interface{}{
        "owner": githubv4.String(o.name),
        "name":  githubv4.String(repoName),
        "cursor": (*githubv4.String)(nil),
    }

    for {
        if err := o.v4client.Query(ctx, &query, variables); err != nil {
            return fmt.Errorf("failed to load environments for repo %s: %w", repoName, err)
        }

        for _, e := range query.Repository.Environments.Nodes {
            reviewers := make([]EnvReviewer, 0)
            for _, r := range e.Reviewers {
                reviewers = append(reviewers, EnvReviewer{
                    Type: r.Type,
                    ID:   int64(r.ID),
                })
            }

            o.envCache[repoName][e.Name] = &EnvV4Data{
                Name:              e.Name,
                CanAdminsBypass:   e.CanAdminsBypass,
                WaitTimer:         int(e.WaitTimer),
                PreventSelfReview: e.PreventSelfReview,
                Reviewers:         reviewers,
                DeploymentBranchPolicy: &BranchPolicyV4{
                    ProtectedBranches:    e.DeploymentBranchPolicy.ProtectedBranches,
                    CustomBranchPolicies: e.DeploymentBranchPolicy.CustomBranchPolicies,
                },
            }
        }

        if !bool(query.Repository.Environments.PageInfo.HasNextPage) {
            break
        }
        variables["cursor"] = githubv4.NewString(query.Repository.Environments.PageInfo.EndCursor)
    }

    return nil
}

// Get single environment from cache or fetch if missing
func (o *Owner) GetEnvironmentFromCache(ctx context.Context, repoName, envName string) (*EnvV4Data, error) {
    if o.envCache == nil || o.envCache[repoName] == nil {
        if err := o.loadAllEnvironmentsV4(ctx, repoName); err != nil {
            return nil, err
        }
    }

    if env, ok := o.envCache[repoName][envName]; ok {
        return env, nil
    }

    // Rare cache miss: fetch single environment (GraphQL query) here if needed
    // ... you can reuse similar GraphQL query for single env and add to cache ...

    return nil, fmt.Errorf("environment %s not found in repo %s", envName, repoName)
}