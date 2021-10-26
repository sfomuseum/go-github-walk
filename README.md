# go-github-walk

 Go package for walking all of the files in a GitHub repository.
 
## Documentation

[![Go Reference](https://pkg.go.dev/badge/github.com/sfomuseum/go-github-walk.svg)](https://pkg.go.dev/github.com/sfomuseum/go-github-walk)

## Example

```
package main

import (
	"context"
	"flag"
	"fmt"
	"github.com/google/go-github/github"
	"github.com/sfomuseum/go-github-walk"
)

func main() {

	walker_uri := flag.String("walker-uri", "", "A valid go-github-walk.GitHubWalker URI string.")
	flag.Parse()

	ctx := context.Background()

	cb := func(ctx context.Context, contents *github.RepositoryContent) error {
		fmt.Println(*contents.Path)
		return nil
	}

	w, _ := walk.NewGitHubWalker(ctx, *walker_uri)
	w.WalkURI(ctx, "", cb)
}
```

For example:

```
$> ./bin/walk -walker-uri 'walk://sfomuseum-data/sfomuseum-data-maps?access_token={ACCESS_TOKEN}&concurrent=1' data
data/147/788/175/3/1477881753.geojson
data/147/788/175/7/1477881757.geojson
data/171/295/239/3/1712952393.geojson
data/136/039/135/1/1360391351.geojson
... and so on
```

## See also

* https://github.com/google/go-github