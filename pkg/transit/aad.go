package transit

import (
	"fmt"
	"strings"
)

func AAD(parts ...string) []byte {
	var builder strings.Builder
	for index, part := range parts {
		if index > 0 {
			builder.WriteByte('|')
		}
		fmt.Fprintf(&builder, "%d:%s", len(part), part)
	}
	return []byte(builder.String())
}
