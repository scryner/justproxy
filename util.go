package justproxy

import (
	"fmt"
	"net/http"
	"regexp"
	"strings"
)

var (
	hostParser *regexp.Regexp
)

func init() {
	hostParser = regexp.MustCompile(`.*:([0-9]+)`)
}

func GetAddr(host string) string {
	ss := hostParser.FindStringSubmatch(host)
	if len(ss) < 2 {
		return fmt.Sprintf("%s:80", host)
	}

	return host
}

func IsRequestStatic(r *http.Request) bool {
	ss := strings.Split(r.RequestURI, ".")
	if len(ss) < 2 {
		return false
	}

	ext := strings.ToUpper(ss[len(ss)-1])

	switch ext {
	case "PNG", "JPG", "JPEG", "GIF", "CSS", "JS":
		return true

	default:
		return false
	}

	panic(`never reached`)
}

func safelyDo(fun func()) (err error) {
	defer func() {
		if e := recover(); e != nil {
			err = fmt.Errorf("%v", e)
		}
	}()

	fun()
	return
}
