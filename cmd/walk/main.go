package main

// make cli && ./bin/walk -walker-uri 'walker://sfomuseum-data/sfomuseum-data-collection?access_token={TOKEN}' data

import (
	"context"
	"flag"
	"github.com/sfomuseum/go-github-walk"
	"log"
)

func main() {

	walker_uri := flag.String("walker-uri", "", "...")

	flag.Parse()
	uris := flag.Args()

	ctx := context.Background()

	w, err := walk.NewGitHubWalker(ctx, *walker_uri)

	if err != nil {
		log.Fatalf("Failed to create new walker, %v", err)
	}

	for _, uri := range uris {

		err := w.WalkURI(ctx, uri)

		if err != nil {
			log.Fatalf("Failed to walk URIs, %v", err)
		}
	}
}
