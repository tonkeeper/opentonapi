package api

import (
	"fmt"
	"runtime/debug"
	"strconv"
	"strings"

	"github.com/tonkeeper/tongo/boc"
)

// deserializeBoc tries to deserialize boc string in base64 or hex format.
func deserializeBoc(bocStr string) ([]*boc.Cell, error) {
	cells, err := boc.DeserializeBocBase64(bocStr)
	if err != nil {
		return boc.DeserializeBocHex(bocStr)
	}
	return cells, nil
}

func deserializeSingleBoc(bocStr string) (*boc.Cell, error) {
	cells, err := deserializeBoc(bocStr)
	if err != nil {
		return nil, err
	}
	if len(cells) != 1 {
		return nil, fmt.Errorf("invalid boc roots number %v", len(cells))
	}
	return cells[0], nil
}

func GetPackageVersionInt(packagePath string) (int, error) {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return 0, fmt.Errorf("error getting build info")
	}

	for _, dep := range info.Deps {
		if strings.Contains(dep.Path, packagePath) {
			version := strings.TrimPrefix(dep.Version, "v")

			parts := strings.Split(version, ".")

			result := 0
			for _, part := range parts {
				num, err := strconv.Atoi(part)
				if err != nil {
					return 0, fmt.Errorf("error parsing version number: %v", err)
				}
				result = result*100 + num
			}

			return result, nil
		}
	}

	return 0, fmt.Errorf("package %s not found", packagePath)
}
