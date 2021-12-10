package external

import (
	"log"
	"strings"
)

type logger struct{}

func (l *logger) Write(p []byte) (n int, err error) {
	buf := string(p)
	for _, line := range strings.Split(buf, "\n") {
		if l := strings.Trim(line, " \t"); l != "" {
			log.Print(string(l))
		}
	}
	return len(p), nil
}
