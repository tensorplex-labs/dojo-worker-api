package utils

import (
	"fmt"
	"strconv"

	"github.com/rs/zerolog/log"
)

func IpDecimalToDotted(decimalIP interface{}) string {
	var ipInt int64
	switch v := decimalIP.(type) {
	case int64:
		ipInt = v
	case string:
		var err error
		ipInt, err = strconv.ParseInt(v, 10, 64)
		if err != nil {
			log.Error().Err(err).Msg("Error converting string to int64")
			return ""
		}
	default:
		fmt.Println("Unsupported type provided")
		return ""
	}
	b0 := ipInt & 0xff
	b1 := (ipInt >> 8) & 0xff
	b2 := (ipInt >> 16) & 0xff
	b3 := (ipInt >> 24) & 0xff

	return fmt.Sprintf("%d.%d.%d.%d", b3, b2, b1, b0)
}
