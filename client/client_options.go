package client

import (
	"fmt"
	"net/http"

	ht "github.com/ogen-go/ogen/http"
)

type clientWithApiKey struct {
	header string
}

func (c clientWithApiKey) Do(r *http.Request) (*http.Response, error) {
	r.Header.Set("Authorization", c.header)
	return http.DefaultClient.Do(r)
}

var _ ht.Client = &clientWithApiKey{}

// WithTonApiKey configures client to use tonApiKey for authorization.
// When working with tonapi.io, you should consider getting an API key at https://tonconsole.com/.
//
// Example:
//
// import (
//
//	tonapiClient "github.com/tonkeeper/opentonapi/client"
//
// )
//
//	func main() {
//	   cli, _ := tonapiClient.NewClient("https://tonapi.io", tonapiClient.WithTonApiKey(tonapiKey))
//	}
func WithTonApiKey(tonApiKey string) ClientOption {
	return WithClient(&clientWithApiKey{header: fmt.Sprintf("Bearer %s", tonApiKey)})
}
