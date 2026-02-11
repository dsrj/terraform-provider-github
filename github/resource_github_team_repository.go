package github

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strconv"

	"github.com/google/go-github/v82/github"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func resourceGithubTeamRepository() *schema.Resource {
	return &schema.Resource{
		Create: resourceGithubTeamRepositoryCreate,
		Read:   resourceGithubTeamRepositoryRead,
		Update: resourceGithubTeamRepositoryUpdate,
		Delete: resourceGithubTeamRepositoryDelete,
		Importer: &schema.ResourceImporter{
			State: func(d *schema.ResourceData, meta any) ([]*schema.ResourceData, error) {
				teamIdString, username, err := parseTwoPartID(d.Id(), "team_id", "username")
				if err != nil {
					return nil, err
				}

				teamId, err := getTeamID(teamIdString, meta)
				if err != nil {
					return nil, err
				}

				d.SetId(buildTwoPartID(strconv.FormatInt(teamId, 10), username))
				return []*schema.ResourceData{d}, nil
			},
		},

		Schema: map[string]*schema.Schema{
			"team_id": {
				Type:        schema.TypeString,
				Required:    true,
				ForceNew:    true,
				Description: "ID or slug of team",
			},
			"repository": {
				Type:        schema.TypeString,
				Required:    true,
				ForceNew:    true,
				Description: "The repository to add to the team.",
			},
			"permission": {
				Type:        schema.TypeString,
				Optional:    true,
				Default:     "pull",
				Description: "The permissions of team members regarding the repository. Must be one of 'pull', 'triage', 'push', 'maintain', 'admin' or the name of an existing custom repository role within the organisation.",
			},
			"etag": {
				Type:     schema.TypeString,
				Computed: true,
			},
		},
	}
}

func resourceGithubTeamRepositoryCreate(d *schema.ResourceData, meta any) error {
	err := checkOrganization(meta)
	if err != nil {
		return err
	}

	client := meta.(*Owner).v3client
	orgId := meta.(*Owner).id

	// The given team id could be an id or a slug
	givenTeamId := d.Get("team_id").(string)
	teamId, err := getTeamID(givenTeamId, meta)
	if err != nil {
		return err
	}

	orgName := meta.(*Owner).name
	repoName := d.Get("repository").(string)
	permission := d.Get("permission").(string)
	ctx := context.Background()

	_, err = client.Teams.AddTeamRepoByID(ctx,
		orgId,
		teamId,
		orgName,
		repoName,
		&github.TeamAddTeamRepoOptions{
			Permission: permission,
		},
	)
	if err != nil {
		return err
	}

	d.SetId(buildTwoPartID(strconv.FormatInt(teamId, 10), repoName))

	
	// ✅ Update Terraform state and v4 cache
	return resourceGithubTeamRepositoryRead(d, meta)
}

