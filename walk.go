package walk

import (
	"context"
	"errors"
	"fmt"
	"github.com/google/go-github/github"
	"golang.org/x/oauth2"
	_ "log"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const DEFAULT_BRANCH string = "main"

type GitHubWalker struct {
	owner      string
	repo       string
	branch     string
	concurrent bool
	client     *github.Client
	throttle   <-chan time.Time
}

func NewGitHubWalker(ctx context.Context, uri string) (*GitHubWalker, error) {

	u, err := url.Parse(uri)

	if err != nil {
		return nil, err
	}

	rate := time.Second / 10
	throttle := time.Tick(rate)

	gw := &GitHubWalker{
		throttle: throttle,
	}

	gw.owner = u.Host

	path := strings.TrimLeft(u.Path, "/")
	parts := strings.Split(path, "/")

	if len(parts) != 1 {
		return nil, errors.New("Invalid path")
	}

	gw.repo = parts[0]
	gw.branch = DEFAULT_BRANCH

	q := u.Query()

	token := q.Get("access_token")
	branch := q.Get("branch")
	concurrent := q.Get("concurrent")

	if token == "" {
		return nil, errors.New("Missing access token")
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

	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: token},
	)

	tc := oauth2.NewClient(ctx, ts)
	client := github.NewClient(tc)

	gw.client = client
	return gw, nil
}

func (gw *GitHubWalker) WalkURI(ctx context.Context, uri string) error {

	// log.Printf("Walk %s/%s/%s", gw.owner, gw.repo, uri)

	select {
	case <-ctx.Done():
		return nil
	default:
		// pass
	}

	file_contents, dir_contents, _, err := gw.client.Repositories.GetContents(ctx, gw.owner, gw.repo, uri, nil)

	if err != nil {
		return err
	}

	if file_contents != nil {
		return gw.walkFileContents(ctx, file_contents)
	}

	if dir_contents != nil {

		if gw.concurrent {
			return gw.walkDirectoryContentsConcurrently(ctx, dir_contents)
		} else {
			return gw.walkDirectoryContents(ctx, dir_contents)
		}
	}

	return nil
}

func (gw *GitHubWalker) walkDirectoryContents(ctx context.Context, contents []*github.RepositoryContent) error {

	for _, e := range contents {

		err := gw.WalkURI(ctx, *e.Path)

		if err != nil {
			return err
		}
	}

	return nil
}

func (gw *GitHubWalker) walkDirectoryContentsConcurrently(ctx context.Context, contents []*github.RepositoryContent) error {

	remaining := len(contents)

	done_ch := make(chan bool)
	err_ch := make(chan error)

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	for _, e := range contents {

		go func(e *github.RepositoryContent) {

			defer func() {
				done_ch <- true
			}()

			err := gw.WalkURI(ctx, *e.Path)

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

func (gw *GitHubWalker) walkFileContents(ctx context.Context, contents *github.RepositoryContent) error {

	path := *contents.Path

	fmt.Println(path)
	return nil

	/*
		name := *contents.Name

		switch filepath.Ext(name) {
		case ".geojson":
			// continue
		default:
			return nil
		}

		id, _, err := uri.ParseURI(path)

		if err != nil {
			return fmt.Errorf("Failed to parse %s, %w", err)
		}

		fmt.Println(id)
		return nil
	*/
}
