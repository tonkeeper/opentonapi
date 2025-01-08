package addressbook

import (
	"fmt"
	"strings"

	imgGenerator "github.com/tonkeeper/opentonapi/pkg/image"
	"github.com/tonkeeper/opentonapi/pkg/references"
	rules "github.com/tonkeeper/scam_backoffice_rules"
	"github.com/tonkeeper/tongo/ton"
)

// AttachedAccountType represents the type of the attached account
const (
	KnownAccountWeight   = 1000
	BoostForFullMatch    = 100
	BoostForOriginalName = 50
	BoostForVerified     = 5
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
	Verified   bool                `json:"-"`
	Normalized string              `json:"-"`
}

// ConvertAttachedAccount converts a known account to an attached account
func ConvertAttachedAccount(slug, image string, account ton.AccountID, weight int, verified bool, accountType AttachedAccountType) (AttachedAccount, error) {
	var name string
	// Handle different account types and assign appropriate values
	switch accountType {
	case TonDomainAccountType, TgDomainAccountType:
		weight = 1000
		name = fmt.Sprintf("%v · account", slug)
		// Generate image URL for "t.me" subdomains
		if strings.HasSuffix(slug, "t.me") && strings.Count(slug, ".") == 2 {
			image = fmt.Sprintf("https://t.me/i/userpic/320/%v.jpg", strings.TrimSuffix(slug, ".t.me"))
		} else {
			image = references.PlugAutoCompleteDomain
		}
	case JettonSymbolAccountType, JettonNameAccountType:
		name = fmt.Sprintf("%v · jetton", slug)
		if image == "" {
			image = references.PlugAutoCompleteJetton
		}
	case NftCollectionAccountType:
		name = fmt.Sprintf("%v · collection", slug)
		if image == "" {
			image = references.PlugAutoCompleteCollection
		}
	case ManualAccountType:
		name = fmt.Sprintf("%v · account", slug)
		if image == "" {
			image = references.PlugAutoCompleteAccount
		}
	default:
		return AttachedAccount{}, fmt.Errorf("unknown account type")
	}
	if len(image) > 0 { // Generate a preview image
		image = imgGenerator.DefaultGenerator.GenerateImageUrl(image, 200, 200)
	}
	return AttachedAccount{
		Name:       name,
		Slug:       slug,
		Preview:    image,
		Wallet:     account,
		Type:       accountType,
		Weight:     int64(weight),
		Popular:    int64(weight),
		Verified:   verified,
		Normalized: rules.NormalizeJettonSymbol(slug),
	}, nil
}

// GenerateNameVariants generates name variants by rotating the words
func GenerateNameVariants(name string) []string {
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
