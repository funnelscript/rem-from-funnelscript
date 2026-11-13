package fetch

import "context"

type Response struct {
	URL    string
	Status int
	Body   []byte
}

type Getter interface {
	Get(ctx context.Context, rawURL string) (Response, error)
}
