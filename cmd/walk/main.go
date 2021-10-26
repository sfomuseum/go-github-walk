package main

// make cli && ./bin/walk -walker-uri 'walker://sfomuseum-data/sfomuseum-data-collection?access_token={TOKEN}' data

import (
	"context"
	"flag"
	"fmt"
	"github.com/google/go-github/github"
	"github.com/sfomuseum/go-github-walk"
	"log"
)

func main() {

	walker_uri := flag.String("walker-uri", "", "...")

	flag.Parse()
	uris := flag.Args()

	ctx := context.Background()

	cb := func(ctx context.Context, contents *github.RepositoryContent) error {
		fmt.Println(*contents.Path)
		return nil
	}

	w, err := walk.NewGitHubWalker(ctx, *walker_uri)

	if err != nil {
		log.Fatalf("Failed to create new walker, %v", err)
	}

	for _, uri := range uris {

		err := w.WalkURI(ctx, uri, cb)

		if err != nil {
			log.Fatalf("Failed to walk URIs, %v", err)
		}
	}
}