func resourceGithubTeamRepositoryRead(d *schema.ResourceData, meta any) error {
	o := meta.(*Owner)

	err := checkOrganization(o)
	if err != nil {
		return err
	}

	client := o.v3client
	orgId := o.id

	teamIdString, repoName, err := parseTwoPartID(d.Id(), "team_id", "repository")
	if err != nil {
		return err
	}

	teamId, err := getTeamID(teamIdString, meta)
	if err != nil {
		return err
	}

	orgName := o.name
	ctx := context.WithValue(context.Background(), ctxId, d.Id())
	if !d.IsNewResource() {
		ctx = context.WithValue(ctx, ctxEtag, d.Get("etag").(string))
	}

	// ---------- manual insert start ----------
	// Use repo cache for checking existence/archived status
	repo, err := o.GetRepoFromCache(ctx, repoName)
	if err != nil {
		log.Printf("[INFO] Removing team repository %s from state because repository %s does not exist", d.Id(), repoName)
		d.SetId("")
		return nil
	}
	if repo.IsArchived {
		log.Printf("[INFO] Removing team repository %s from state because repository %s is archived", d.Id(), repoName)
		d.SetId("")
		return nil
	}
	// ---------- manual insert end ----------

	// ---------- fetch team repo from v4 cache ----------
	var repoData *TeamRepoV4Data
	if o.teamRepoCache != nil && o.teamRepoCache[teamId] != nil {
		if r, ok := o.teamRepoCache[teamId][repoName]; ok {
			repoData = r
		}
	}

	// ---------- fallback to v3 API if not cached ----------
	if repoData == nil {
		// Check if team has access to the repo
		repoInfo, resp, err := client.Teams.IsTeamRepoByID(ctx, orgId, teamId, orgName, repoName)
		if err != nil {
			var ghErr *github.ErrorResponse
			if errors.As(err, &ghErr) {
				if ghErr.Response.StatusCode == http.StatusNotModified {
					return nil
				}
				if ghErr.Response.StatusCode == http.StatusNotFound {
					log.Printf("[INFO] Removing team repository association %s from state because it no longer exists in GitHub", d.Id())
					d.SetId("")
					return nil
				}
			}
			return err
		}

		repoData = &TeamRepoV4Data{
			Name:       repoName,
			Permission: getPermission(repoInfo.GetRoleName()),
		}

		// Initialize outer map if nil
		o.teamRepoCacheOnce.Do(func() {
			if o.teamRepoCache == nil {
				o.teamRepoCache = make(map[int64]map[string]*TeamRepoV4Data)
			}
		})

		// Initialize inner map if nil
		if o.teamRepoCache[teamId] == nil {
			o.teamRepoCache[teamId] = make(map[string]*TeamRepoV4Data)
		}

		// Add to cache
		o.teamRepoCache[teamId][repoName] = repoData

		// Save ETag
		if err := d.Set("etag", resp.Header.Get("ETag")); err != nil {
			return err
		}
	}

	// ---------- populate Terraform state ----------
	if d.Get("team_id") == "" {
		if err := d.Set("team_id", teamIdString); err != nil {
			return err
		}
	}
	if err := d.Set("repository", repoData.Name); err != nil {
		return err
	}
	if err := d.Set("permission", repoData.Permission); err != nil {
		return err
	}

	return nil
}
func resourceGithubTeamRepositoryUpdate(d *schema.ResourceData, meta any) error {
	err := checkOrganization(meta)
	if err != nil {
		return err
	}

	client := meta.(*Owner).v3client
	orgId := meta.(*Owner).id

	teamIdString, repoName, err := parseTwoPartID(d.Id(), "team_id", "repository")
	if err != nil {
		return err
	}
	teamId, err := strconv.ParseInt(teamIdString, 10, 64)
	if err != nil {
		return unconvertibleIdErr(teamIdString, err)
	}
	orgName := meta.(*Owner).name
	permission := d.Get("permission").(string)
	ctx := context.WithValue(context.Background(), ctxId, d.Id())

//---------- manual insert start ----------
// 1️⃣ Check if repository exists
repo, _, repoErr := client.Repositories.Get(ctx, orgName, repoName)
if repoErr != nil {
    var ghErr *github.ErrorResponse
    if errors.As(repoErr, &ghErr) && ghErr.Response != nil && ghErr.Response.StatusCode == http.StatusNotFound {
        log.Printf("[INFO] Removing team repository %s from state because repository %s does not exist", d.Id(), repoName)
        d.SetId("") // remove from state
        return nil
    }
    return repoErr
}

// 2️⃣ Check if repository is archived
if repo.GetArchived() {
    log.Printf("[INFO] Removing team repository %s from state because repository %s is archived", d.Id(), repoName)
    d.SetId("") // remove from state
    return nil
}
//---------- manual insert end ----------

	// the go-github library's AddTeamRepo method uses the add/update endpoint from GitHub API
	_, err = client.Teams.AddTeamRepoByID(ctx,
		orgId,
		teamId,
		orgName,
		repoName,
		&github.TeamAddTeamRepoOptions{
			Permission: permission,
		},
	)
	if err != nil {
		return err
	}
	d.SetId(buildTwoPartID(teamIdString, repoName))


	// ✅ Update Terraform state and v4 cache
	return resourceGithubTeamRepositoryRead(d, meta)
}

func resourceGithubTeamRepositoryDelete(d *schema.ResourceData, meta any) error {
	err := checkOrganization(meta)
	if err != nil {
		return err
	}

	client := meta.(*Owner).v3client
	orgId := meta.(*Owner).id


	
	teamIdString, repoName, err := parseTwoPartID(d.Id(), "team_id", "repository")
	if err != nil {
		return err
	}
	teamId, err := strconv.ParseInt(teamIdString, 10, 64)
	if err != nil {
		return unconvertibleIdErr(teamIdString, err)
	}
	orgName := meta.(*Owner).name
	ctx := context.WithValue(context.Background(), ctxId, d.Id())

	

	//---------- manual insert start ----------
	repo, _, repoErr := client.Repositories.Get(ctx, orgName, repoName)
	if repoErr != nil {
		if ghErr, ok := repoErr.(*github.ErrorResponse); ok && ghErr.Response.StatusCode == http.StatusNotFound {
			log.Printf("[INFO] Removing team repository %s from state because repository %s does not exist", d.Id(), repoName)
			d.SetId("")
			return nil
		}
		return repoErr
	}

	if repo.GetArchived() {
		log.Printf("[INFO] Removing team repository %s from state because repository %s is archived", d.Id(), repoName)
		d.SetId("")
		return nil
	}
	//---------- manual insert end ----------

	resp, err := client.Teams.RemoveTeamRepoByID(ctx, orgId, teamId, orgName, repoName) // actual delete fucntion call

	if resp.StatusCode == 404 {
		log.Printf("[DEBUG] Failed to find team %s to delete for repo: %s.", teamIdString, repoName)
		repo, _, err := client.Repositories.Get(ctx, orgName, repoName)
		if err != nil {
			return err
		}
		newRepoName := repo.GetName()
		if newRepoName != repoName {
			log.Printf("[INFO] Repo name has changed %s -> %s. "+
				"Try deleting team repository again.",
				repoName, newRepoName)
			_, err := client.Teams.RemoveTeamRepoByID(ctx, orgId, teamId, orgName, newRepoName)
			return handleArchivedRepoDelete(err, "team repository access", fmt.Sprintf("team %s", teamIdString), orgName, newRepoName)
		}
	}
// ✅ Remove from v4 cache
	if meta.(*Owner).teamRepoCache != nil && meta.(*Owner).teamRepoCache[teamId] != nil {
		delete(meta.(*Owner).teamRepoCache[teamId], repoName)
	}
	return handleArchivedRepoDelete(err, "team repository access", fmt.Sprintf("team %s", teamIdString), orgName, repoName)
}
