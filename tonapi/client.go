package tonapi

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

// WithToken configures client to use tonApiKey for authorization.
// When working with tonapi.io, you should consider getting an API key at https://tonconsole.com/.
//
// Example:
//
// import (
//
//	tonapi "github.com/tonkeeper/opentonapi/tonapi"
//
// )
//
//	func main() {
//	   cli, _ := tonapi.New(tonapi.WithToken(tonapiKey))
//	}
func WithToken(tonApiKey string) ClientOption {
	return WithClient(&clientWithApiKey{header: fmt.Sprintf("Bearer %s", tonApiKey)})
}

const TonApiURL = "https://tonapi.io"

// TestnetTonApiURL is an endpoint to work with testnet.
//
// Example:
// client, err := NewClient(tonapi.TestnetTonApiURL)
const TestnetTonApiURL = "https://testnet.tonapi.io"

// New returns a new Client.
func New(opts ...ClientOption) (*Client, error) {
	return NewClient(TonApiURL, opts...)
}
