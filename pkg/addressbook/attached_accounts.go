package addressbook

import (
	"fmt"
	"strings"

	"github.com/tonkeeper/opentonapi/pkg/core"
	imgGenerator "github.com/tonkeeper/opentonapi/pkg/image"
	"github.com/tonkeeper/opentonapi/pkg/references"
	rules "github.com/tonkeeper/scam_backoffice_rules"
	"github.com/tonkeeper/tongo/ton"
)

// AttachedAccountType represents the type of the attached account
const (
	KnownAccountWeight = 5000
	BoostForFullMatch  = 100
	BoostForVerified   = 50
)

// AttachedAccount represents domains, nft collections for quick search by name are presented
type AttachedAccount struct {
	Name       string              `json:"name"`
	Preview    string              `json:"preview"`
	Wallet     ton.AccountID       `json:"wallet"`
	Slug       string              `json:"-"`
	Symbol     string              `json:"-"`
	Type       AttachedAccountType `json:"-"`
	Weight     int64               `json:"-"`
	Popular    int64               `json:"-"`
	Trust      core.TrustType      `json:"-"`
	Normalized string              `json:"-"`
}

// ConvertAttachedAccount converts a known account to an attached account
func ConvertAttachedAccount(name, slug, image string, account ton.AccountID, weight int, trust core.TrustType, accountType AttachedAccountType) (AttachedAccount, error) {
	var convertedName string
	// Handle different account types and assign appropriate values
	switch accountType {
	case TonDomainAccountType, TgDomainAccountType:
		weight = 1000
		convertedName = name + " · account"
		// Generate image URL for "t.me" subdomains
		if strings.HasSuffix(name, "t.me") && strings.Count(name, ".") == 2 {
			// Generate specific preview from Telegram if domain matches
			username := strings.TrimSuffix(name, ".t.me")
			image = "https://t.me/i/userpic/320/" + username + ".jpg"
		} else {
			image = references.PlugAutoCompleteDomain
		}
	case JettonSymbolAccountType, JettonNameAccountType:
		convertedName = name + " · jetton"
		if image == "" {
			image = references.PlugAutoCompleteJetton
		}
	case NftCollectionAccountType:
		convertedName = name + " · collection"
		if image == "" {
			image = references.PlugAutoCompleteCollection
		}
	case ManualAccountType:
		convertedName = name + " · account"
		if image == "" {
			image = references.PlugAutoCompleteAccount
		}
	default:
		return AttachedAccount{}, fmt.Errorf("unknown account type: %v", accountType)
	}
	if len(image) > 0 { // Generate a preview image
		image = imgGenerator.DefaultGenerator.GenerateImageUrl(image, 200, 200)
	}
	return AttachedAccount{
		Name:       convertedName,
		Slug:       slug,
		Preview:    image,
		Wallet:     account,
		Type:       accountType,
		Weight:     int64(weight),
		Popular:    int64(weight),
		Trust:      trust,
		Normalized: rules.NormalizeString(slug),
	}, nil
}

// GenerateSlugVariants generates name variants by rotating the words
func GenerateSlugVariants(name string) []string {
	words := strings.Fields(name) // Split the name into words
	var variants []string
	// Generate up to 3 variants by rotating the words
	for i := 0; i < len(words) && i < 3; i++ {
		variant := append(words[i:], words[:i]...) // Rotate the words
		variants = append(variants, strings.Join(variant, " "))
	}
	return variants
}

// FindIndexes finds the start and end indexes of the prefix in the sorted list
func FindIndexes(sortedList []AttachedAccount, prefix string) (int, int) {
	low, high := 0, len(sortedList)-1
	startIdx := -1
	// Find starting index for the prefix
	for low <= high {
		med := (low + high) / 2
		if strings.HasPrefix(sortedList[med].Normalized, prefix) {
			startIdx = med
			high = med - 1
		} else if sortedList[med].Normalized < prefix {
			low = med + 1
		} else {
			high = med - 1
		}
	}
	if startIdx == -1 { // No prefix match
		return -1, -1
	}
	low, high = startIdx, len(sortedList)-1
	endIdx := -1
	// Find ending index for the prefix
	for low <= high {
		med := (low + high) / 2
		if strings.HasPrefix(sortedList[med].Normalized, prefix) {
			endIdx = med
			low = med + 1
		} else {
			high = med - 1
		}
	}

	return startIdx, endIdx
}
