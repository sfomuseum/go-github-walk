package walk

import (
	"context"
	"fmt"
	"github.com/google/go-github/github"
	"golang.org/x/oauth2"
	_ "log"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// DEFAULT_BRANCH is the assumed default branch for any given GitHub repository.
const DEFAULT_BRANCH string = "main"

// WalkCallbackFunc defines a custom callback function to be invoked for every file in a Github repository.
type WalkCallbackFunc func(context.Context, *github.RepositoryContent) error

// GitHubWalker is a struct that wraps operations for walking all the files in a GitHub repository.
type GitHubWalker struct {
	// A valid GitHub user or organization name.
	owner string
	// A valid GitHub repository name.
	repo string
	// A valid GitHub repository branch.
	branch string
	// A boolean flag indicating whether directory contents should be processed concurrently.
	concurrent bool
	// A  *github.Client instance
	client *github.Client
	// A time.Tick instance liming the maximum number of API requests to 5/sec.
	api_throttle <-chan time.Time
	// A boolean flag indicating whether on an GitHub API rate limit error the GitHubWalker should pause until the specified reset time.
	wait_on_reset bool
}

// NewGitHubWalker will create a new `GitHubWalker` instance from details defined in uri. uri takes the form of:
//
//	walk://sfomuseum-data/sfomuseum-data-collection?access_token={ACCESS_TOKEN}&concurrent=1
//
// Where it's component part are:
//
// scheme: `walk`, but this can be anything.
// host: A valid GitHub user or organization name.
// path: A valid GitHub repository name.
//
// And it's allowable query parameters are:
//
// access_token: A valid GitHub API access token (required).
// branch: A valid GitHub repository branch to walk.
// concurrent: A boolean flag indicating whether directory contents should be processed concurrently.
// wait-on-reset: A boolean flag indicating whether on an GitHub API rate limit error the GitHubWalker should pause until the specified reset time.
func NewGitHubWalker(ctx context.Context, uri string) (*GitHubWalker, error) {

	u, err := url.Parse(uri)

	if err != nil {
		return nil, fmt.Errorf("Failed to parse URI, %w", err)
	}

	rate := time.Second / 5
	api_throttle := time.Tick(rate)

	gw := &GitHubWalker{
		api_throttle: api_throttle,
	}

	gw.owner = u.Host

	path := strings.TrimLeft(u.Path, "/")
	parts := strings.Split(path, "/")

	if len(parts) != 1 {
		return nil, fmt.Errorf("Invalid path")
	}

	gw.repo = parts[0]
	gw.branch = DEFAULT_BRANCH

	q := u.Query()

	token := q.Get("access_token")
	branch := q.Get("branch")
	concurrent := q.Get("concurrent")
	wait := q.Get("wait-on-reset")

	if token == "" {
		return nil, fmt.Errorf("Missing access token")
	}

	if branch != "" {
		gw.branch = branch
	}

	if concurrent != "" {

		c, err := strconv.ParseBool(concurrent)

		if err != nil {
			return nil, err
		}

		gw.concurrent = c

	}

	if wait != "" {

		w, err := strconv.ParseBool(wait)

		if err != nil {
			return nil, fmt.Errorf("Failed to parse wait-on-reset parameter, %w", err)
		}

		gw.wait_on_reset = w
	}

	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: token},
	)

	tc := oauth2.NewClient(ctx, ts)
	client := github.NewClient(tc)

	gw.client = client
	return gw, nil
}

// WalkURI will retrieve uri and if it is a file pass it to cb for final processing. If the contents of uri is
// a directory then each of its children will be processed by calling gw.WalkURI.
func (gw *GitHubWalker) WalkURI(ctx context.Context, uri string, cb WalkCallbackFunc) error {

	// fmt.Printf("walk %s\n", uri)

	// log.Printf("Walk %s/%s/%s", gw.owner, gw.repo, uri)

	<-gw.api_throttle

	select {
	case <-ctx.Done():
		return nil
	default:
		// pass
	}

	// https://pkg.go.dev/github.com/google/go-github/v39/github#hdr-Rate_Limiting
	// https://docs.github.com/en/rest/overview/resources-in-the-rest-api#rate-limiting

	// https://pkg.go.dev/github.com/google/go-github/v39/github#RepositoriesService.GetContents
	// https://docs.github.com/en/rest/reference/repos#get-repository-content
	// https://pkg.go.dev/github.com/google/go-github/v39/github#RepositoryContentGetOptions

	file_contents, dir_contents, rsp, err := gw.client.Repositories.GetContents(ctx, gw.owner, gw.repo, uri, nil)

	/*
		rate_limit := rsp.Rate.Limit
		rate_remaining := rsp.Rate.Remaining
		fmt.Printf("Fetch %s (%d/%d)\n", uri, rate_remaining, rate_limit)
	*/

	if err != nil {

		_, is_rate_limit_err := err.(*github.RateLimitError)

		if is_rate_limit_err && gw.wait_on_reset {

			reset := rsp.Rate.Reset.Unix()
			now := time.Now().Unix()

			sleep := time.Duration(reset - now)

			<-time.After(sleep * time.Second)
			return gw.WalkURI(ctx, uri, cb)
		}

		return fmt.Errorf("Failed to get contents for %s, %w", uri, err)
	}

	if file_contents != nil {

		err := cb(ctx, file_contents)

		if err != nil {
			return fmt.Errorf("Walk callback func failed, %w", err)
		}

		return nil
	}

	if dir_contents != nil {

		if gw.concurrent {
			return gw.walkDirectoryContentsConcurrently(ctx, dir_contents, cb)
		} else {
			return gw.walkDirectoryContents(ctx, dir_contents, cb)
		}
	}

	return nil
}

// walkDirectoryContents will process contents invoking wg.WalkURI for each item.
func (gw *GitHubWalker) walkDirectoryContents(ctx context.Context, contents []*github.RepositoryContent, cb WalkCallbackFunc) error {

	for _, e := range contents {

		select {
		case <-ctx.Done():
			return nil
		default:
			// pass
		}

		err := gw.WalkURI(ctx, *e.Path, cb)

		if err != nil {
			return err
		}
	}

	return nil
}

// walkDirectoryContentsConcurrently will process contents concurrently invoking wg.WalkURI for each item. Processes are
// throttled according to the maximum number of read workers defined in the gw constructor.
func (gw *GitHubWalker) walkDirectoryContentsConcurrently(ctx context.Context, contents []*github.RepositoryContent, cb WalkCallbackFunc) error {

	remaining := len(contents)

	done_ch := make(chan bool)
	err_ch := make(chan error)

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	for _, e := range contents {

		select {
		case <-ctx.Done():
			return nil
		default:
			// pass
		}

		go func(e *github.RepositoryContent) {

			defer func() {
				done_ch <- true
			}()

			err := gw.WalkURI(ctx, *e.Path, cb)

			if err != nil {
				err_ch <- err
			}

		}(e)
	}

	for remaining > 0 {
		select {
		case <-done_ch:
			remaining -= 1
		case err := <-err_ch:
			return err
		default:
			// pass
		}
	}

	return nil
}
