package instrument

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// parseOSI turns a 21-character osi value (symbol padded to 6, yymmdd in the years 2000 to 2099, C or P, strike times
// 1000 in 8 digits) into the right, the strike as decimal text without trailing zeros and the expiry day as
// yyyy-mm-dd. A value of another length, a right other than C or P, or a date or strike that does not parse is an error.
func parseOSI(osi string) (right, strike, expiry string, err error) {
	if len(osi) != 21 {
		return "", "", "", fmt.Errorf("instrument: osi %q is not 21 characters", osi)
	}
	right = osi[12:13]
	if right != "C" && right != "P" {
		return "", "", "", fmt.Errorf("instrument: osi %q has right %q, not C or P", osi, right)
	}
	day, err := time.Parse("20060102", "20"+osi[6:12])
	if err != nil {
		return "", "", "", fmt.Errorf("instrument: osi %q has no valid date: %w", osi, err)
	}
	thousandths, err := strconv.ParseUint(osi[13:], 10, 64)
	if err != nil {
		return "", "", "", fmt.Errorf("instrument: osi %q has no valid strike: %w", osi, err)
	}
	strike = strconv.FormatUint(thousandths/1000, 10)
	if frac := strings.TrimRight(fmt.Sprintf("%03d", thousandths%1000), "0"); frac != "" {
		strike += "." + frac
	}
	return right, strike, day.Format(time.DateOnly), nil
}
